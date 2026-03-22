package agent

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
		path:          filepath.Join(stateDir, "agents.json"),
		workspacesDir: workspacesDir,
		sessionPrefix: sessionPrefix,
	}
}

func (m *Manager) sessionName(name string) string {
	return m.sessionPrefix + "-" + name
}

func (m *Manager) loadAll() ([]Agent, error) {
	var agents []Agent
	if err := state.ReadJSON(m.path, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (m *Manager) saveAll(agents []Agent) error {
	return state.WriteJSON(m.path, agents)
}

// SpawnOptions configures how an agent is spawned.
type SpawnOptions struct {
	Name    string
	Role    Role
	Project *project.Project // nil = isolated workspace
	Repo    *project.Repo    // nil = isolated workspace
}

// Spawn creates a new agent with a tmux session running Claude Code.
func (m *Manager) Spawn(opts SpawnOptions) (*Agent, error) {
	// Check if agent already exists
	agents, err := m.loadAll()
	if err != nil {
		return nil, err
	}
	for _, a := range agents {
		if a.Name == opts.Name {
			return nil, fmt.Errorf("agent %q already exists", opts.Name)
		}
	}

	session := m.sessionName(opts.Name)

	// Determine working directory
	workDir, worktreeDir, err := m.resolveWorkDir(opts)
	if err != nil {
		return nil, err
	}

	// Create tmux session
	if err := tmux.NewSession(session); err != nil {
		return nil, fmt.Errorf("create tmux session: %w", err)
	}

	// cd into working directory
	if err := tmux.SendKeys(session, "cd "+workDir); err != nil {
		tmux.KillSession(session)
		return nil, fmt.Errorf("cd to workspace: %w", err)
	}

	// Launch Claude Code
	if err := tmux.SendKeys(session, "claude --dangerously-skip-permissions"); err != nil {
		tmux.KillSession(session)
		return nil, fmt.Errorf("launch claude: %w", err)
	}

	// Accept startup dialogs (workspace trust + bypass permissions warning)
	if err := tmux.AcceptStartupDialogs(session); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not auto-accept dialogs for %s: %v\n", opts.Name, err)
	}

	role := opts.Role
	if role == "" {
		role = RoleWorker
	}

	agent := Agent{
		ID:          opts.Name,
		Name:        opts.Name,
		Status:      StatusIdle,
		Role:        role,
		Session:     session,
		WorkDir:     workDir,
		WorktreeDir: worktreeDir,
		CreatedAt:   time.Now(),
	}

	if opts.Project != nil {
		agent.ProjectID = opts.Project.ID
	}
	if opts.Repo != nil {
		agent.RepoName = opts.Repo.Name
	}

	agents = append(agents, agent)
	if err := m.saveAll(agents); err != nil {
		tmux.KillSession(session)
		return nil, err
	}

	return &agent, nil
}

// resolveWorkDir determines where the agent should work.
// Returns (workDir, worktreeDir, error). worktreeDir is non-empty only if a worktree was created.
func (m *Manager) resolveWorkDir(opts SpawnOptions) (string, string, error) {
	// No repo → isolated workspace (original v1 behavior)
	if opts.Repo == nil {
		workDir := filepath.Join(m.workspacesDir, opts.Name)
		if err := os.MkdirAll(workDir, 0755); err != nil {
			return "", "", fmt.Errorf("create workspace: %w", err)
		}
		return workDir, "", nil
	}

	// Repo is a git repo → create worktree
	if opts.Repo.IsGitRepo {
		projectName := "default"
		if opts.Project != nil {
			projectName = opts.Project.Name
		}
		worktreePath := filepath.Join(m.workspacesDir, projectName, opts.Name)

		branchName := "orch/" + opts.Name
		if err := git.WorktreeAdd(opts.Repo.Path, worktreePath, branchName); err != nil {
			return "", "", fmt.Errorf("create worktree: %w", err)
		}
		return worktreePath, worktreePath, nil
	}

	// Repo is not a git repo → work directly in the repo path
	return opts.Repo.Path, "", nil
}

// Kill destroys an agent's tmux session, cleans up worktrees, and removes from registry.
func (m *Manager) Kill(name string) error {
	agents, err := m.loadAll()
	if err != nil {
		return err
	}

	found := false
	var remaining []Agent
	for _, a := range agents {
		if a.Name == name {
			found = true
			// Kill tmux session
			session := m.sessionName(name)
			if tmux.HasSession(session) {
				tmux.KillSession(session)
			}
			// Clean up worktree if one was created
			if a.WorktreeDir != "" && a.RepoName != "" {
				// We need the repo path to remove the worktree.
				// The worktree remove command works with just the path.
				git.WorktreeRemove(a.WorkDir, a.WorktreeDir)
			}
		} else {
			remaining = append(remaining, a)
		}
	}

	if !found {
		return fmt.Errorf("agent %q not found", name)
	}

	return m.saveAll(remaining)
}

// List returns all registered agents, with live tmux status check.
func (m *Manager) List() ([]Agent, error) {
	agents, err := m.loadAll()
	if err != nil {
		return nil, err
	}

	for i := range agents {
		if !tmux.HasSession(agents[i].Session) {
			agents[i].Status = StatusFailed
		}
	}

	return agents, nil
}

// Get returns a single agent by name.
func (m *Manager) Get(name string) (*Agent, error) {
	agents, err := m.loadAll()
	if err != nil {
		return nil, err
	}
	for _, a := range agents {
		if a.Name == name {
			if !tmux.HasSession(a.Session) {
				a.Status = StatusFailed
			}
			return &a, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found", name)
}

// Update modifies an agent in-place using the provided function.
func (m *Manager) Update(name string, fn func(*Agent)) error {
	agents, err := m.loadAll()
	if err != nil {
		return err
	}
	for i := range agents {
		if agents[i].Name == name {
			fn(&agents[i])
			return m.saveAll(agents)
		}
	}
	return fmt.Errorf("agent %q not found", name)
}

// Attach connects the user's terminal to an agent's tmux session.
func (m *Manager) Attach(name string) error {
	session := m.sessionName(name)
	if !tmux.HasSession(session) {
		return fmt.Errorf("agent %q has no active session", name)
	}
	return tmux.AttachSession(session)
}

// Assign sends a task prompt to an agent's Claude Code session.
func (m *Manager) Assign(name string, prompt string) error {
	agent, err := m.Get(name)
	if err != nil {
		return err
	}
	if !tmux.HasSession(agent.Session) {
		return fmt.Errorf("agent %q session is not running", name)
	}

	promptFile := filepath.Join(agent.WorkDir, ".task-prompt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write prompt file: %w", err)
	}

	cmd := fmt.Sprintf("cat %s", promptFile)
	return tmux.SendKeys(agent.Session, cmd)
}
