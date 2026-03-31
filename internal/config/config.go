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

	cfg := &Config{
		HomeDir:       home,
		StateDir:      filepath.Join(home, "state"),
		WorkspacesDir: filepath.Join(home, "workspaces"),
		PlannerDir:    filepath.Join(home, "planner"),
		SessionPrefix: "orch",
	}

	// Ensure directories exist
	for _, dir := range []string{cfg.StateDir, cfg.WorkspacesDir, cfg.PlannerDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
