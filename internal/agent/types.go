package agent

import "time"

type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Agent struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    Status    `json:"status"`
	TaskID    string    `json:"task_id,omitempty"`
	Session   string    `json:"session"`
	WorkDir   string    `json:"work_dir"`
	CreatedAt time.Time `json:"created_at"`
}
