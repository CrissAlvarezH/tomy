# Tomy v1 — Multi-Agent Claude Code Tomytor

A CLI tool that manages projects with multiple repos, spawns Claude Code workers in tmux sessions, and coordinates work through a Planner session. Inspired by [Gas Town](https://github.com/steveyegge/gastown).

Single external dependency: [bbolt](https://github.com/etcd-io/bbolt) (pure Go key/value store).

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
tomy project create my-app
tomy repo add /path/to/backend --name backend --setup 'cp "$TOMY_REPO_PATH/.env" .'
tomy repo add /path/to/frontend --name frontend

# Start the planner — you'll plan work interactively with it
tomy planner start
```

Inside the planner session, discuss your goal. The planner will create plans, add tasks, spawn workers, and assign plans — all using `tomy` commands. Workers get git worktrees for all project repos and create PRs targeting `develop` when done.

Detach from tmux with `Ctrl+B` then `D`. Re-attach with `tomy planner attach`.

## Installation

```bash
make install    # copies binary to ~/.local/bin/tomy
make uninstall  # removes it
```

Make sure `~/.local/bin` is in your PATH.

## Concepts

| Concept | Description |
|---------|-------------|
| **Project** | A named group (e.g. "my-ecommerce"). Contains repos. |
| **Repo** | A directory/git repo within a project. Workers get worktrees of each repo. Can have a setup command for post-worktree initialization. |
| **Plan** | A named group of tasks for a feature. Has a content file (.md) and tracks completion percentage. Assigned to one worker. |
| **Task** | A unit of work belonging to a plan. Has a title, description, and status. Individually tracked for progress. |
| **Planner** | An interactive Claude session where you plan work. It creates plans, spawns workers, and monitors progress. |
| **Worker** | A Claude Code session in tmux with git worktrees for all project repos. Receives a plan and executes its tasks. Creates PRs to `develop` when done. |

## Commands

### Project Management

```bash
tomy project create <name>            # Create a project (auto-set as active)
tomy project list                     # List all projects
tomy project set <name>               # Switch active project
tomy project status                   # Show active project details
```

### Repo Management

```bash
tomy repo add <path> [--name <name>] [--setup <cmd>]  # Add a repo (with optional setup command)
tomy repo list                                         # List repos in active project
tomy repo remove <name>                                # Remove a repo
tomy repo setup <name> --cmd <command>                  # Set/update post-worktree setup command
tomy repo setup <name>                                 # Show current setup command
```

### Plan Management

```bash
tomy plan create --name "..."                          # Create a plan
tomy plan list                                         # List all plans with progress
tomy plan show <plan-id>                               # Show plan tasks with completion percentage
tomy plan assign <plan-id> <worker-name>               # Assign plan to a worker
```

### Task Management

```bash
tomy task create --plan <id> --title "..." --desc "..."  # Create a task under a plan
tomy task list                                            # List all tasks
tomy task status <task-id>                                # Show task details
tomy task done <task-id>                                  # Mark a task as done
```

### Planner (Interactive Tomytor)

```bash
tomy planner start                    # Select project + spawn planner + auto-attach
tomy planner attach                   # Re-attach to planner's session
tomy planner stop                     # Kill the planner
```

### Worker Management

```bash
tomy worker spawn <name>              # Spawn worker (worktrees + run setup commands)
tomy worker list                      # Show all workers with plan progress
tomy worker peek <name>               # See what a worker is doing right now
tomy worker kill <name>               # Kill worker + clean up worktrees
tomy worker attach <name>             # Attach to worker's tmux session
tomy done <worker-name>               # Mark worker and all plan tasks as done
```

### Communication

```bash
tomy msg send <to> <message> --from <name>    # Send a message (idle: direct, busy: queued)
tomy msg inbox <name>                          # Read unread messages
```

### All-in-One

```bash
tomy run --name "..." --title "..." --desc "..." [--worker <name>]
```

Creates a plan with one task, spawns a worker, and assigns in one command.

## How It Works

```
tomy project create my-ecommerce
tomy repo add /path/to/backend --name backend
tomy repo add /path/to/frontend --name frontend
tomy planner start
```

The planner spawns in its own directory (`~/.tomy/planner/<project>/`) with a `CLAUDE.md` containing its instructions. It accesses all repos via `--add-dir`. You discuss the goal, and the planner runs:

```
tomy plan create --name "add-user-auth"
tomy task create --plan abc123 --title "Add user model" --desc "..."
tomy task create --plan abc123 --title "Add auth endpoints" --desc "..."
tomy task create --plan abc123 --title "Add integration tests" --desc "..."
tomy worker spawn auth-worker
tomy plan assign abc123 auth-worker
```

The plan groups all tasks and is assigned as a whole to the worker. Track progress:

```
tomy plan show abc123

Plan:   add-user-auth (abc123)
Status: assigned
Worker: auth-worker

Progress: 1/3 tasks done (33%)

STATUS          ID        TITLE
[done]          d4e5      Add user model
[assigned]      f6a7      Add auth endpoints
[assigned]      b8c9      Add integration tests
```

Workers get **git worktrees** for every repo in the project — isolated branches of each codebase:

```
auth-worker/
  ├── backend/   (worktree, branch: tomy/auth-worker)
  └── frontend/  (worktree, branch: tomy/auth-worker)
```

Workers mark individual tasks done as they complete them with `tomy task done <task-id>`. When the last task is marked done, the plan and worker are automatically completed and the planner is notified. Workers can also run `tomy done <name>` to finish everything at once.

## Worktree Setup Commands

Git worktrees don't include gitignored files like `.env`. If your repos need environment files, port adjustments, or Docker services per worker, configure a **setup command** on the repo. It runs automatically in each new worktree after `worker spawn`.

```bash
# Copy .env and offset ports per worker
tomy repo add ./my-api --name api --setup \
  'cp "$TOMY_REPO_PATH/.env" .env && sed "s/3000/$((3000 + TOMY_WORKER_INDEX * 10))/" .env > .env.tmp && mv .env.tmp .env'

# Spin up an isolated Docker stack per worker
tomy repo setup api --cmd \
  'cp "$TOMY_REPO_PATH/.env" . && docker compose -p "api-$TOMY_WORKER_NAME" up -d'
```

Setup commands run via `sh -c` with a 60-second timeout. Failures log a warning but don't block worker creation.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TOMY_WORKTREE_PATH` | Absolute path to the created worktree |
| `TOMY_REPO_PATH` | Absolute path to the original repo (for copying gitignored files) |
| `TOMY_REPO_NAME` | Name of the repo |
| `TOMY_WORKER_NAME` | Name of the worker |
| `TOMY_WORKER_INDEX` | 0-based index of this worker within the project (for port offsetting) |
| `TOMY_WORKSPACE_DIR` | Worker's workspace root directory |

## Project Structure

```
clone-v1/
├── cmd/
│   └── main.go                # CLI entry point — subcommand router
├── internal/
│   ├── worker/
│   │   ├── types.go           # Worker struct (status, plan_id, worktrees)
│   │   ├── manager.go         # Spawn (with multi-repo worktrees), kill, list, assign
│   │   └── setup.go           # Post-worktree setup command runner
│   ├── plan/
│   │   ├── types.go           # Plan struct (name, content_file, status, worker)
│   │   └── store.go           # Plan CRUD with file locking
│   ├── task/
│   │   ├── types.go           # Task struct (plan_id, status)
│   │   └── store.go           # CRUD + ListByPlan on tasks.json
│   ├── project/
│   │   ├── types.go           # Project and Repo structs
│   │   └── store.go           # Project CRUD, repo add/remove, active project
│   ├── planner/
│   │   ├── planner.go         # Planner start/stop/attach lifecycle
│   │   └── prompt.go          # System prompt template for the planner
│   ├── git/
│   │   └── git.go             # Git worktree add/remove, IsGitRepo
│   ├── msg/
│   │   ├── types.go           # Message struct
│   │   └── store.go           # Per-recipient inbox (send, unread, mark read)
│   ├── nudge/
│   │   └── queue.go           # Filesystem queue for deferred notifications
│   ├── tmux/
│   │   └── tmux.go            # tmux wrapper (sessions, send-keys, idle detection)
│   ├── config/
│   │   └── config.go          # Resolves ~/.tomy directories
│   └── state/
│       └── db.go              # bbolt database wrapper with typed generic helpers
├── build/                     # Compiled binary
├── Makefile
├── go.mod
└── README.md
```

## Makefile Targets

```
make build       Build the binary to ./build/tomy
make run ARGS=   Build and run (e.g. make run ARGS="project status")
make install     Copy binary to ~/.local/bin
make uninstall   Remove from ~/.local/bin
make fmt         Format Go files
make vet         Run go vet
make test        Run tests
make check       fmt + vet + test
make clean       Remove build artifacts
make reset       Wipe state files (does not kill tmux sessions)
make kill-all    Kill all tomy-* tmux sessions
make nuke        Kill sessions + wipe ~/.tomy entirely
make workers     Shortcut for worker list
make tasks       Shortcut for task list
```

## Data Directory

All runtime data lives in `~/.tomy/`:

```
~/.tomy/
├── state/
│   └── tomy.db                # bbolt database (projects, tasks, plans, workers, inbox, nudges, meta)
├── planner/
│   └── my-ecommerce/          # Planner home for this project
│       └── CLAUDE.md          # Planner instructions (survives /clear)
└── workspaces/
    └── my-ecommerce/          # Project-scoped workspaces
        ├── auth-worker/       # Worker workspace
        │   ├── backend/       # Git worktree (branch: tomy/auth-worker)
        │   └── frontend/      # Git worktree (branch: tomy/auth-worker)
        └── fix-api/
            ├── backend/
            └── frontend/
```

Override location with `TOMY_HOME=/custom/path` env var.

## Example: Full Workflow

```bash
# Set up project
tomy project create my-api
tomy repo add ~/code/my-api-backend --name backend
tomy repo add ~/code/my-api-frontend --name frontend

# Start the planner and plan together
tomy planner start

# (Inside the planner session, you discuss and it creates plans + spawns workers)
# Detach with Ctrl+B D

# Check progress from outside
tomy worker list
tomy plan list
tomy plan show abc123

# See what a worker is doing
tomy worker peek auth-worker

# Send a message to a worker
tomy msg send auth-worker "prioritize the API endpoints first" --from user

# Attach to a worker to see its progress
tomy worker attach auth-worker

# Re-attach to the planner
tomy planner attach

# When done, clean up
tomy planner stop
make kill-all
```

## Limitations

- No task dependencies — tasks are independent (no blocking/ordering constraints)
- No worker health monitoring — stuck or crashed workers must be detected manually
- No memory — workers don't share context between sessions
- Planner doesn't auto-monitor — you plan interactively, it doesn't poll

## Dependencies

| Need | Package |
|------|---------|
| Embedded key/value store | `go.etcd.io/bbolt` |
| CLI flags | `flag` |
| Run tmux/git | `os/exec` |
| Session name validation | `regexp` |
| ID generation | `crypto/rand` |
| Table output | `text/tabwriter` |
| Prompt template | `text/template` |
