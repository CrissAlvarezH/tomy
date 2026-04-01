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
orchestra repo add /path/to/backend --name backend --setup 'cp "$ORCH_REPO_PATH/.env" .'
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
| **Repo** | A directory/git repo within a project. Workers get worktrees of each repo. Can have a setup command for post-worktree initialization. |
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
orchestra repo add <path> [--name <name>] [--setup <cmd>]  # Add a repo (with optional setup command)
orchestra repo list                                         # List repos in active project
orchestra repo remove <name>                                # Remove a repo
orchestra repo setup <name> --cmd <command>                  # Set/update post-worktree setup command
orchestra repo setup <name>                                 # Show current setup command
```

### Planner (Interactive Orchestrator)

```bash
orchestra planner start                    # Select project + spawn planner + auto-attach
orchestra planner attach                   # Re-attach to planner's session
orchestra planner stop                     # Kill the planner
```

### Worker Management

```bash
orchestra worker spawn <name>              # Spawn worker (worktrees + run setup commands)
orchestra worker list                      # Show all workers with status
orchestra worker peek <name>               # See what a worker is doing right now
orchestra worker kill <name>               # Kill worker + clean up worktrees
orchestra worker attach <name>             # Attach to worker's tmux session
orchestra done <worker-name>               # Mark worker and its task as done
```

### Task Management

```bash
orchestra task create --title "..." --desc "..."   # Create a new task
orchestra task list                                 # List all tasks
orchestra task status <task-id>                     # Show task details
orchestra task assign <task-id> <worker-name>       # Assign task to an idle worker
```

### Communication

```bash
orchestra nudge <name> <message>           # Send a message into a session (planner or worker)
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

The planner spawns in its own directory (`~/.orchestra/planner/<project>/`) with a `CLAUDE.md` containing its instructions. It accesses all repos via `--add-dir`. You discuss the goal, and the planner runs:

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

When a worker finishes, it commits, pushes, creates a PR targeting `develop` via `gh pr create`, and marks itself done with `orchestra done <name>`.

## Worktree Setup Commands

Git worktrees don't include gitignored files like `.env`. If your repos need environment files, port adjustments, or Docker services per worker, configure a **setup command** on the repo. It runs automatically in each new worktree after `worker spawn`.

```bash
# Copy .env and offset ports per worker
orchestra repo add ./my-api --name api --setup \
  'cp "$ORCH_REPO_PATH/.env" .env && sed "s/3000/$((3000 + ORCH_WORKER_INDEX * 10))/" .env > .env.tmp && mv .env.tmp .env'

# Spin up an isolated Docker stack per worker
orchestra repo setup api --cmd \
  'cp "$ORCH_REPO_PATH/.env" . && docker compose -p "api-$ORCH_WORKER_NAME" up -d'
```

Setup commands run via `sh -c` with a 60-second timeout. Failures log a warning but don't block worker creation.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ORCH_WORKTREE_PATH` | Absolute path to the created worktree |
| `ORCH_REPO_PATH` | Absolute path to the original repo (for copying gitignored files) |
| `ORCH_REPO_NAME` | Name of the repo |
| `ORCH_WORKER_NAME` | Name of the worker |
| `ORCH_WORKER_INDEX` | 0-based index of this worker within the project (for port offsetting) |
| `ORCH_WORKSPACE_DIR` | Worker's workspace root directory |

## Project Structure

```
clone-v1/
├── cmd/
│   └── main.go                # CLI entry point — subcommand router
├── internal/
│   ├── worker/
│   │   ├── types.go           # Worker struct (status, project, worktrees)
│   │   ├── manager.go         # Spawn (with multi-repo worktrees), kill, list, assign
│   │   └── setup.go           # Post-worktree setup command runner
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
├── planner/
│   └── my-ecommerce/          # Planner home for this project
│       └── CLAUDE.md          # Planner instructions (survives /clear)
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

# See what a worker is doing
orchestra worker peek add-user-model

# Send a message to a worker
orchestra nudge add-user-model "prioritize the API endpoints first"

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
