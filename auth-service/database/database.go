package database

import (
	"database/sql"
	"log"
	"os"
)

func InitDB() *sql.DB {
	connStr := os.Getenv("AUTH_DATABASE_URL")
	if connStr == "" {
		log.Fatal("AUTH_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Auth DB connected")

	createTable(db)

	return db
}

func createTable(db *sql.DB) {
	sql := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	);`
	if _, err := db.Exec(sql); err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}
}
