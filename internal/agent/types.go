package agent

import "time"

type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Role string

const (
	RoleWorker Role = "worker"
	RoleLead   Role = "lead"
)

type Agent struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      Status    `json:"status"`
	Role        Role      `json:"role,omitempty"`
	TaskID      string    `json:"task_id,omitempty"`
	ProjectID   string    `json:"project_id,omitempty"`
	RepoName    string    `json:"repo_name,omitempty"`
	Session     string    `json:"session"`
	WorkDir     string    `json:"work_dir"`
	WorktreeDir string    `json:"worktree_dir,omitempty"` // set if git worktree was created
	CreatedAt   time.Time `json:"created_at"`
}
