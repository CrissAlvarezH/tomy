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

Inside the planner session, discuss your goal. The planner will create plans, add tasks, spawn workers, and assign plans — all using `orchestra` commands. Workers get git worktrees for all project repos and create PRs targeting `develop` when done.

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
| **Project** | A named group (e.g. "my-ecommerce"). Contains repos. |
| **Repo** | A directory/git repo within a project. Workers get worktrees of each repo. Can have a setup command for post-worktree initialization. |
| **Plan** | A named group of tasks for a feature. Has a content file (.md) and tracks completion percentage. Assigned to one worker. |
| **Task** | A unit of work belonging to a plan. Has a title, description, and status. Individually tracked for progress. |
| **Planner** | An interactive Claude session where you plan work. It creates plans, spawns workers, and monitors progress. |
| **Worker** | A Claude Code session in tmux with git worktrees for all project repos. Receives a plan and executes its tasks. Creates PRs to `develop` when done. |

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

### Plan Management

```bash
orchestra plan create --name "..."                          # Create a plan
orchestra plan list                                         # List all plans with progress
orchestra plan show <plan-id>                               # Show plan tasks with completion percentage
orchestra plan assign <plan-id> <worker-name>               # Assign plan to a worker
```

### Task Management

```bash
orchestra task create --plan <id> --title "..." --desc "..."  # Create a task under a plan
orchestra task list                                            # List all tasks
orchestra task status <task-id>                                # Show task details
orchestra task done <task-id>                                  # Mark a task as done
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
orchestra worker list                      # Show all workers with plan progress
orchestra worker peek <name>               # See what a worker is doing right now
orchestra worker kill <name>               # Kill worker + clean up worktrees
orchestra worker attach <name>             # Attach to worker's tmux session
orchestra done <worker-name>               # Mark worker and all plan tasks as done
```

### Communication

```bash
orchestra msg send <to> <message> --from <name>    # Send a message (idle: direct, busy: queued)
orchestra msg inbox <name>                          # Read unread messages
```

### All-in-One

```bash
orchestra run --name "..." --title "..." --desc "..." [--worker <name>]
```

Creates a plan with one task, spawns a worker, and assigns in one command.

## How It Works

```
orchestra project create my-ecommerce
orchestra repo add /path/to/backend --name backend
orchestra repo add /path/to/frontend --name frontend
orchestra planner start
```

The planner spawns in its own directory (`~/.orchestra/planner/<project>/`) with a `CLAUDE.md` containing its instructions. It accesses all repos via `--add-dir`. You discuss the goal, and the planner runs:

```
orchestra plan create --name "add-user-auth"
orchestra task create --plan abc123 --title "Add user model" --desc "..."
orchestra task create --plan abc123 --title "Add auth endpoints" --desc "..."
orchestra task create --plan abc123 --title "Add integration tests" --desc "..."
orchestra worker spawn auth-worker
orchestra plan assign abc123 auth-worker
```

The plan groups all tasks and is assigned as a whole to the worker. Track progress:

```
orchestra plan show abc123

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
  ├── backend/   (worktree, branch: orch/auth-worker)
  └── frontend/  (worktree, branch: orch/auth-worker)
```

Workers mark individual tasks done as they complete them with `orchestra task done <task-id>`. When the last task is marked done, the plan and worker are automatically completed and the planner is notified. Workers can also run `orchestra done <name>` to finish everything at once.

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
│   ├── workers.json           # Worker registry (with plan_id per worker)
│   ├── tasks.json             # Task list with plan_id and status tracking
│   ├── plans.json             # Plan registry (name, content_file, status, worker)
│   ├── projects.json          # Project registry
│   ├── active_project.json    # Currently active project pointer
│   ├── inbox/                 # Per-recipient message inboxes
│   ├── plans/                 # Plan content files (.md)
│   └── nudge_queue/           # Deferred notifications for busy sessions
├── planner/
│   └── my-ecommerce/          # Planner home for this project
│       └── CLAUDE.md          # Planner instructions (survives /clear)
└── workspaces/
    └── my-ecommerce/          # Project-scoped workspaces
        ├── auth-worker/       # Worker workspace
        │   ├── backend/       # Git worktree (branch: orch/auth-worker)
        │   └── frontend/      # Git worktree (branch: orch/auth-worker)
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

# (Inside the planner session, you discuss and it creates plans + spawns workers)
# Detach with Ctrl+B D

# Check progress from outside
orchestra worker list
orchestra plan list
orchestra plan show abc123

# See what a worker is doing
orchestra worker peek auth-worker

# Send a message to a worker
orchestra msg send auth-worker "prioritize the API endpoints first" --from user

# Attach to a worker to see its progress
orchestra worker attach auth-worker

# Re-attach to the planner
orchestra planner attach

# When done, clean up
orchestra planner stop
make kill-all
```

## Limitations

- No task dependencies — tasks are independent (no blocking/ordering constraints)
- No worker health monitoring — stuck or crashed workers must be detected manually
- No memory — workers don't share context between sessions
- Planner doesn't auto-monitor — you plan interactively, it doesn't poll

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
