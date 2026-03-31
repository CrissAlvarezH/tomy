package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orchestra/v1/internal/git"
	"github.com/orchestra/v1/internal/project"
	"github.com/orchestra/v1/internal/state"
	"github.com/orchestra/v1/internal/tmux"
)

type Manager struct {
	path          string
	workspacesDir string
	sessionPrefix string
}

func NewManager(stateDir, workspacesDir, sessionPrefix string) *Manager {
	return &Manager{
		path:          filepath.Join(stateDir, "workers.json"),
		workspacesDir: workspacesDir,
		sessionPrefix: sessionPrefix,
	}
}

func (m *Manager) SessionName(name string) string {
	return m.sessionPrefix + "-" + name
}

func (m *Manager) loadAll() ([]Worker, error) {
	var workers []Worker
	if err := state.ReadJSON(m.path, &workers); err != nil {
		return nil, err
	}
	return workers, nil
}

func (m *Manager) saveAll(workers []Worker) error {
	return state.WriteJSON(m.path, workers)
}

// SpawnOptions configures how a worker is spawned.
type SpawnOptions struct {
	Name    string
	Project *project.Project
}

// Spawn creates a new worker with a tmux session running Claude Code.
// It creates a worktree for each git repo in the project.
func (m *Manager) Spawn(opts SpawnOptions) (*Worker, error) {
	if opts.Project == nil {
		return nil, fmt.Errorf("project is required to spawn a worker")
	}

	workers, err := m.loadAll()
	if err != nil {
		return nil, err
	}
	for _, w := range workers {
		if w.Name == opts.Name {
			return nil, fmt.Errorf("worker %q already exists", opts.Name)
		}
	}

	session := m.SessionName(opts.Name)

	// Create workspace directory for this worker
	workspaceDir := filepath.Join(m.workspacesDir, opts.Project.Name, opts.Name)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// Create worktrees for each git repo in the project
	var worktreeDirs []string
	var addDirs []string
	branchName := "orch/" + opts.Name

	for _, repo := range opts.Project.Repos {
		if repo.IsGitRepo {
			wtPath := filepath.Join(workspaceDir, repo.Name)
			if err := git.WorktreeAdd(repo.Path, wtPath, branchName); err != nil {
				// Clean up already-created worktrees on failure
				for _, d := range worktreeDirs {
					git.WorktreeRemove(d, d)
				}
				return nil, fmt.Errorf("create worktree for %s: %w", repo.Name, err)
			}
			worktreeDirs = append(worktreeDirs, wtPath)
			addDirs = append(addDirs, wtPath)
		} else {
			// Non-git repos: reference the original path
			addDirs = append(addDirs, repo.Path)
		}
	}

	// Determine primary workdir (first worktree, or first repo, or workspace root)
	workDir := workspaceDir
	if len(addDirs) > 0 {
		workDir = addDirs[0]
	}

	// Create tmux session
	if err := tmux.NewSession(session); err != nil {
		for _, d := range worktreeDirs {
			git.WorktreeRemove(d, d)
		}
		return nil, fmt.Errorf("create tmux session: %w", err)
	}

	// cd into primary working directory
	if err := tmux.SendKeys(session, "cd "+workDir); err != nil {
		tmux.KillSession(session)
		return nil, fmt.Errorf("cd to workspace: %w", err)
	}

	// Build Claude command with --add-dir for each repo
	claudeCmd := "claude --dangerously-skip-permissions"
	for _, dir := range addDirs {
		claudeCmd += " --add-dir " + dir
	}

	if err := tmux.SendKeys(session, claudeCmd); err != nil {
		tmux.KillSession(session)
		return nil, fmt.Errorf("launch claude: %w", err)
	}

	// Accept startup dialogs
	if err := tmux.AcceptStartupDialogs(session); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-accept dialogs for %s: %v\n", opts.Name, err)
	}

	w := Worker{
		ID:           opts.Name,
		Name:         opts.Name,
		Status:       StatusIdle,
		TaskID:       "",
		ProjectID:    opts.Project.ID,
		Session:      session,
		WorkDir:      workDir,
		WorktreeDirs: worktreeDirs,
		CreatedAt:    time.Now(),
	}

	workers = append(workers, w)
	if err := m.saveAll(workers); err != nil {
		tmux.KillSession(session)
		return nil, err
	}

	return &w, nil
}

// Kill destroys a worker's tmux session, cleans up worktrees, and removes from registry.
func (m *Manager) Kill(name string) error {
	workers, err := m.loadAll()
	if err != nil {
		return err
	}

	found := false
	var remaining []Worker
	for _, w := range workers {
		if w.Name == name {
			found = true
			session := m.SessionName(name)
			if tmux.HasSession(session) {
				tmux.KillSession(session)
			}
			// Clean up all worktrees
			for _, wtDir := range w.WorktreeDirs {
				git.WorktreeRemove(wtDir, wtDir)
			}
		} else {
			remaining = append(remaining, w)
		}
	}

	if !found {
		return fmt.Errorf("worker %q not found", name)
	}

	return m.saveAll(remaining)
}

// List returns all registered workers, with live tmux status check.
func (m *Manager) List() ([]Worker, error) {
	workers, err := m.loadAll()
	if err != nil {
		return nil, err
	}

	for i := range workers {
		if !tmux.HasSession(workers[i].Session) {
			workers[i].Status = StatusFailed
		}
	}

	return workers, nil
}

// Get returns a single worker by name.
func (m *Manager) Get(name string) (*Worker, error) {
	workers, err := m.loadAll()
	if err != nil {
		return nil, err
	}
	for _, w := range workers {
		if w.Name == name {
			if !tmux.HasSession(w.Session) {
				w.Status = StatusFailed
			}
			return &w, nil
		}
	}
	return nil, fmt.Errorf("worker %q not found", name)
}

// Update modifies a worker in-place using the provided function.
func (m *Manager) Update(name string, fn func(*Worker)) error {
	workers, err := m.loadAll()
	if err != nil {
		return err
	}
	for i := range workers {
		if workers[i].Name == name {
			fn(&workers[i])
			return m.saveAll(workers)
		}
	}
	return fmt.Errorf("worker %q not found", name)
}

// Attach connects the user's terminal to a worker's tmux session.
func (m *Manager) Attach(name string) error {
	session := m.SessionName(name)
	if !tmux.HasSession(session) {
		return fmt.Errorf("worker %q has no active session", name)
	}
	return tmux.AttachSession(session)
}

// Assign sends a task prompt to a worker's Claude Code session.
func (m *Manager) Assign(name string, prompt string) error {
	w, err := m.Get(name)
	if err != nil {
		return err
	}
	if !tmux.HasSession(w.Session) {
		return fmt.Errorf("worker %q session is not running", name)
	}

	promptFile := filepath.Join(w.WorkDir, ".task-prompt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write prompt file: %w", err)
	}

	cmd := fmt.Sprintf("cat %s", promptFile)
	return tmux.SendKeys(w.Session, cmd)
}
