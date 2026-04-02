package plan

import "time"

type Status string

const (
	StatusDraft      Status = "draft"
	StatusAssigned   Status = "assigned"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

type Plan struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentFile string    `json:"content_file"`
	WorkerName  string    `json:"worker_name,omitempty"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
