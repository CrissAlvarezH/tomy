# Orchestra v1 — Multi-Agent Claude Code Orchestrator

A CLI tool that manages projects with multiple repos, spawns Claude Code workers in tmux sessions, and coordinates work through a Planner session. Inspired by [Gas Town](https://github.com/steveyegge/gastown).

Zero external dependencies — Go stdlib only.

## Prerequisites

- **Go** 1.22+ — `brew install go`
- **tmux** — `brew install tmux`
- **Claude Code** — installed and authenticated (`claude` available in PATH)
- **GitHub CLI** — `brew install gh` (workers use it to create PRs)

## Quick Start

```bash
# Build
make build

# Create a project and add your repos
orchestra project create my-app
orchestra repo add /path/to/backend --name backend
orchestra repo add /path/to/frontend --name frontend

# Start the planner — you'll plan work interactively with it
orchestra planner start
```

Inside the planner session, discuss your goal. The planner will create tasks, spawn workers, and assign work — all using `orchestra` commands. Workers get git worktrees for all project repos and create PRs targeting `develop` when done.

Detach from tmux with `Ctrl+B` then `D`. Re-attach with `orchestra planner attach`.

## Installation

```bash
make install    # copies binary to ~/.local/bin/orchestra
make uninstall  # removes it
```

Make sure `~/.local/bin` is in your PATH.

## Concepts

| Concept | Description |
|---------|-------------|
| **Project** | A named group (e.g. "my-ecommerce"). Contains repos and tasks. |
| **Repo** | A directory/git repo within a project. Workers get worktrees of each repo. |
| **Planner** | An interactive Claude session where you plan work. It spawns workers and assigns tasks. |
| **Worker** | A Claude Code session in tmux with git worktrees for all project repos. Creates PRs to `develop` when done. |
| **Task** | A unit of work with a title and description. Assigned to one worker. |

## Commands

### Project Management

```bash
orchestra project create <name>            # Create a project (auto-set as active)
orchestra project list                     # List all projects
orchestra project set <name>               # Switch active project
orchestra project status                   # Show active project details
```

### Repo Management

```bash
orchestra repo add <path> [--name <name>]  # Add a repo to active project
orchestra repo list                        # List repos in active project
orchestra repo remove <name>               # Remove a repo
```

### Planner (Interactive Orchestrator)

```bash
orchestra planner start                    # Select project + spawn planner + auto-attach
orchestra planner attach                   # Re-attach to planner's session
orchestra planner stop                     # Kill the planner
```

### Worker Management

```bash
orchestra worker spawn <name>              # Spawn worker (worktrees for all project repos)
orchestra worker list                      # Show all workers with status
orchestra worker kill <name>               # Kill worker + clean up worktrees
orchestra worker attach <name>             # Attach to worker's tmux session
```

### Task Management

```bash
orchestra task create --title "..." --desc "..."   # Create a new task
orchestra task list                                 # List all tasks
orchestra task status <task-id>                     # Show task details
orchestra task assign <task-id> <worker-name>       # Assign task to an idle worker
```

### All-in-One

```bash
orchestra run --title "..." --desc "..." [--worker <name>]
```

Creates a task, spawns a worker, and assigns in one command.

## How It Works

```
orchestra project create my-ecommerce
orchestra repo add /path/to/backend --name backend
orchestra repo add /path/to/frontend --name frontend
orchestra planner start
```

The planner spawns in the first repo's directory with a system prompt listing all repos and available commands. You discuss the goal, and the planner runs:

```
orchestra task create --title "Add user model" --desc "..."
orchestra worker spawn add-user-model
orchestra task assign abc123 add-user-model
```

Workers get **git worktrees** for every repo in the project — isolated branches of each codebase:

```
add-user-model/
  ├── backend/   (worktree, branch: orch/add-user-model)
  └── frontend/  (worktree, branch: orch/add-user-model)
```

When a worker finishes, it commits, pushes, and creates a PR targeting `develop` via `gh pr create`.

## Project Structure

```
clone-v1/
├── cmd/
│   └── main.go                # CLI entry point — subcommand router
├── internal/
│   ├── worker/
│   │   ├── types.go           # Worker struct (status, project, worktrees)
│   │   └── manager.go         # Spawn (with multi-repo worktrees), kill, list, assign
│   ├── project/
│   │   ├── types.go           # Project and Repo structs
│   │   └── store.go           # Project CRUD, repo add/remove, active project
│   ├── planner/
│   │   ├── planner.go         # Planner start/stop/attach lifecycle
│   │   └── prompt.go          # System prompt template for the planner
│   ├── git/
│   │   └── git.go             # Git worktree add/remove, IsGitRepo
│   ├── task/
│   │   ├── types.go           # Task struct and status enum
│   │   └── store.go           # CRUD on tasks.json with file locking
│   ├── tmux/
│   │   └── tmux.go            # tmux wrapper (sessions, send-keys, capture-pane)
│   ├── config/
│   │   └── config.go          # Resolves ~/.orchestra directories
│   └── state/
│       └── store.go           # Generic JSON read/write with syscall.Flock
├── build/                     # Compiled binary
├── Makefile
├── go.mod
└── README.md
```

## Makefile Targets

```
make build       Build the binary to ./build/orchestra
make run ARGS=   Build and run (e.g. make run ARGS="project status")
make install     Copy binary to ~/.local/bin
make uninstall   Remove from ~/.local/bin
make fmt         Format Go files
make vet         Run go vet
make test        Run tests
make check       fmt + vet + test
make clean       Remove build artifacts
make reset       Wipe state files (does not kill tmux sessions)
make kill-all    Kill all orch-* tmux sessions
make nuke        Kill sessions + wipe ~/.orchestra entirely
make workers     Shortcut for worker list
make tasks       Shortcut for task list
```

## Data Directory

All runtime data lives in `~/.orchestra/`:

```
~/.orchestra/
├── state/
│   ├── workers.json           # Worker registry
│   ├── tasks.json             # Task list
│   ├── projects.json          # Project registry
│   └── active_project.json    # Currently active project pointer
└── workspaces/
    └── my-ecommerce/          # Project-scoped workspaces
        ├── add-user-model/    # Worker workspace
        │   ├── backend/       # Git worktree (branch: orch/add-user-model)
        │   └── frontend/      # Git worktree (branch: orch/add-user-model)
        └── fix-api/
            ├── backend/
            └── frontend/
```

Override location with `ORCHESTRA_HOME=/custom/path` env var.

## Example: Full Workflow

```bash
# Set up project
orchestra project create my-api
orchestra repo add ~/code/my-api-backend --name backend
orchestra repo add ~/code/my-api-frontend --name frontend

# Start the planner and plan together
orchestra planner start

# (Inside the planner session, you discuss and it creates tasks + spawns workers)
# Detach with Ctrl+B D

# Check progress from outside
orchestra worker list
orchestra task list

# Attach to a worker to see its progress
orchestra worker attach add-user-model

# Re-attach to the planner
orchestra planner attach

# When done, clean up
orchestra planner stop
make kill-all
```

## Limitations

- No task dependencies — tasks are independent
- No memory — workers don't share context between sessions
- No inter-agent communication — workers can't talk to each other
- No completion detection — you check status manually
- Planner doesn't auto-monitor — you plan interactively, it doesn't poll

These are addressed in v2 (automated orchestrator), v3 (memory), and v4 (communication).

## Dependencies

None. The entire project uses Go standard library:

| Need | Package |
|------|---------|
| CLI flags | `flag` |
| JSON state | `encoding/json` |
| File locking | `syscall` |
| Run tmux/git | `os/exec` |
| Session name validation | `regexp` |
| ID generation | `crypto/rand` |
| Table output | `text/tabwriter` |
| Prompt template | `text/template` |
