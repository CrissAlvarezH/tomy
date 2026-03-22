# Orchestra v1 — Multi-Agent Claude Code Orchestrator

A CLI tool that manages projects with multiple repos, spawns Claude Code agents in tmux sessions, and coordinates work through a Lead agent. Inspired by [Gas Town](https://github.com/steveyegge/gastown).

Zero external dependencies — Go stdlib only.

## Prerequisites

- **Go** 1.22+ — `brew install go`
- **tmux** — `brew install tmux`
- **Claude Code** — installed and authenticated (`claude` available in PATH)

## Quick Start

```bash
# Build
make build

# Create a project and add your repos
orchestra project create my-app
orchestra repo add /path/to/backend --name backend
orchestra repo add /path/to/frontend --name frontend

# Start the lead — you'll plan work interactively with it
orchestra lead start
```

Inside the lead session, discuss your goal. The lead will create tasks, spawn workers in the right repos, and assign work — all using `orchestra` commands.

Detach from tmux with `Ctrl+B` then `D`. Re-attach with `orchestra lead attach`.

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
| **Repo** | A directory/git repo within a project (e.g. "backend"). Agents work in repos. |
| **Lead** | An interactive Claude session where you plan work. It spawns workers and assigns tasks. |
| **Agent** | A Claude Code session in tmux. Workers get git worktrees for isolation. |
| **Task** | A unit of work with a title and description. Assigned to one agent. |

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

### Lead (Interactive Orchestrator)

```bash
orchestra lead start                       # Spawn lead + auto-attach
orchestra lead attach                      # Re-attach to lead's session
orchestra lead stop                        # Kill the lead
```

### Agent Management

```bash
orchestra agent spawn <name> [--repo <r>]  # Spawn agent (in a repo's worktree if --repo)
orchestra agent list                       # Show all agents with status
orchestra agent kill <name>                # Kill agent + clean up worktree
orchestra agent attach <name>              # Attach to agent's tmux session
```

### Task Management

```bash
orchestra task create --title "..." --desc "..."   # Create a new task
orchestra task list                                 # List all tasks
orchestra task status <task-id>                     # Show task details
orchestra task assign <task-id> <agent-name>        # Assign task to an idle agent
```

### All-in-One

```bash
orchestra run --title "..." --desc "..." [--repo <r>] [--agent <name>]
```

Creates a task, spawns an agent, and assigns in one command.

## How It Works

### With a Project (recommended)

```
orchestra project create my-ecommerce
orchestra repo add /path/to/backend --name backend
orchestra repo add /path/to/frontend --name frontend
orchestra lead start
```

The lead spawns in the first repo's directory with a system prompt listing all repos and available commands. You discuss the goal, and the lead runs:

```
orchestra task create --title "Add user model" --desc "..."
orchestra agent spawn db-worker --repo backend
orchestra task assign abc123 db-worker
```

Workers spawned with `--repo` on a git repo get **git worktrees** — isolated branches of the same codebase:

```
db-worker  → ~/.orchestra/workspaces/my-ecommerce/db-worker/  (branch: orch/db-worker)
api-worker → ~/.orchestra/workspaces/my-ecommerce/api-worker/ (branch: orch/api-worker)
ui-worker  → ~/.orchestra/workspaces/my-ecommerce/ui-worker/  (branch: orch/ui-worker)
```

### Without a Project (standalone agents)

```
orchestra agent spawn toast
orchestra task create --title "Build a hello world" --desc "..."
orchestra task assign <id> toast
orchestra agent attach toast
```

Agents get isolated workspaces at `~/.orchestra/workspaces/<name>/`.

## Project Structure

```
clone-v1/
├── cmd/
│   └── main.go                # CLI entry point — subcommand router
├── internal/
│   ├── agent/
│   │   ├── types.go           # Agent struct (status, role, project, repo, worktree)
│   │   └── manager.go         # Spawn (with worktrees), kill, list, assign, attach
│   ├── project/
│   │   ├── types.go           # Project and Repo structs
│   │   └── store.go           # Project CRUD, repo add/remove, active project
│   ├── lead/
│   │   ├── lead.go            # Lead start/stop/attach lifecycle
│   │   └── prompt.go          # System prompt template for the lead
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
make agents      Shortcut for agent list
make tasks       Shortcut for task list
```

## Data Directory

All runtime data lives in `~/.orchestra/`:

```
~/.orchestra/
├── state/
│   ├── agents.json            # Agent registry
│   ├── tasks.json             # Task list
│   ├── projects.json          # Project registry
│   └── active_project.json    # Currently active project pointer
└── workspaces/
    ├── standalone-agent/      # Standalone agent (no project)
    └── my-ecommerce/          # Project-scoped worktrees
        ├── db-worker/         # Git worktree (branch: orch/db-worker)
        └── api-worker/        # Git worktree (branch: orch/api-worker)
```

Override location with `ORCHESTRA_HOME=/custom/path` env var.

## Example: Full Workflow

```bash
# Set up project
orchestra project create my-api
orchestra repo add ~/code/my-api-backend --name backend

# Start the lead and plan together
orchestra lead start

# (Inside the lead session, you discuss and it creates tasks + spawns workers)
# Detach with Ctrl+B D

# Check progress from outside
orchestra agent list
orchestra task list

# Attach to a worker to see its progress
orchestra agent attach api-worker

# Re-attach to the lead
orchestra lead attach

# When done, clean up
orchestra lead stop
make kill-all
```

## Limitations

- No task dependencies — tasks are independent
- No memory — agents don't share context between sessions
- No inter-agent communication — agents can't talk to each other
- No completion detection — you check status manually
- Lead doesn't auto-monitor — you plan interactively, it doesn't poll

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
