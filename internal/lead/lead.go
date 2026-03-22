package lead

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/orchestra/v1/internal/agent"
	"github.com/orchestra/v1/internal/project"
	"github.com/orchestra/v1/internal/tmux"
)

const LeadName = "lead"

// Start spawns the lead agent in the active project and attaches to it.
func Start(mgr *agent.Manager, proj *project.Project) error {
	// Determine the lead's working directory
	var workDir string
	if len(proj.Repos) > 0 {
		workDir = proj.Repos[0].Path
	} else {
		// No repos yet — use a temp dir under workspaces
		workDir = filepath.Join(os.TempDir(), "orchestra-lead-"+proj.Name)
		os.MkdirAll(workDir, 0755)
	}

	// Render the system prompt
	promptContent, err := renderPrompt(proj)
	if err != nil {
		return fmt.Errorf("render lead prompt: %w", err)
	}

	// Write prompt file to the working directory
	promptFile := filepath.Join(workDir, ".orchestra-lead-prompt.md")
	if err := os.WriteFile(promptFile, []byte(promptContent), 0644); err != nil {
		return fmt.Errorf("write lead prompt: %w", err)
	}

	// Spawn the lead agent
	a, err := mgr.Spawn(agent.SpawnOptions{
		Name:    LeadName,
		Role:    agent.RoleLead,
		Project: proj,
		Repo:    nil, // lead doesn't use a worktree — works in the repo directly
	})
	if err != nil {
		return err
	}

	// The lead was spawned in an isolated workspace. We need to:
	// 1. Kill the default claude session that was started
	// 2. cd to the actual project directory
	// 3. Restart claude with the system prompt file

	// First, exit the claude session that auto-started in the isolated workspace
	tmux.SendKeys(a.Session, "/exit")

	// Wait a moment for claude to exit
	// Then cd to the project dir and restart with system prompt
	tmux.SendKeys(a.Session, "cd "+workDir)

	// Build claude command with system prompt and add-dir for all repos
	claudeCmd := "claude --dangerously-skip-permissions --append-system-prompt-file " + promptFile
	for _, repo := range proj.Repos {
		claudeCmd += " --add-dir " + repo.Path
	}
	tmux.SendKeys(a.Session, claudeCmd)

	// Accept startup dialogs for the restarted session
	if err := tmux.AcceptStartupDialogs(a.Session); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-accept dialogs for lead: %v\n", err)
	}

	// Update the agent's work dir to the actual project dir
	mgr.Update(LeadName, func(a *agent.Agent) {
		a.WorkDir = workDir
	})

	fmt.Printf("Lead spawned for project %q (session: %s)\n", proj.Name, a.Session)

	// Auto-attach
	return mgr.Attach(LeadName)
}

// Stop kills the lead agent.
func Stop(mgr *agent.Manager) error {
	return mgr.Kill(LeadName)
}

// Attach connects to the lead's tmux session.
func Attach(mgr *agent.Manager) error {
	return mgr.Attach(LeadName)
}
