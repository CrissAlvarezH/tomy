package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/orchestra/v1/internal/agent"
	"github.com/orchestra/v1/internal/config"
	"github.com/orchestra/v1/internal/task"
)

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}

	agents := agent.NewManager(cfg.StateDir, cfg.WorkspacesDir, cfg.SessionPrefix)
	tasks := task.NewStore(cfg.StateDir)

	switch os.Args[1] {
	case "agent":
		if len(os.Args) < 3 {
			fatal("usage: orchestra agent <spawn|list|kill|attach>")
		}
		switch os.Args[2] {
		case "spawn":
			cmdAgentSpawn(os.Args[3:], agents)
		case "list":
			cmdAgentList(agents)
		case "kill":
			cmdAgentKill(os.Args[3:], agents)
		case "attach":
			cmdAgentAttach(os.Args[3:], agents)
		default:
			fatal("unknown agent subcommand: " + os.Args[2])
		}

	case "task":
		if len(os.Args) < 3 {
			fatal("usage: orchestra task <create|list|status|assign>")
		}
		switch os.Args[2] {
		case "create":
			cmdTaskCreate(os.Args[3:], tasks)
		case "list":
			cmdTaskList(tasks)
		case "status":
			cmdTaskStatus(os.Args[3:], tasks)
		case "assign":
			cmdTaskAssign(os.Args[3:], tasks, agents)
		default:
			fatal("unknown task subcommand: " + os.Args[2])
		}

	case "run":
		cmdRun(os.Args[2:], tasks, agents)

	case "help", "--help", "-h":
		printUsage()

	default:
		fatal("unknown command: " + os.Args[1])
	}
}

func printUsage() {
	fmt.Println(`orchestra - multi-agent Claude Code orchestrator (v1)

Usage:
  orchestra agent spawn <name>                   Spawn a new Claude Code agent
  orchestra agent list                           List all agents
  orchestra agent kill <name>                    Kill an agent
  orchestra agent attach <name>                  Attach to agent's tmux session

  orchestra task create --title "..." --desc "..." Create a task
  orchestra task list                              List all tasks
  orchestra task status <task-id>                   Show task details
  orchestra task assign <task-id> <agent-name>      Assign task to agent

  orchestra run --title "..." --desc "..."       Create task + spawn agent + assign`)
}

// --- Agent commands ---

func cmdAgentSpawn(args []string, mgr *agent.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra agent spawn <name>")
	}
	name := args[0]

	a, err := mgr.Spawn(name)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned agent %q (session: %s, workspace: %s)\n", a.Name, a.Session, a.WorkDir)
}

func cmdAgentList(mgr *agent.Manager) {
	agents, err := mgr.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(agents) == 0 {
		fmt.Println("No agents running.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tTASK\tSESSION")
	for _, a := range agents {
		taskID := a.TaskID
		if taskID == "" {
			taskID = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Name, a.Status, taskID, a.Session)
	}
	w.Flush()
}

func cmdAgentKill(args []string, mgr *agent.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra agent kill <name>")
	}
	if err := mgr.Kill(args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Killed agent %q\n", args[0])
}

func cmdAgentAttach(args []string, mgr *agent.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra agent attach <name>")
	}
	if err := mgr.Attach(args[0]); err != nil {
		fatal(err.Error())
	}
}

// --- Task commands ---

func cmdTaskCreate(args []string, store *task.Store) {
	fs := flag.NewFlagSet("task create", flag.ExitOnError)
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("desc", "", "Task description / prompt for Claude")
	fs.Parse(args)

	if *title == "" {
		fatal("--title is required")
	}

	t, err := store.Create(*title, *desc)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created task %s: %s\n", t.ID, t.Title)
}

func cmdTaskList(store *task.Store) {
	tasks, err := store.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(tasks) == 0 {
		fmt.Println("No tasks.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tASSIGNED TO")
	for _, t := range tasks {
		assignee := t.AssignedTo
		if assignee == "" {
			assignee = "-"
		}
		// Truncate long titles
		title := t.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, title, t.Status, assignee)
	}
	w.Flush()
}

func cmdTaskStatus(args []string, store *task.Store) {
	if len(args) < 1 {
		fatal("usage: orchestra task status <task-id>")
	}

	t, err := store.Get(args[0])
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf("ID:          %s\n", t.ID)
	fmt.Printf("Title:       %s\n", t.Title)
	fmt.Printf("Status:      %s\n", t.Status)
	fmt.Printf("Assigned To: %s\n", valueOr(t.AssignedTo, "-"))
	fmt.Printf("Created:     %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))
	if t.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", t.Description)
	}
	if t.Result != "" {
		fmt.Printf("\nResult:\n%s\n", t.Result)
	}
}

func cmdTaskAssign(args []string, store *task.Store, mgr *agent.Manager) {
	if len(args) < 2 {
		fatal("usage: orchestra task assign <task-id> <agent-name>")
	}
	taskID := args[0]
	agentName := args[1]

	// Verify task exists and is assignable
	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}
	if t.Status != task.StatusPending {
		fatal(fmt.Sprintf("task %s is %s, can only assign pending tasks", taskID, t.Status))
	}

	// Verify agent exists
	a, err := mgr.Get(agentName)
	if err != nil {
		fatal(err.Error())
	}
	if a.Status != agent.StatusIdle {
		fatal(fmt.Sprintf("agent %s is %s, can only assign to idle agents", agentName, a.Status))
	}

	// Build the prompt
	prompt := buildPrompt(t)

	// Send prompt to agent
	if err := mgr.Assign(agentName, prompt); err != nil {
		fatal(err.Error())
	}

	// Update task status
	if err := store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusAssigned
		t.AssignedTo = agentName
	}); err != nil {
		fatal(err.Error())
	}

	// Update agent status
	if err := mgr.Update(agentName, func(a *agent.Agent) {
		a.Status = agent.StatusWorking
		a.TaskID = taskID
	}); err != nil {
		fatal(err.Error())
	}

	fmt.Printf("Assigned task %s to agent %s\n", taskID, agentName)
}

// --- Run command (convenience) ---

func cmdRun(args []string, store *task.Store, mgr *agent.Manager) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("desc", "", "Task description / prompt")
	agentName := fs.String("agent", "", "Agent name (auto-generated if empty)")
	fs.Parse(args)

	if *title == "" {
		fatal("--title is required")
	}

	// Create task
	t, err := store.Create(*title, *desc)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created task %s: %s\n", t.ID, t.Title)

	// Determine agent name
	name := *agentName
	if name == "" {
		name = "agent-" + t.ID
	}

	// Spawn agent
	a, err := mgr.Spawn(name)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned agent %q (session: %s)\n", a.Name, a.Session)

	// Build and send prompt
	prompt := buildPrompt(t)
	if err := mgr.Assign(name, prompt); err != nil {
		fatal(err.Error())
	}

	// Update statuses
	store.Update(t.ID, func(t *task.Task) {
		t.Status = task.StatusAssigned
		t.AssignedTo = name
	})
	mgr.Update(name, func(a *agent.Agent) {
		a.Status = agent.StatusWorking
		a.TaskID = t.ID
	})

	fmt.Printf("Assigned task %s to agent %s\n", t.ID, name)
	fmt.Printf("\nAttach with: orchestra agent attach %s\n", name)
}

// --- Helpers ---

func buildPrompt(t *task.Task) string {
	var b strings.Builder
	b.WriteString(t.Title)
	if t.Description != "" {
		b.WriteString("\n\n")
		b.WriteString(t.Description)
	}
	return b.String()
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
