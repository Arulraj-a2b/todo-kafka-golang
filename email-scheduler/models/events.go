package models

import (
	"encoding/json"
	"time"
)

const (
	TypeOverdue       = "todo.overdue"
	TypeDailySummary  = "user.daily_summary"
)

// EmailEvent is the envelope on the email.events Kafka topic. The notification-worker
// peeks at Type and decodes Data into the matching payload.
type EmailEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Todo struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Priority  string     `json:"priority"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	Tags      []string   `json:"tags"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type OverdueData struct {
	Todo             Todo   `json:"todo"`
	UserEmail        string `json:"user_email"`
	OverdueBySeconds int64  `json:"overdue_by_seconds"`
}

type DailySummaryCounts struct {
	Pending            int `json:"pending"`
	InProgress         int `json:"in_progress"`
	DueToday           int `json:"due_today"`
	Overdue            int `json:"overdue"`
	CompletedYesterday int `json:"completed_yesterday"`
}

type DailySummaryHighlight struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Kind    string     `json:"kind"` // "overdue" | "due_today"
	DueDate *time.Time `json:"due_date,omitempty"`
}

type DailySummaryData struct {
	UserID     string                  `json:"user_id"`
	UserEmail  string                  `json:"user_email"`
	Date       string                  `json:"date"` // YYYY-MM-DD
	Counts     DailySummaryCounts      `json:"counts"`
	Highlights []DailySummaryHighlight `json:"highlights,omitempty"`
}
