package handlers

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"todo-service/cache"
	"todo-service/kafka/producer"
	"todo-service/models"
	"todo-service/pagination"
	"todo-service/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	pageOneCacheTTL = 60 * time.Second
)

func isValidStatus(s models.Status) bool {
	return s == models.StatusPending || s == models.StatusInProgress ||
		s == models.StatusCompleted || s == models.StatusDeleted
}

func isValidPriority(p models.Priority) bool {
	return p == models.PriorityLow || p == models.PriorityMedium || p == models.PriorityHigh
}

// publishEvent publishes a fire-and-forget Kafka event for downstream consumers
// (notification-worker for email, cache-invalidator). Errors are logged, not
// returned — the DB write has already committed.
func publishEvent(action string, todo models.Todo, userEmail string) {
	event := models.TodoEvent{Action: action, Todo: todo, UserEmail: userEmail}
	b, err := json.Marshal(event)
	if err != nil {
		log.Printf("event marshal failed: %v", err)
		return
	}
	if producer.KafkaWriter == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := producer.KafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(todo.ID),
		Value: b,
	}); err != nil {
		log.Printf("kafka publish failed for %s: %v", todo.ID, err)
	}
}

// GetTodos godoc
// @Summary      List todos for the authenticated user (cursor-paginated)
// @Description  Returns a page of todos ordered by created_at DESC, id DESC. Use the response's next_cursor to fetch the next page. Optional status filter.
// @Tags         todos
// @Produce      json
// @Param        limit   query     int     false  "Page size (default 50, max 100)"
// @Param        cursor  query     string  false  "Opaque cursor from a previous response"
// @Param        status  query     string  false  "Filter by status"  Enums(pending, in_progress, completed, deleted)
// @Success      200     {object}  TodosListResponse
// @Failure      400     {object}  ErrorResponse
// @Failure      401     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos [get]
func GetTodos(c *gin.Context, db *sql.DB, cc *cache.Client) {
	userID := c.GetString("user_id")

	limit := defaultPageSize
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > maxPageSize {
				n = maxPageSize
			}
			limit = n
		}
	}

	statusFilter := c.Query("status")
	cursorStr := c.Query("cursor")

	var cur pagination.Cursor
	hasCursor := false
	if cursorStr != "" {
		decoded, err := pagination.Decode(cursorStr)
		if err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		cur = decoded
		hasCursor = true
	}

	// Cache-aside on page 1 only. Cache key embeds the filter so different
	// status filters get their own entry. Deeper pages skip cache because
	// invalidating per-page is messy and the index makes them fast anyway.
	cacheKey := ""
	if !hasCursor && cc.Enabled() {
		cacheKey = pageOneCacheKey(userID, limit, statusFilter)
		if cached, err := cc.Get(c.Request.Context(), cacheKey); err == nil {
			c.Data(http.StatusOK, "application/json", cached)
			return
		} else if err != nil && !cache.IsMiss(err) {
			log.Printf("cache get %s: %v", cacheKey, err)
		}
	}

	args := []any{userID}
	q := `
		SELECT id, user_id, title, status, priority, due_date, tags, created_at, updated_at
		FROM todos
		WHERE user_id = $1`
	if statusFilter != "" {
		q += ` AND status = $` + strconv.Itoa(len(args)+1)
		args = append(args, statusFilter)
	}
	if hasCursor {
		// (created_at, id) < ($n, $n+1) — keyset pagination using the composite index.
		q += ` AND (created_at, id) < ($` + strconv.Itoa(len(args)+1) + `, $` + strconv.Itoa(len(args)+2) + `)`
		args = append(args, cur.CreatedAt, cur.ID)
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1) // +1 to detect has_more

	rows, err := db.Query(q, args...)
	if err != nil {
		log.Printf("query todos: %v", err)
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
			log.Printf("scan todo: %v", err)
			continue
		}
		if t.Tags == nil {
			t.Tags = []string{}
		}
		todos = append(todos, t)
	}

	hasMore := false
	if len(todos) > limit {
		hasMore = true
		todos = todos[:limit]
	}

	resp := TodosListResponse{Todos: todos, HasMore: hasMore}
	if hasMore {
		last := todos[len(todos)-1]
		resp.NextCursor = pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	}

	body, err := json.Marshal(resp)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to encode response"})
		return
	}

	if cacheKey != "" {
		cc.Set(c.Request.Context(), cacheKey, body, pageOneCacheTTL)
	}

	c.Data(http.StatusOK, "application/json", body)
}

