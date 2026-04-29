package repository

import (
	"database/sql"
	"errors"
	"todo-service/models"

	"github.com/lib/pq"
)

var ErrNotFound = errors.New("todo not found")

func InsertTodo(db *sql.DB, t models.Todo) error {
	_, err := db.Exec(`
		INSERT INTO todos (id, user_id, title, status, priority, due_date, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		t.ID, t.UserID, t.Title, t.Status, t.Priority, t.DueDate,
		pq.Array(t.Tags), t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func UpdateTodo(db *sql.DB, t models.Todo) error {
	res, err := db.Exec(`
		UPDATE todos
		SET title = $1, status = $2, priority = $3, due_date = $4, tags = $5, updated_at = $6
		WHERE id = $7 AND user_id = $8
	`,
		t.Title, t.Status, t.Priority, t.DueDate,
		pq.Array(t.Tags), t.UpdatedAt, t.ID, t.UserID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func DeleteTodo(db *sql.DB, id, userID string) error {
	res, err := db.Exec(`DELETE FROM todos WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
