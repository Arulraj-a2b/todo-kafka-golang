package database

import (
	"database/sql"
	"log"
	"os"
	"time"
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

	// Connection pool sizing — important when running N replicas behind a load
	// balancer. With 25 max conns/instance × 10 instances = 250 backend conns
	// to Postgres, well under PG's typical 100-500 limit. Beyond that, front
	// PG with PgBouncer in transaction-pooling mode.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Todo DB connected")

	// Idempotent migrations (CREATE TABLE / INDEX IF NOT EXISTS). Safe at
	// startup. For very large tables, switch to golang-migrate so index
	// creation is controlled (CONCURRENTLY, off-hours) rather than blocking
	// app boot.
	runMigrations(db)

	return db
}

func runMigrations(db *sql.DB) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS todos (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			priority TEXT NOT NULL DEFAULT 'medium',
			due_date TIMESTAMPTZ,
			tags TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);`,
		// Composite index on (user_id, created_at DESC) — covers GetTodos,
		// also covers cursor pagination via (created_at, id) keyset.
		`CREATE INDEX IF NOT EXISTS idx_todos_user_created
			ON todos (user_id, created_at DESC, id DESC);`,
		// Partial index for status-filtered list views (e.g., Kanban "In Progress").
		`CREATE INDEX IF NOT EXISTS idx_todos_user_status
			ON todos (user_id, status) WHERE status != 'deleted';`,
		// Partial index for upcoming-deadline queries.
		`CREATE INDEX IF NOT EXISTS idx_todos_user_due
			ON todos (user_id, due_date) WHERE due_date IS NOT NULL;`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			log.Fatalf("migration failed: %v\nstmt: %s", err, s)
		}
	}
	log.Println("Todo DB migrations applied")
}
