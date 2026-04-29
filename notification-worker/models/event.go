package models

import "time"

const (
	ActionCreate = "create"
	ActionDelete = "delete"
	ActionUpdate = "update"
)

type Status string
type Priority string

type Todo struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Status    Status     `json:"status"`
	Priority  Priority   `json:"priority"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	Tags      []string   `json:"tags"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type TodoEvent struct {
	Action    string `json:"action"`
	Todo      Todo   `json:"todo"`
	UserEmail string `json:"user_email,omitempty"`
}
