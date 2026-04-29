package database

import (
	"database/sql"
	"log"
	"os"
	"time"
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

	// Pool sizing matches todo-service (see comment there). Auth traffic is
	// even more bursty (every authenticated request can trigger a /me lookup
	// in the absence of caching), so generous idle-conn count helps.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Auth DB connected")

	runMigrations(db)

	return db
}

func runMigrations(db *sql.DB) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);`,
		// (id) and (email UNIQUE) are auto-indexed; no extra indexes needed
		// for current query patterns. Adding (created_at) only when an
		// admin-style "list users by signup time" endpoint exists.
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			log.Fatalf("migration failed: %v\nstmt: %s", err, s)
		}
	}
	log.Println("Auth DB migrations applied")
}
