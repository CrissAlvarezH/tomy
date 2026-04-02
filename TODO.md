# TODO — Features Inspired by Gastown

## 1. Convoy (Batch Work Tracking)

Group related tasks into a single unit with lifecycle tracking. Currently tasks are independent — there's no way to say "these 5 tasks are all part of the auth refactor" and track them as a batch. A convoy would have states (created → launched → staged → landed → closed), aggregate progress across its tasks, and let the planner reason about completion at a higher level than individual tasks.

**Commands:** `convoy create`, `convoy add <task>`, `convoy status`, `convoy close`, `convoy list`

## 2. Witness (Worker Health Monitoring)

Background monitoring that detects stuck, crashed, or zombie workers. Right now the only way to know if a worker is alive is to manually `peek` at it. A witness would periodically check each worker's tmux session for signs of life (prompt visible but no activity for too long, session gone, error patterns in output), and automatically notify the planner or attempt recovery. This is the difference between "hope it works" and "know when it doesn't."

**Commands:** `witness start`, `witness stop`, `witness status`
**Signals:** notify planner on stuck detection, optionally auto-kill and respawn

## 3. Broadcast (Message All Workers)

Send a single message to every active worker at once. Currently messaging is one-to-one — if the planner needs to tell all workers "stop what you're doing, we're changing direction," it has to send N individual messages. Broadcast uses the existing `msg send` machinery but iterates over all workers (and optionally the planner).

**Commands:** `msg broadcast <message> --from <name>`

## 4. Status Dashboard (Aggregate Health View)

A single command that shows the full picture: active project, all workers with their status and current task, all tasks with progress, message queue depths, and session health. Currently you need to run `worker list`, `task list`, and `project status` separately and mentally compose them. This command composes everything into one view.

**Commands:** `orchestra status` (enhanced beyond current `project status`)

## 5. Checkpoint & Recovery (Session Context Persistence)

Save and restore worker context so a crashed session can resume where it left off. When a worker is actively working, periodically snapshot its state (current task, recent output, plan file). If the tmux session dies, a new session can be spawned and primed with the saved context instead of starting blind. Pairs well with Witness — detect crash, then auto-recover.

**Commands:** `worker checkpoint <name>`, `worker recover <name>`
**State:** `~/.orchestra/state/checkpoints/<worker>.json`

## 6. Refinery (Merge Queue)

Automate the PR landing flow after workers complete tasks. Instead of the planner manually reviewing and merging each PR, the refinery queues completed work, runs checks (CI status, conflict detection), and lands PRs in order. Handles the common case where two workers touch overlapping files — detect conflicts early and rebase automatically or escalate.

**Commands:** `refinery start`, `refinery stop`, `refinery status`, `refinery queue`, `refinery next`
**Integration:** hooks into `done` command to auto-submit to the queue

## 7. Formula (Workflow Templates)

Reusable task definitions so common workflows don't need to be described from scratch every time. A formula is a template (TOML or YAML) that defines a set of tasks with descriptions, dependencies, and variable placeholders. Instead of the planner writing "create a REST endpoint for users with CRUD operations and tests" every time, it instantiates a formula like `api-crud` with `resource=users`. Saves planner tokens and ensures consistency.

**Commands:** `formula create`, `formula list`, `formula show <name>`, `formula run <name> --var key=value`
**State:** `~/.orchestra/formulas/` directory with template files

## 8. Escalation (Structured Error Routing)

A severity-routed system for workers to report problems instead of sending unstructured messages. Workers can escalate issues with a severity level (info, warning, critical), and the system routes them appropriately — info goes to the planner's inbox, warnings get a nudge, critical interrupts immediately. Includes acknowledgment tracking so escalations don't get lost.

**Commands:** `escalate <severity> <message> --from <worker>`, `escalate list`, `escalate ack <id>`
**State:** `~/.orchestra/state/escalations.json`
