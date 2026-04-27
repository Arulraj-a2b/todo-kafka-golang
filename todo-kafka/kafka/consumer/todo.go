package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"todo-kafka/kafka/pending"
	"todo-kafka/models"

	"github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

func CreateTable(db *sql.DB) {
	usersSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	);`
	if _, err := db.Exec(usersSQL); err != nil {
		log.Printf("Failed to create users table: %v", err)
	}

	todosSQL := `
	CREATE TABLE IF NOT EXISTS todos (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id),
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		priority TEXT NOT NULL DEFAULT 'medium',
		due_date TIMESTAMPTZ,
		tags TEXT[] NOT NULL DEFAULT '{}',
		created_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	);`
	if _, err := db.Exec(todosSQL); err != nil {
		log.Printf("Failed to create todos table: %v", err)
	}
}

func InsertTodoToDB(todo models.Todo, db *sql.DB) pending.Result {
	_, err := db.Exec(`
		INSERT INTO todos (id, user_id, title, status, priority, due_date, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		todo.ID,
		todo.UserID,
		todo.Title,
		todo.Status,
		todo.Priority,
		todo.DueDate,
		pq.Array(todo.Tags),
		todo.CreatedAt,
		todo.UpdatedAt,
	)
	if err != nil {
		log.Printf("Failed to insert todo into database: %v", err)
		return pending.Result{Status: pending.StatusError, Err: err.Error()}
	}
	return pending.Result{Status: pending.StatusOK}
}

func DeleteTodoFromDB(todo models.Todo, db *sql.DB) pending.Result {
	res, err := db.Exec("DELETE FROM todos WHERE id = $1 AND user_id = $2", todo.ID, todo.UserID)
	if err != nil {
		log.Printf("Failed to delete todo from database: %v", err)
		return pending.Result{Status: pending.StatusError, Err: err.Error()}
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("Failed to read RowsAffected: %v", err)
		return pending.Result{Status: pending.StatusError, Err: err.Error()}
	}
	if rowsAffected == 0 {
		return pending.Result{Status: pending.StatusNotFound}
	}
	return pending.Result{Status: pending.StatusOK}
}

func UpdateTodoInDB(todo models.Todo, db *sql.DB) pending.Result {
	res, err := db.Exec(`
		UPDATE todos
		SET title = $1, status = $2, priority = $3, due_date = $4, tags = $5, updated_at = $6
		WHERE id = $7 AND user_id = $8
	`,
		todo.Title,
		todo.Status,
		todo.Priority,
		todo.DueDate,
		pq.Array(todo.Tags),
		todo.UpdatedAt,
		todo.ID,
		todo.UserID,
	)
	if err != nil {
		log.Printf("Failed to update todo in database: %v", err)
		return pending.Result{Status: pending.StatusError, Err: err.Error()}
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return pending.Result{Status: pending.StatusNotFound}
	}
	return pending.Result{Status: pending.StatusOK}
}

func InitConsumer(db *sql.DB) {

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{os.Getenv("KAFKA_BROKER")},
		Topic:       os.Getenv("KAFKA_TOPIC"),
		GroupID:     os.Getenv("KAFKA_GROUP_ID"),
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()

	log.Println("Kafka consumer started...")

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Failed to read message from Kafka: %v", err)
			continue
		}
		var event models.TodoEvent
		err = json.Unmarshal(msg.Value, &event)
		if err != nil {
			log.Printf("Failed to unmarshal Kafka message: %v", err)
			continue
		}

		var result pending.Result
		switch event.Action {
		case models.ActionCreate:
			result = InsertTodoToDB(event.Todo, db)
		case models.ActionDelete:
			result = DeleteTodoFromDB(event.Todo, db)
		case models.ActionUpdate:
			result = UpdateTodoInDB(event.Todo, db)
		default:
			log.Printf("Unknown event action %q", event.Action)
			result = pending.Result{Status: pending.StatusError, Err: "unknown action"}
		}

		pending.Completed(event.Todo.ID, result)

		log.Printf("Received message: %s", string(msg.Value))
	}

}
