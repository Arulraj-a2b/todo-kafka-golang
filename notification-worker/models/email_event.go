package models

import (
	"encoding/json"
	"time"
)

// Event types carried by the email.events Kafka topic. The producer is
// email-scheduler. The shape mirrors the scheduler's models package — they
// can drift independently because both sides JSON-encode through this
// envelope.
const (
	EmailTypeOverdue      = "todo.overdue"
	EmailTypeDailySummary = "user.daily_summary"
)

type EmailEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
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
	Kind    string     `json:"kind"`
	DueDate *time.Time `json:"due_date,omitempty"`
}

type DailySummaryData struct {
	UserID     string                  `json:"user_id"`
	UserEmail  string                  `json:"user_email"`
	Date       string                  `json:"date"`
	Counts     DailySummaryCounts      `json:"counts"`
	Highlights []DailySummaryHighlight `json:"highlights,omitempty"`
}
