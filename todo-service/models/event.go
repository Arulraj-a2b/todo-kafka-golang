package models

const (
	ActionCreate = "create"
	ActionDelete = "delete"
	ActionUpdate = "update"
)

type TodoEvent struct {
	Action           string `json:"action"`
	Todo             Todo   `json:"todo"`
	UserEmail        string `json:"user_email,omitempty"`
	SkipNotification bool   `json:"skip_notification,omitempty"`
}
