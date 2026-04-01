package config

import (
	"os"
	"path/filepath"
)

// Default home: ~/.orchestra
// Override with ORCHESTRA_HOME env var.

type Config struct {
	HomeDir       string // ~/.orchestra (or $ORCHESTRA_HOME)
	StateDir      string // ~/.orchestra/state
	WorkspacesDir string // ~/.orchestra/workspaces
	PlannerDir    string // ~/.orchestra/planner
	InboxDir      string // ~/.orchestra/state/inbox
	PlansDir      string // ~/.orchestra/state/plans
	SessionPrefix string // tmux session name prefix
}

func Load() (*Config, error) {
	home := os.Getenv("ORCHESTRA_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		home = filepath.Join(userHome, ".orchestra")
	}

	stateDir := filepath.Join(home, "state")
	cfg := &Config{
		HomeDir:       home,
		StateDir:      stateDir,
		WorkspacesDir: filepath.Join(home, "workspaces"),
		PlannerDir:    filepath.Join(home, "planner"),
		InboxDir:      filepath.Join(stateDir, "inbox"),
		PlansDir:      filepath.Join(stateDir, "plans"),
		SessionPrefix: "orch",
	}

	// Ensure directories exist
	for _, dir := range []string{cfg.StateDir, cfg.WorkspacesDir, cfg.PlannerDir, cfg.InboxDir, cfg.PlansDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
