package database

import (
	"database/sql"
	"log"
	"os"
	"todo-kafka/kafka/consumer"
)

func InitDB() *sql.DB {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Database connected")

	consumer.CreateTable(db)

	return db
}
