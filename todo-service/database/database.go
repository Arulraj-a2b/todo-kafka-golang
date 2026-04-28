package database

import (
	"database/sql"
	"log"
	"os"
	"todo-service/kafka/consumer"
)

func InitDB() *sql.DB {
	connStr := os.Getenv("TODO_DATABASE_URL")
	if connStr == "" {
		log.Fatal("TODO_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Todo DB connected")

	consumer.CreateTable(db)

	return db
}
