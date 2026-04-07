package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tomy/v1/internal/git"
	"github.com/tomy/v1/internal/project"
	"github.com/tomy/v1/internal/state"
	"github.com/tomy/v1/internal/tmux"
	bolt "go.etcd.io/bbolt"
)

const bucket = "workers"

type Manager struct {
	db            *state.DB
	workspacesDir string
	sessionPrefix string
}

func NewManager(db *state.DB, workspacesDir, sessionPrefix string) *Manager {
	return &Manager{
		db:            db,
		workspacesDir: workspacesDir,
		sessionPrefix: sessionPrefix,
	}
}

func (m *Manager) SessionName(name string) string {
	return m.sessionPrefix + "-" + name
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

	// Check for duplicate
	workers, err := state.List[Worker](m.db, bucket)
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
	var baseBranch string
	branchName := "tomy/" + opts.Name

	for _, repo := range opts.Project.Repos {
		if repo.IsGitRepo {
			// Capture base branch from the first git repo
			if baseBranch == "" {
				if branch, err := git.CurrentBranch(repo.Path); err == nil {
					baseBranch = branch
				}
			}
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

	// Run setup commands for repos that have them
	workerIndex := 0
	for _, w := range workers {
		if w.ProjectID == opts.Project.ID {
			workerIndex++
		}
	}
	wtIdx := 0
	for _, repo := range opts.Project.Repos {
		if !repo.IsGitRepo {
			continue
		}
		if repo.SetupCommand != "" {
			if err := RunSetupCommand(repo, worktreeDirs[wtIdx], workspaceDir, opts.Name, workerIndex); err != nil {
				fmt.Fprintf(os.Stderr, "warning: setup for %s failed: %v\n", repo.Name, err)
			}
		}
		wtIdx++
	}

	// Worker home is the workspace root (CLAUDE.md lives here)
	workDir := workspaceDir

	if baseBranch == "" {
		baseBranch = "main"
	}

	// Write worker CLAUDE.md with operating instructions
	workerCLAUDE := renderWorkerCLAUDE(opts.Name, baseBranch)
	claudeMdPath := filepath.Join(workspaceDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMdPath, []byte(workerCLAUDE), 0644); err != nil {
		return nil, fmt.Errorf("write worker CLAUDE.md: %w", err)
	}

	// Write .claude/settings.json with UserPromptSubmit hook
	// so queued messages are injected at each turn boundary
	claudeDir := filepath.Join(workspaceDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return nil, fmt.Errorf("create .claude dir: %w", err)
	}
	hookSettings := fmt.Sprintf(`{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "tomy msg inbox %s --inject"
          }
        ]
      }
    ]
  }
}
`, opts.Name)
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(hookSettings), 0644); err != nil {
		return nil, fmt.Errorf("write hook settings: %w", err)
	}

	// Create tmux session
	if err := tmux.NewSession(session); err != nil {
		for _, d := range worktreeDirs {
			git.WorktreeRemove(d, d)
		}
		return nil, fmt.Errorf("create tmux session: %w", err)
	}

	// cd into workspace root (where CLAUDE.md is)
	if err := tmux.SendKeys(session, "cd "+workDir); err != nil {
		tmux.KillSession(session)
		return nil, fmt.Errorf("cd to workspace: %w", err)
	}

	// Build Claude command with --add-dir for each repo worktree
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
		ProjectID:    opts.Project.ID,
		Session:      session,
		WorkDir:      workDir,
		WorktreeDirs: worktreeDirs,
		BaseBranch:   baseBranch,
		CreatedAt:    time.Now(),
	}

	if err := m.db.Put(bucket, w.Name, w); err != nil {
		tmux.KillSession(session)
		return nil, err
	}

	return &w, nil
}

// Kill destroys a worker's tmux session, cleans up worktrees, and removes from registry.
func (m *Manager) Kill(name string) error {
	var w Worker
	if err := m.db.Get(bucket, name, &w); err != nil {
		return fmt.Errorf("worker %q not found", name)
	}

	session := m.SessionName(name)
	if tmux.HasSession(session) {
		tmux.KillSession(session)
	}
	// Clean up all worktrees
	for _, wtDir := range w.WorktreeDirs {
		git.WorktreeRemove(wtDir, wtDir)
	}

	return m.db.Delete(bucket, name)
}

