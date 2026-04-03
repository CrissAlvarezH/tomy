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
	PlanID       string    `json:"plan_id,omitempty"`
	ProjectID    string    `json:"project_id"`
	Session      string    `json:"session"`
	WorkDir      string    `json:"work_dir"`
	WorktreeDirs []string  `json:"worktree_dirs,omitempty"`
	BaseBranch   string    `json:"base_branch,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
