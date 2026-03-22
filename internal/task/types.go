package task

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusAssigned   Status = "assigned"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	AssignedTo  string    `json:"assigned_to,omitempty"`
	Result      string    `json:"result,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
