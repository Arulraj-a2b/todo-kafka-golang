package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
	"todo-kafka/kafka/pending"
	"todo-kafka/kafka/producer"
	"todo-kafka/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

func isValidStatus(s models.Status) bool {
	return s == models.StatusPending || s == models.StatusInProgress ||
		s == models.StatusCompleted || s == models.StatusDeleted
}

func isValidPriority(p models.Priority) bool {
	return p == models.PriorityLow || p == models.PriorityMedium || p == models.PriorityHigh
}

// GetTodos godoc
// @Summary      List todos for the authenticated user
// @Tags         todos
// @Produce      json
// @Success      200  {object}  TodosListResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos [get]
func GetTodos(c *gin.Context, db *sql.DB) {
	userID := c.GetString("user_id")
	rows, err := db.Query(`
		SELECT id, user_id, title, status, priority, due_date, tags, created_at, updated_at
		FROM todos
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Failed to query todos: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to query todos"})
		return
	}
	defer rows.Close()

	todos := []models.Todo{}
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Title, &t.Status, &t.Priority,
			&t.DueDate, pq.Array(&t.Tags), &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			log.Printf("Failed to scan todo: %v", err)
			continue
		}
		if t.Tags == nil {
			t.Tags = []string{}
		}
		todos = append(todos, t)
	}

	c.IndentedJSON(http.StatusOK, gin.H{
		"todos": todos,
		"total": len(todos),
	})
}

// CreateTodo godoc
// @Summary      Create a todo
// @Description  Publishes a create event to Kafka and waits for the consumer to confirm.
// @Tags         todos
// @Accept       json
// @Produce      json
// @Param        body  body      TodoRequest  true  "Todo to create"
// @Success      201   {object}  models.Todo
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Failure      504   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos [post]
func CreateTodo(c *gin.Context) {
	userID := c.GetString("user_id")

	var payload TodoRequest

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body - title and status are required"})
		return
	}

	if payload.Title == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if !isValidStatus(payload.Status) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
		return
	}
	if payload.Priority == "" {
		payload.Priority = models.PriorityMedium
	}
	if !isValidPriority(payload.Priority) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid priority - must be low, medium, or high"})
		return
	}
	if payload.Tags == nil {
		payload.Tags = []string{}
	}

	newTodo := models.Todo{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     payload.Title,
		Status:    payload.Status,
		Priority:  payload.Priority,
		DueDate:   payload.DueDate,
		Tags:      payload.Tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	completed := pending.Register(newTodo.ID)

	eventJSON, err := json.Marshal(models.TodoEvent{Action: models.ActionCreate, Todo: newTodo})
	if err != nil {
		log.Printf("Failed to marshal todo event: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal todo"})
		return
	}

	err = producer.KafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(newTodo.ID),
		Value: eventJSON,
	})
	if err != nil {
		log.Printf("Failed to write to Kafka: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to send to Kafka"})
		return
	}

	select {
	case r := <-completed:
		switch r.Status {
		case pending.StatusOK:
			c.IndentedJSON(http.StatusCreated, newTodo)
		default:
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": r.Err})
		}
	case <-time.After(50 * time.Second):
		c.IndentedJSON(http.StatusGatewayTimeout, gin.H{"error": "consumer took too long"})
	}
}

// DeleteTodo godoc
// @Summary      Delete a todo
// @Description  Publishes a delete event to Kafka and waits for confirmation.
// @Tags         todos
// @Produce      json
// @Param        id   path      string  true  "Todo ID (UUID)"
// @Success      200  {object}  MessageResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Failure      504  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos/{id} [delete]
func DeleteTodo(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	completed := pending.Register(id)

	eventJSON, err := json.Marshal(models.TodoEvent{
		Action: models.ActionDelete,
		Todo:   models.Todo{ID: id, UserID: userID},
	})
	if err != nil {
		log.Printf("Failed to marshal todo event: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal todo"})
		return
	}

	err = producer.KafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(id),
		Value: eventJSON,
	})
	if err != nil {
		log.Printf("Failed to write to Kafka: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to send to Kafka"})
		return
	}

	select {
	case r := <-completed:
		switch r.Status {
		case pending.StatusOK:
			c.IndentedJSON(http.StatusOK, gin.H{"message": "todo deleted"})
		case pending.StatusNotFound:
			c.IndentedJSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		default:
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": r.Err})
		}
	case <-time.After(50 * time.Second):
		c.IndentedJSON(http.StatusGatewayTimeout, gin.H{"error": "consumer took too long"})
	}
}

// UpdateTodo godoc
// @Summary      Update a todo
// @Description  Publishes an update event to Kafka and waits for confirmation.
// @Tags         todos
// @Accept       json
// @Produce      json
// @Param        id    path      string       true  "Todo ID (UUID)"
// @Param        body  body      TodoRequest  true  "Updated todo"
// @Success      200   {object}  models.Todo
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Failure      504   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos/{id} [put]
func UpdateTodo(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var payload TodoRequest

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body - title and status are required"})
		return
	}

	if payload.Title == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if !isValidStatus(payload.Status) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
		return
	}
	if payload.Priority == "" {
		payload.Priority = models.PriorityMedium
	}
	if !isValidPriority(payload.Priority) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid priority - must be low, medium, or high"})
		return
	}
	if payload.Tags == nil {
		payload.Tags = []string{}
	}

	updatedTodo := models.Todo{
		ID:        id,
		UserID:    userID,
		Title:     payload.Title,
		Status:    payload.Status,
		Priority:  payload.Priority,
		DueDate:   payload.DueDate,
		Tags:      payload.Tags,
		UpdatedAt: time.Now(),
	}

	completed := pending.Register(id)

	eventJSON, err := json.Marshal(models.TodoEvent{Action: models.ActionUpdate, Todo: updatedTodo})
	if err != nil {
		log.Printf("Failed to marshal todo event: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal todo"})
		return
	}

	err = producer.KafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(id),
		Value: eventJSON,
	})
	if err != nil {
		log.Printf("Failed to write to Kafka: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to send to Kafka"})
		return
	}

	select {
	case r := <-completed:
		switch r.Status {
		case pending.StatusOK:
			c.IndentedJSON(http.StatusOK, updatedTodo)
		case pending.StatusNotFound:
			c.IndentedJSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		default:
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": r.Err})
		}
	case <-time.After(50 * time.Second):
		c.IndentedJSON(http.StatusGatewayTimeout, gin.H{"error": "consumer took too long"})
	}
}
