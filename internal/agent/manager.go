package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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

// Spawn creates a new agent with a tmux session running Claude Code.
func (m *Manager) Spawn(name string) (*Agent, error) {
	// Check if agent already exists
	agents, err := m.loadAll()
	if err != nil {
		return nil, err
	}
	for _, a := range agents {
		if a.Name == name {
			return nil, fmt.Errorf("agent %q already exists", name)
		}
	}

	session := m.sessionName(name)
	workDir := filepath.Join(m.workspacesDir, name)

	// Create workspace directory
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// Create tmux session
	if err := tmux.NewSession(session); err != nil {
		return nil, fmt.Errorf("create tmux session: %w", err)
	}

	// cd into workspace
	if err := tmux.SendKeys(session, "cd "+workDir); err != nil {
		tmux.KillSession(session) // cleanup on failure
		return nil, fmt.Errorf("cd to workspace: %w", err)
	}

	// Launch Claude Code
	if err := tmux.SendKeys(session, "claude --dangerously-skip-permissions"); err != nil {
		tmux.KillSession(session)
		return nil, fmt.Errorf("launch claude: %w", err)
	}

	agent := Agent{
		ID:        name, // use name as ID for simplicity in v1
		Name:      name,
		Status:    StatusIdle,
		Session:   session,
		WorkDir:   workDir,
		CreatedAt: time.Now(),
	}

	agents = append(agents, agent)
	if err := m.saveAll(agents); err != nil {
		tmux.KillSession(session)
		return nil, err
	}

	return &agent, nil
}

// Kill destroys an agent's tmux session and removes it from the registry.
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
			session := m.sessionName(name)
			if tmux.HasSession(session) {
				tmux.KillSession(session)
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

	// Check which sessions are actually alive
	for i := range agents {
		if !tmux.HasSession(agents[i].Session) {
			agents[i].Status = StatusFailed // session died
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

	// Write prompt to a file in the agent's workspace so it's not truncated by tmux
	promptFile := filepath.Join(agent.WorkDir, ".task-prompt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write prompt file: %w", err)
	}

	// Send the prompt content via tmux send-keys
	// For long prompts, we pipe the file content
	cmd := fmt.Sprintf("cat %s", promptFile)
	return tmux.SendKeys(agent.Session, cmd)
}
