package worker

import "time"

type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Worker struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Status       Status    `json:"status"`
	TaskID       string    `json:"task_id,omitempty"`
	ProjectID    string    `json:"project_id"`
	Session      string    `json:"session"`
	WorkDir      string    `json:"work_dir"`
	WorktreeDirs []string  `json:"worktree_dirs,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
