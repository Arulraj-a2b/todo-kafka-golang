package store

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"email-scheduler/models"

	"github.com/lib/pq"
)

// DB bundles the two read-only Postgres connections the scheduler needs:
//   AuthDB — users table (for email lookups)
//   TodoDB — todos table (for overdue scan + summary counts)
//
// They're separate databases on the same Postgres instance.
type DB struct {
	AuthDB *sql.DB
	TodoDB *sql.DB
}

func InitDB() *DB {
	auth := mustOpen(os.Getenv("AUTH_DATABASE_URL"), "AUTH_DATABASE_URL")
	todo := mustOpen(os.Getenv("TODO_DATABASE_URL"), "TODO_DATABASE_URL")
	return &DB{AuthDB: auth, TodoDB: todo}
}

func mustOpen(connStr, varName string) *sql.DB {
	if connStr == "" {
		slog.Error(varName + " not set")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("sql.Open", "err", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		slog.Error("db.Ping", "err", err, "var", varName)
		os.Exit(1)
	}
	return db
}

// FetchOverdueTodos returns todos whose due_date is in the past and which
// aren't completed/deleted, capped at limit. Uses idx_todos_user_due partial
// index for fast scans even on huge tables.
func (d *DB) FetchOverdueTodos(ctx context.Context, limit int) ([]models.Todo, error) {
	q := `
		SELECT id, user_id, title, status, priority, due_date, tags, created_at, updated_at
		FROM todos
		WHERE due_date IS NOT NULL
		  AND due_date < NOW()
		  AND status NOT IN ('completed', 'deleted')
		  AND due_date > NOW() - INTERVAL '30 days'
		ORDER BY due_date ASC
		LIMIT $1`
	rows, err := d.TodoDB.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Todo{}
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Status, &t.Priority,
			&t.DueDate, pq.Array(&t.Tags), &t.CreatedAt, &t.UpdatedAt); err != nil {
			slog.WarnContext(ctx, "scan overdue todo", "err", err)
			continue
		}
		if t.Tags == nil {
			t.Tags = []string{}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// LookupUserEmails returns a map of userID → email for the given IDs.
// Single round-trip via ANY($1::text[]).
func (d *DB) LookupUserEmails(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}
	rows, err := d.AuthDB.QueryContext(ctx,
		`SELECT id, email FROM users WHERE id = ANY($1::text[])`,
		pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string, len(userIDs))
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			continue
		}
		out[id] = email
	}
	return out, rows.Err()
}

// UserIDsWithActiveTodos returns one batch of user IDs that have at least
// one non-deleted, non-completed todo. Cursor-paginated by user_id ascending.
// Pass empty afterID to get the first batch.
func (d *DB) UserIDsWithActiveTodos(ctx context.Context, afterID string, limit int) ([]string, error) {
	q := `
		SELECT DISTINCT user_id
		FROM todos
		WHERE status NOT IN ('completed', 'deleted')
		  AND user_id > $1
		ORDER BY user_id ASC
		LIMIT $2`
	rows, err := d.TodoDB.QueryContext(ctx, q, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// SummaryCount is the aggregate per user produced by the daily summary query.
type SummaryCount struct {
	UserID             string
	Pending            int
	InProgress         int
	DueToday           int
	Overdue            int
	CompletedYesterday int
}

// SummaryCountsForUsers aggregates per-user counts in one query for a batch.
func (d *DB) SummaryCountsForUsers(ctx context.Context, userIDs []string) (map[string]SummaryCount, error) {
	if len(userIDs) == 0 {
		return map[string]SummaryCount{}, nil
	}
	q := `
		SELECT user_id,
		       count(*) FILTER (WHERE status='pending')                                           AS pending,
		       count(*) FILTER (WHERE status='in_progress')                                       AS in_progress,
		       count(*) FILTER (WHERE due_date IS NOT NULL AND due_date::date = CURRENT_DATE)     AS due_today,
		       count(*) FILTER (WHERE due_date IS NOT NULL AND due_date < NOW() AND status NOT IN ('completed','deleted')) AS overdue,
		       count(*) FILTER (WHERE status='completed' AND updated_at >= NOW() - INTERVAL '1 day') AS completed_yesterday
		FROM todos
		WHERE user_id = ANY($1::text[])
		GROUP BY user_id`
	rows, err := d.TodoDB.QueryContext(ctx, q, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]SummaryCount, len(userIDs))
	for rows.Next() {
		var s SummaryCount
		if err := rows.Scan(&s.UserID, &s.Pending, &s.InProgress, &s.DueToday, &s.Overdue, &s.CompletedYesterday); err != nil {
			continue
		}
		out[s.UserID] = s
	}
	return out, rows.Err()
}

// HighlightsForUser returns up to N todos to feature in the daily summary email
// (overdue + due-today, ordered by oldest due_date first).
func (d *DB) HighlightsForUser(ctx context.Context, userID string, limit int) ([]models.DailySummaryHighlight, error) {
	q := `
		SELECT id, title, due_date,
		       CASE
		         WHEN due_date < NOW() AND status NOT IN ('completed','deleted') THEN 'overdue'
		         WHEN due_date::date = CURRENT_DATE THEN 'due_today'
		         ELSE 'other'
		       END AS kind
		FROM todos
		WHERE user_id = $1
		  AND due_date IS NOT NULL
		  AND status NOT IN ('completed', 'deleted')
		  AND (due_date < NOW() OR due_date::date = CURRENT_DATE)
		ORDER BY due_date ASC
		LIMIT $2`
	rows, err := d.TodoDB.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.DailySummaryHighlight{}
	for rows.Next() {
		var h models.DailySummaryHighlight
		if err := rows.Scan(&h.ID, &h.Title, &h.DueDate, &h.Kind); err == nil {
			out = append(out, h)
		}
	}
	return out, rows.Err()
}

func (d *DB) Close() {
	if d.AuthDB != nil {
		_ = d.AuthDB.Close()
	}
	if d.TodoDB != nil {
		_ = d.TodoDB.Close()
	}
}
