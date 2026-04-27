package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"todo-kafka/kafka/pending"
	"todo-kafka/kafka/producer"
	"todo-kafka/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

// ExportTodos godoc
// @Summary      Export todos as CSV
// @Description  Returns the authenticated user's todos as a CSV file (columns: title,status,priority,due_date,tags). Tags are pipe-separated.
// @Tags         todos
// @Produce      text/csv
// @Success      200  {file}    file  "CSV download (todos.csv)"
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos/export [get]
func ExportTodos(c *gin.Context, db *sql.DB) {
	userID := c.GetString("user_id")
	rows, err := db.Query(`
		SELECT title, status, priority, due_date, tags
		FROM todos
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Failed to query todos for export: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", `attachment; filename="todos.csv"`)

	w := csv.NewWriter(c.Writer)
	if err := w.Write([]string{"title", "status", "priority", "due_date", "tags"}); err != nil {
		log.Printf("Failed to write CSV header: %v", err)
		return
	}

	for rows.Next() {
		var title, status, priority string
		var due *time.Time
		var tags []string
		if err := rows.Scan(&title, &status, &priority, &due, pq.Array(&tags)); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		dueStr := ""
		if due != nil {
			dueStr = due.Format(time.RFC3339)
		}
		_ = w.Write([]string{title, status, priority, dueStr, strings.Join(tags, "|")})
	}
	w.Flush()
}

// ImportTodos godoc
// @Summary      Import todos from CSV
// @Description  Multipart upload. CSV columns: title,status,priority,due_date,tags (tags pipe-separated). Each row is published to Kafka and confirmed by the consumer.
// @Tags         todos
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "CSV file"
// @Success      200   {object}  ImportResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos/import [post]
func ImportTodos(c *gin.Context) {
	userID := c.GetString("user_id")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required (form field 'file')"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot open file"})
		return
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty or invalid CSV"})
		return
	}

	imported, failed := 0, 0
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) < 5 {
			failed++
			continue
		}

		var due *time.Time
		if strings.TrimSpace(row[3]) != "" {
			if t, err := time.Parse(time.RFC3339, row[3]); err == nil {
				due = &t
			}
		}

		tags := []string{}
		if strings.TrimSpace(row[4]) != "" {
			tags = strings.Split(row[4], "|")
		}

		status := models.Status(row[1])
		if !isValidStatus(status) {
			status = models.StatusPending
		}
		priority := models.Priority(row[2])
		if !isValidPriority(priority) {
			priority = models.PriorityMedium
		}

		todo := models.Todo{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     row[0],
			Status:    status,
			Priority:  priority,
			DueDate:   due,
			Tags:      tags,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		completed := pending.Register(todo.ID)

		eventJSON, err := json.Marshal(models.TodoEvent{
			Action:           models.ActionCreate,
			Todo:             todo,
			SkipNotification: true,
		})
		if err != nil {
			failed++
			continue
		}

		if err := producer.KafkaWriter.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(todo.ID),
			Value: eventJSON,
		}); err != nil {
			log.Printf("Import Kafka write failed: %v", err)
			failed++
			continue
		}

		select {
		case res := <-completed:
			if res.Status == pending.StatusOK {
				imported++
			} else {
				failed++
			}
		case <-time.After(10 * time.Second):
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{"imported": imported, "failed": failed})
}
