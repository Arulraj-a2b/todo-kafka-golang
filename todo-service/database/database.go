package database

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"todo-service/internal/obs"

	"github.com/XSAM/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitDB() *sql.DB {
	connStr := os.Getenv("TODO_DATABASE_URL")
	if connStr == "" {
		slog.Error("TODO_DATABASE_URL not set")
		os.Exit(1)
	}

	db, err := otelsql.Open("postgres", connStr,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
	)
	if err != nil {
		slog.Error("sql.Open", "err", err)
		os.Exit(1)
	}

	// Pool sizing — important when running N replicas behind a load balancer.
	// 25 max conns/instance × 10 instances = 250 backend conns to Postgres,
	// well under PG's typical 100-500 limit. Beyond that, front PG with
	// PgBouncer in transaction-pooling mode.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err = db.Ping(); err != nil {
		slog.Error("db.Ping", "err", err)
		os.Exit(1)
	}
	slog.Info("todo db connected")

	go sampleDBStats(db)

	runMigrations(db)
	return db
}

func sampleDBStats(db *sql.DB) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for range t.C {
		s := db.Stats()
		obs.DBConnectionsInUse.Set(float64(s.InUse))
		obs.DBConnectionsIdle.Set(float64(s.Idle))
	}
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
		`CREATE INDEX IF NOT EXISTS idx_todos_user_created
			ON todos (user_id, created_at DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_todos_user_status
			ON todos (user_id, status) WHERE status != 'deleted';`,
		`CREATE INDEX IF NOT EXISTS idx_todos_user_due
			ON todos (user_id, due_date) WHERE due_date IS NOT NULL;`,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			slog.Error("migration failed", "err", err, "stmt", s)
			os.Exit(1)
		}
	}
	slog.Info("todo db migrations applied")
}
