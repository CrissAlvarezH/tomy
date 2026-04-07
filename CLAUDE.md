# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development

```bash
make build          # Compile to ./build/tomy
make test           # go test ./... -v
make check          # fmt + vet + test (run before committing)
make install        # Copy binary to ~/.local/bin/tomy
make fmt            # gofmt
make vet            # go vet
```

Useful during development:
```bash
make reset          # Wipe state database (keeps sessions alive)
make nuke           # Kill all tmux sessions + wipe ~/.tomy
make workers        # Quick alias for tomy worker list
make tasks          # Quick alias for tomy task list
make kill-all       # Kill all tomy tmux sessions
```

Run a single test: `go test ./internal/tmux/ -run TestIsIdle -v`

## Architecture

Tmux-based multi-agent tomytor for Claude Code. A **planner** session coordinates **worker** sessions, each running Claude Code in isolated git worktrees. Single external dependency: bbolt (pure Go key/value store).

### Key flow
1. User creates a **project** and adds **repos** to it
2. `tomy planner start` spawns a Claude Code session with a system prompt (CLAUDE.md) listing available commands
3. Planner creates **tasks**, spawns **workers** (each gets worktrees for all project repos on branch `tomy/<worker-name>`), and assigns tasks
4. Workers execute plans, communicate back via **messages**
5. `tomy done <worker>` marks completion

### Packages (`internal/`)

- **config** — Resolves `~/.tomy` directory layout (`TOMY_HOME` overrides)
- **state** — bbolt database wrapper with typed helpers (`Get`, `Put`, `Delete`, `List`, `ListByPrefix`, `DrainByPrefix`). All stores use this.
- **project** — Project/repo CRUD, active project tracking
- **worker** — Worker lifecycle: spawn (creates worktrees + tmux session + CLAUDE.md + hooks), kill (cleanup worktrees), list, attach, assign
- **task** — Task CRUD with status flow: pending → assigned → in-progress → done/failed
- **planner** — Planner session management, system prompt rendering via `text/template`
- **tmux** — Wrapper around tmux CLI. `IsIdle()` polls for prompt visibility + busy indicator for safe message delivery
- **msg** — Per-recipient inbox stored in bbolt `inbox` bucket with composite keys (`recipient/msg_id`)
- **nudge** — Deferred notifications in bbolt `nudges` bucket with composite keys (`recipient/timestamp`). Drained atomically via `--inject` flag and formatted as `<system-reminder>` blocks for Claude Code hook injection
- **git** — Git worktree add/remove, repo detection

### Messaging: idle-aware delivery

`msg send` detects if the recipient's tmux session is idle (prompt visible, no "esc to interrupt"). If idle, delivers via `tmux send-keys`. If busy, enqueues a nudge to bbolt. A `UserPromptSubmit` hook (written to `.claude/settings.json` at spawn time) drains the queue at each turn boundary and injects messages as `<system-reminder>` context.

### State layout (`~/.tomy/`)

All state lives in a single bbolt database with ACID transactions:
- `state/tomy.db` — bbolt database containing all buckets:
  - `projects`, `tasks`, `plans`, `plan_content`, `workers` — entity stores
  - `inbox` — messages with composite keys (`recipient/msg_id`)
  - `nudges` — deferred notifications with composite keys (`recipient/timestamp`)
  - `meta` — singleton values (e.g. `active_project`)
- `workspaces/<project>/<worker>/` — worker home dirs with worktrees

### CLI routing

`cmd/main.go` is the single entry point. All subcommands (`project`, `repo`, `worker`, `task`, `msg`, `planner`, `done`, `run`) are routed in `main()` and dispatched to `cmd*` handler functions in the same file.

## Versioning

The version is defined in `cmd/main.go` as the `version` constant. **Every commit must bump it** using semver:
- **Patch** (1.2.x → 1.2.x+1): bug fixes, small tweaks
- **Minor** (1.x.0 → 1.x+1.0): new features, new commands, new packages
- **Major** (x.0.0 → x+1.0.0): breaking changes to CLI interface or state format

Before committing, determine the appropriate bump based on the changes and update the `version` const.

## Conventions

- Go's `flag` package stops parsing at the first non-flag arg. Commands that accept flags mixed with positional args must reorder args before `fs.Parse()` (see `cmdMsgSend` for the pattern).
- Session names: `tomy-<name>` (validated as alphanumeric + underscore + dash).
- Worker branches: `tomy/<worker-name>`.
- Planner and worker sessions get auto-generated CLAUDE.md and `.claude/settings.json` at spawn time.
