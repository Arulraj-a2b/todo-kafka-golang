package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"todo-service/models"
	"todo-service/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// maxImportRows caps a single CSV upload. Beyond this we'd want async / 202.
const maxImportRows = 10000

// ExportTodos godoc
// @Summary      Export todos as CSV
// @Description  Streams the authenticated user's todos as a CSV file. Tags are pipe-separated.
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
		slog.ErrorContext(c.Request.Context(), "export query failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", `attachment; filename="todos.csv"`)

	w := csv.NewWriter(c.Writer)
	if err := w.Write([]string{"title", "status", "priority", "due_date", "tags"}); err != nil {
		slog.ErrorContext(c.Request.Context(), "csv header write failed", "err", err)
		return
	}

	for rows.Next() {
		var title, status, priority string
		var due *time.Time
		var tags []string
		if err := rows.Scan(&title, &status, &priority, &due, pq.Array(&tags)); err != nil {
			slog.ErrorContext(c.Request.Context(), "csv export row scan failed", "err", err)
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
// @Description  Multipart upload. CSV columns: title,status,priority,due_date,tags (pipe-separated). Streamed row-by-row; max 10000 rows per upload. Each row is inserted directly into Postgres (no Kafka, no email notification).
// @Tags         todos
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "CSV file"
// @Success      200   {object}  ImportResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /todos/import [post]
func ImportTodos(c *gin.Context, db *sql.DB) {
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
	r.FieldsPerRecord = -1 // tolerate ragged rows; we validate width manually

	imported, failed := 0, 0
	importErrors := []string{}

	rowNum := 0
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowNum++
		if err != nil {
			failed++
			importErrors = append(importErrors, fmt.Sprintf("row %d: malformed CSV: %v", rowNum, err))
			continue
		}
		if rowNum == 1 {
			continue // header
		}
		if (rowNum - 1) > maxImportRows {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("import capped at %d rows; split your file", maxImportRows),
			})
			return
		}

		if len(row) < 5 {
			failed++
			importErrors = append(importErrors, fmt.Sprintf("row %d: expected 5 columns, got %d", rowNum, len(row)))
			continue
		}

		title := strings.TrimSpace(row[0])
		if title == "" {
			failed++
			importErrors = append(importErrors, fmt.Sprintf("row %d: title is required", rowNum))
			continue
		}

		status := models.Status(strings.TrimSpace(row[1]))
		if !isValidStatus(status) {
			failed++
			importErrors = append(importErrors, fmt.Sprintf("row %d: invalid status %q", rowNum, row[1]))
			continue
		}

		priority := models.Priority(strings.TrimSpace(row[2]))
		if !isValidPriority(priority) {
			failed++
			importErrors = append(importErrors, fmt.Sprintf("row %d: invalid priority %q", rowNum, row[2]))
			continue
		}

		var due *time.Time
		if strings.TrimSpace(row[3]) != "" {
			t, perr := time.Parse(time.RFC3339, strings.TrimSpace(row[3]))
			if perr != nil {
				failed++
				importErrors = append(importErrors, fmt.Sprintf("row %d: invalid due_date %q (RFC3339)", rowNum, row[3]))
				continue
			}
			due = &t
		}

		tags := []string{}
		if strings.TrimSpace(row[4]) != "" {
			tags = strings.Split(row[4], "|")
		}

		now := time.Now()
		todo := models.Todo{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     title,
			Status:    status,
			Priority:  priority,
			DueDate:   due,
			Tags:      tags,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := repository.InsertTodo(db, todo); err != nil {
			failed++
			importErrors = append(importErrors, fmt.Sprintf("row %d: insert failed", rowNum))
			continue
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{"imported": imported, "failed": failed, "errors": importErrors})
}