// List returns all registered workers, with live tmux status check.
func (m *Manager) List() ([]Worker, error) {
	workers, err := state.List[Worker](m.db, bucket)
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
	var w Worker
	if err := m.db.Get(bucket, name, &w); err != nil {
		return nil, fmt.Errorf("worker %q not found", name)
	}
	if !tmux.HasSession(w.Session) {
		w.Status = StatusFailed
	}
	return &w, nil
}

// Update modifies a worker in-place using the provided function.
func (m *Manager) Update(name string, fn func(*Worker)) error {
	return m.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(name))
		if v == nil {
			return fmt.Errorf("worker %q not found", name)
		}
		var w Worker
		if err := json.Unmarshal(v, &w); err != nil {
			return err
		}
		fn(&w)
		data, err := json.Marshal(w)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), data)
	})
}

// Attach connects the user's terminal to a worker's tmux session.
func (m *Manager) Attach(name string) error {
	session := m.SessionName(name)
	if !tmux.HasSession(session) {
		return fmt.Errorf("worker %q has no active session", name)
	}
	return tmux.AttachSession(session)
}

// Assign writes the plan content to a temp file and delivers it to the worker's Claude session.
func (m *Manager) Assign(name string, content []byte) error {
	w, err := m.Get(name)
	if err != nil {
		return err
	}
	if !tmux.HasSession(w.Session) {
		return fmt.Errorf("worker %q session is not running", name)
	}

	// Write to a temp file for tmux delivery via cat
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("tomy-plan-%s.md", name))
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return fmt.Errorf("write temp plan file: %w", err)
	}

	// Deliver plan to worker session
	cmd := fmt.Sprintf("cat %s", tmpFile)
	return tmux.SendKeys(w.Session, cmd)
}

// renderWorkerCLAUDE generates the CLAUDE.md content for a worker.
func renderWorkerCLAUDE(workerName, baseBranch string) string {
	return fmt.Sprintf(`# Worker: %s

You are a worker agent in the Tomy system. You receive a plan containing tasks to execute.

## Your Environment

You are working in **git worktrees** — isolated copies of each project repo.
- Your branch is **tomy/%s** in every repo (branched from **%s**)
- Your worktrees are at ~/.tomy/workspaces/<project>/%s/<repo>/
- Changes you make here do NOT affect the original repos or other workers
- Use the repos added to your session (visible via your working directories)

## Operating Instructions

1. Read the plan carefully — it lists all your tasks with their IDs
2. Before starting each task, mark it as in-progress:
`+"```"+`
tomy task start <task-id>
`+"```"+`
3. Work through the task
4. After completing the task, mark it done:
`+"```"+`
tomy task done <task-id>
`+"```"+`
   This tracks progress — the planner can see your completion percentage.
5. If you are blocked, mark the task and message the planner:
`+"```"+`
tomy task block <task-id> --reason "describe the blocker"
tomy msg send planner "blocked on <task-id>: reason" --from %s
`+"```"+`
6. Commit your changes in each repo you modify
7. Push your branch: git push -u origin tomy/%s
8. Create a PR targeting %s: gh pr create --base %s --fill

## When All Tasks Are Done

When you mark the last task done, the plan and worker are automatically completed.
You can also finish everything at once:
`+"```"+`
tomy done %s
`+"```"+`

## Communication

Send messages to the planner:
`+"```"+`
tomy msg send planner "your message" --from %s
`+"```"+`

Check your inbox for messages:
`+"```"+`
tomy msg inbox %s
`+"```"+`

## Rules

- Focus on the assigned plan only — do not take on extra work
- Mark each task done as you complete it so progress is tracked
- If you are stuck or need clarification, mark the task blocked and message the planner
- Do NOT push directly to %s — always use a PR from your tomy/%s branch
`, workerName, workerName, baseBranch, workerName, workerName, workerName, baseBranch, baseBranch, workerName, workerName, workerName, baseBranch, workerName)
}