// pageOneCacheKey shape: todos:list:{userID}:p1:{filterHash}
// Hash isolates "no filter" from "status=pending" etc. without leaking
// status names into Redis keys.
func pageOneCacheKey(userID string, limit int, status string) string {
	h := sha1.Sum([]byte(strconv.Itoa(limit) + "|" + status))
	return "todos:list:" + userID + ":p1:" + hex.EncodeToString(h[:8])
}

// CreateTodo godoc
// @Summary      Create a todo
// @Description  Writes to Postgres synchronously and publishes a fire-and-forget event for emails.
// @Tags         todos
// @Accept       json
// @Produce      json
// @Param        body  body      TodoRequest  true  "Todo to create"
// @Success      201   {object}  models.Todo
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos [post]
func CreateTodo(c *gin.Context, db *sql.DB) {
	userID := c.GetString("user_id")
	userEmail := c.GetString("email")

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

	now := time.Now()
	todo := models.Todo{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     payload.Title,
		Status:    payload.Status,
		Priority:  payload.Priority,
		DueDate:   payload.DueDate,
		Tags:      payload.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repository.InsertTodo(db, todo); err != nil {
		log.Printf("insert todo: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to insert todo"})
		return
	}

	go publishEvent(models.ActionCreate, todo, userEmail)

	c.IndentedJSON(http.StatusCreated, todo)
}

// UpdateTodo godoc
// @Summary      Update a todo
// @Description  Writes to Postgres synchronously and publishes a fire-and-forget event for emails.
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
// @Security     BearerAuth
// @Router       /todos/{id} [put]
func UpdateTodo(c *gin.Context, db *sql.DB) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	userEmail := c.GetString("email")

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

	updated := models.Todo{
		ID:        id,
		UserID:    userID,
		Title:     payload.Title,
		Status:    payload.Status,
		Priority:  payload.Priority,
		DueDate:   payload.DueDate,
		Tags:      payload.Tags,
		UpdatedAt: time.Now(),
	}

	if err := repository.UpdateTodo(db, updated); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.IndentedJSON(http.StatusNotFound, gin.H{"error": "todo not found"})
			return
		}
		log.Printf("update todo: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update todo"})
		return
	}

	go publishEvent(models.ActionUpdate, updated, userEmail)

	c.IndentedJSON(http.StatusOK, updated)
}

// DeleteTodo godoc
// @Summary      Delete a todo
// @Description  Removes the todo from Postgres synchronously and publishes a fire-and-forget event for cache invalidation. No email is sent on delete.
// @Tags         todos
// @Produce      json
// @Param        id   path      string  true  "Todo ID (UUID)"
// @Success      200  {object}  MessageResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos/{id} [delete]
func DeleteTodo(c *gin.Context, db *sql.DB) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	if err := repository.DeleteTodo(db, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.IndentedJSON(http.StatusNotFound, gin.H{"error": "todo not found"})
			return
		}
		log.Printf("delete todo: %v", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to delete todo"})
		return
	}

	// Publish so cache-invalidator drops the user's list cache.
	// UserEmail empty → notification-worker won't send email for delete.
	go publishEvent(models.ActionDelete, models.Todo{ID: id, UserID: userID}, "")

	c.IndentedJSON(http.StatusOK, gin.H{"message": "todo deleted"})
}
