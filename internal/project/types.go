package project

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Repos     []Repo    `json:"repos"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Repo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsGitRepo    bool   `json:"is_git_repo"`
	SetupCommand string `json:"setup_command"`
}

type ActiveProject struct {
	ProjectID string `json:"project_id"`
}
