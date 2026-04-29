package handlers

import (
	"time"
	"todo-service/models"
)

type TodoRequest struct {
	Title    string          `json:"title" binding:"required" example:"Buy milk"`
	Status   models.Status   `json:"status" binding:"required" example:"pending" enums:"pending,in_progress,completed,deleted"`
	Priority models.Priority `json:"priority,omitempty" example:"medium" enums:"low,medium,high"`
	DueDate  *time.Time      `json:"due_date,omitempty" swaggertype:"string" format:"date-time" example:"2026-05-01T09:00:00Z"`
	Tags     []string        `json:"tags,omitempty" example:"home,errand"`
}

// TodosListResponse is the cursor-paginated response shape for GET /todos.
// next_cursor is empty when has_more is false.
type TodosListResponse struct {
	Todos      []models.Todo `json:"todos"`
	NextCursor string        `json:"next_cursor,omitempty" example:"eyJjIjoiMjAyNi0wNC0yOFQxMjowMDowMFoiLCJpIjoiYWJjIn0="`
	HasMore    bool          `json:"has_more" example:"true"`
}

type MessageResponse struct {
	Message string `json:"message" example:"todo deleted"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"invalid request body"`
}

type ImportResponse struct {
	Imported int      `json:"imported" example:"42"`
	Failed   int      `json:"failed" example:"3"`
	Errors   []string `json:"errors" example:"row 3: invalid status \"in_progresssss\""`
}
