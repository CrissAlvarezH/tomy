# Orchestra v1 — Core Agent Management

A CLI tool that spawns and manages multiple Claude Code sessions as autonomous agents using tmux. Inspired by [Gas Town](https://github.com/steveyegge/gastown).

Zero external dependencies — Go stdlib only.

## Prerequisites

- **Go** 1.22+ — `brew install go`
- **tmux** — `brew install tmux`
- **Claude Code** — installed and authenticated (`claude` available in PATH)

## Quick Start

```bash
# Build
make build

# Spawn an agent
./build/orchestra agent spawn toast

# Create a task
./build/orchestra task create --title "Build a REST API" --desc "Create a Go HTTP server with GET /health endpoint"

# Assign the task (use the ID printed by the previous command)
./build/orchestra task assign <task-id> toast

# Watch the agent work
./build/orchestra agent attach toast
```

Detach from tmux with `Ctrl+B` then `D`.

## Installation

```bash
make install    # copies binary to ~/.local/bin/orchestra
```

Make sure `~/.local/bin` is in your PATH. Then use `orchestra` from anywhere.

```bash
make uninstall  # removes it
```

## Commands

### Agent Management

```bash
orchestra agent spawn <name>     # Create agent with tmux session + Claude Code
orchestra agent list             # Show all agents with status
orchestra agent kill <name>      # Kill agent's tmux session and deregister
orchestra agent attach <name>    # Attach terminal to agent's tmux session
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
orchestra run --title "..." --desc "..."              # Create task + spawn agent + assign
orchestra run --title "..." --desc "..." --agent bob   # Same but name the agent
```

## How It Works

```
orchestra agent spawn toast
        │
        ├─ 1. Creates workspace directory:  ./workspaces/toast/
        ├─ 2. Starts tmux session:          orch-toast
        ├─ 3. Runs inside tmux:             cd ./workspaces/toast/
        ├─ 4. Runs inside tmux:             claude --dangerously-skip-permissions
        └─ 5. Registers agent in:           ./state/agents.json

orchestra task assign <id> toast
        │
        ├─ 1. Reads task from:              ./state/tasks.json
        ├─ 2. Writes prompt to:             ./workspaces/toast/.task-prompt
        ├─ 3. Sends to tmux session:        cat .task-prompt
        ├─ 4. Updates task status:          pending → assigned
        └─ 5. Updates agent status:         idle → working
```

The agent (Claude Code) receives the prompt and works autonomously from there.

## Project Structure

```
clone-v1/
├── cmd/
│   └── main.go                # CLI entry point — subcommand router
├── internal/
│   ├── agent/
│   │   ├── types.go           # Agent struct and status enum
│   │   └── manager.go         # Spawn, kill, list, assign, attach
│   ├── task/
│   │   ├── types.go           # Task struct and status enum
│   │   └── store.go           # CRUD on tasks.json with file locking
│   ├── tmux/
│   │   └── tmux.go            # tmux wrapper (new-session, kill, send-keys, list)
│   ├── config/
│   │   └── config.go          # Resolves state/ and workspaces/ directories
│   └── state/
│       └── store.go           # Generic JSON read/write with syscall.Flock
├── state/                     # Runtime state (created automatically)
│   ├── agents.json            # Agent registry
│   └── tasks.json             # Task list
├── workspaces/                # Per-agent working directories (created automatically)
│   └── <agent-name>/
├── build/                     # Compiled binary (created by make build)
├── Makefile
├── go.mod
└── README.md
```

## Makefile Targets

```
make build       Build the binary to ./build/orchestra
make run ARGS=   Build and run (e.g. make run ARGS="agent list")
make install     Copy binary to ~/.local/bin
make uninstall   Remove from ~/.local/bin
make fmt         Format Go files
make vet         Run go vet
make test        Run tests
make check       fmt + vet + test
make clean       Remove build artifacts
make reset       Wipe state files (does not kill tmux sessions)
make kill-all    Kill all orch-* tmux sessions
make nuke        Kill sessions + wipe state + remove workspaces
make agents      Shortcut for agent list
make tasks       Shortcut for task list
```

## State Files

All state is stored as JSON files in `./state/`. Concurrent access is safe — reads use shared locks (`LOCK_SH`) and writes use exclusive locks (`LOCK_EX`) via `syscall.Flock`.

**agents.json** — array of agents:
```json
[
  {
    "id": "toast",
    "name": "toast",
    "status": "working",
    "task_id": "a1b2c3d4",
    "session": "orch-toast",
    "work_dir": "/path/to/workspaces/toast",
    "created_at": "2026-03-22T10:00:00Z"
  }
]
```

**tasks.json** — array of tasks:
```json
[
  {
    "id": "a1b2c3d4",
    "title": "Build a REST API",
    "description": "Create a Go HTTP server with GET /health endpoint",
    "status": "assigned",
    "assigned_to": "toast",
    "created_at": "2026-03-22T10:00:00Z",
    "updated_at": "2026-03-22T10:01:00Z"
  }
]
```

## Example: Multiple Agents Working in Parallel

```bash
# Create tasks
orchestra task create --title "Set up database models" --desc "Create SQLite schema for users and posts"
orchestra task create --title "Build HTTP handlers" --desc "Create CRUD endpoints for /users and /posts"
orchestra task create --title "Write tests" --desc "Add unit tests for the handlers"

# Spawn agents and assign
orchestra run --title "Set up database models" --desc "..." --agent db-worker
orchestra run --title "Build HTTP handlers" --desc "..." --agent api-worker

# Check progress
orchestra agent list
orchestra task list

# Watch a specific agent
orchestra agent attach api-worker

# When done, clean up
make kill-all
```

## Limitations (v1)

- No task dependencies — tasks are independent
- No orchestrator — you manually create tasks and assign them
- No memory — agents don't share context between sessions
- No inter-agent communication — agents can't talk to each other
- No completion detection — you check status manually

These are addressed in v2 (orchestrator), v3 (memory), and v4 (communication).

## Dependencies

None. The entire project uses Go standard library:

| Need | Package |
|------|---------|
| CLI flags | `flag` |
| JSON state | `encoding/json` |
| File locking | `syscall` |
| Run tmux | `os/exec` |
| Session name validation | `regexp` |
| ID generation | `crypto/rand` |
| Table output | `text/tabwriter` |
