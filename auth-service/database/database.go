package database

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"auth-service/internal/obs"

	"github.com/XSAM/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitDB() *sql.DB {
	connStr := os.Getenv("AUTH_DATABASE_URL")
	if connStr == "" {
		slog.Error("AUTH_DATABASE_URL not set")
		os.Exit(1)
	}

	// otelsql.Open wraps the driver so every Exec/Query/QueryRow gets a span.
	db, err := otelsql.Open("postgres", connStr,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
	)
	if err != nil {
		slog.Error("sql.Open", "err", err)
		os.Exit(1)
	}

	// Pool sizing — see comment in todo-service/database/database.go.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err = db.Ping(); err != nil {
		slog.Error("db.Ping", "err", err)
		os.Exit(1)
	}

	slog.Info("auth db connected")

	// Sample DBStats every 10s into Prometheus gauges.
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
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);`,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			slog.Error("migration failed", "err", err, "stmt", s)
			os.Exit(1)
		}
	}
	slog.Info("auth db migrations applied")
}
