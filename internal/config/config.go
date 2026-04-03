package config

import (
	"os"
	"path/filepath"
)

// Default home: ~/.tomy
// Override with TOMY_HOME env var.

type Config struct {
	HomeDir       string // ~/.tomy (or $TOMY_HOME)
	StateDir      string // ~/.tomy/state
	WorkspacesDir string // ~/.tomy/workspaces
	PlannerDir    string // ~/.tomy/planner
	InboxDir      string // ~/.tomy/state/inbox
	PlansDir      string // ~/.tomy/state/plans
	NudgeQueueDir string // ~/.tomy/state/nudge_queue
	SessionPrefix string // tmux session name prefix
}

func Load() (*Config, error) {
	home := os.Getenv("TOMY_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		home = filepath.Join(userHome, ".tomy")
	}

	stateDir := filepath.Join(home, "state")
	cfg := &Config{
		HomeDir:       home,
		StateDir:      stateDir,
		WorkspacesDir: filepath.Join(home, "workspaces"),
		PlannerDir:    filepath.Join(home, "planner"),
		InboxDir:      filepath.Join(stateDir, "inbox"),
		PlansDir:      filepath.Join(stateDir, "plans"),
		NudgeQueueDir: filepath.Join(stateDir, "nudge_queue"),
		SessionPrefix: "tomy",
	}

	// Ensure directories exist
	for _, dir := range []string{cfg.StateDir, cfg.WorkspacesDir, cfg.PlannerDir, cfg.InboxDir, cfg.PlansDir, cfg.NudgeQueueDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
