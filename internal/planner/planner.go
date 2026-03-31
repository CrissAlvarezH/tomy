package planner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/orchestra/v1/internal/tmux"
	"github.com/orchestra/v1/internal/project"
	"github.com/orchestra/v1/internal/worker"
)

const PlannerName = "planner"

// Start spawns the planner session for the given project and attaches to it.
func Start(mgr *worker.Manager, proj *project.Project) error {
	// Determine the planner's working directory
	var workDir string
	if len(proj.Repos) > 0 {
		workDir = proj.Repos[0].Path
	} else {
		workDir = filepath.Join(os.TempDir(), "orchestra-planner-"+proj.Name)
		os.MkdirAll(workDir, 0755)
	}

	// Render the system prompt
	promptContent, err := renderPrompt(proj)
	if err != nil {
		return fmt.Errorf("render planner prompt: %w", err)
	}

	// Write prompt file to the working directory
	promptFile := filepath.Join(workDir, ".orchestra-planner-prompt.md")
	if err := os.WriteFile(promptFile, []byte(promptContent), 0644); err != nil {
		return fmt.Errorf("write planner prompt: %w", err)
	}

	// Spawn the planner as a worker entry (for session tracking)
	session := mgr.SessionName(PlannerName)

	// Check if session already exists
	if tmux.HasSession(session) {
		fmt.Printf("Planner already running for project %q (session: %s)\n", proj.Name, session)
		return mgr.Attach(PlannerName)
	}

	// Create tmux session
	if err := tmux.NewSession(session); err != nil {
		return fmt.Errorf("create tmux session: %w", err)
	}

	// cd into the project's first repo
	tmux.SendKeys(session, "cd "+workDir)

	// Build claude command with system prompt and --add-dir for all repos
	claudeCmd := "claude --dangerously-skip-permissions --append-system-prompt-file " + promptFile
	for _, repo := range proj.Repos {
		claudeCmd += " --add-dir " + repo.Path
	}
	tmux.SendKeys(session, claudeCmd)

	// Accept startup dialogs
	if err := tmux.AcceptStartupDialogs(session); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-accept dialogs for planner: %v\n", err)
	}

	fmt.Printf("Planner spawned for project %q (session: %s)\n", proj.Name, session)

	// Auto-attach
	return tmux.AttachSession(session)
}

// Stop kills the planner's tmux session.
func Stop(mgr *worker.Manager) error {
	session := mgr.SessionName(PlannerName)
	if !tmux.HasSession(session) {
		return fmt.Errorf("planner is not running")
	}
	return tmux.KillSession(session)
}

// Attach connects to the planner's tmux session.
func Attach(mgr *worker.Manager) error {
	session := mgr.SessionName(PlannerName)
	if !tmux.HasSession(session) {
		return fmt.Errorf("planner is not running")
	}
	return tmux.AttachSession(session)
}
