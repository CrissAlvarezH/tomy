package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	RootDir       string // Project root (where orchestra is run from)
	StateDir      string // state/ directory for JSON files
	WorkspacesDir string // workspaces/ directory for agent working dirs
	SessionPrefix string // tmux session name prefix
}

func Load() (*Config, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		RootDir:       root,
		StateDir:      filepath.Join(root, "state"),
		WorkspacesDir: filepath.Join(root, "workspaces"),
		SessionPrefix: "orch",
	}

	// Ensure directories exist
	for _, dir := range []string{cfg.StateDir, cfg.WorkspacesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
