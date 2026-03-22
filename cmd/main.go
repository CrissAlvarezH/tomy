package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/orchestra/v1/internal/agent"
	"github.com/orchestra/v1/internal/config"
	"github.com/orchestra/v1/internal/lead"
	"github.com/orchestra/v1/internal/project"
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
	projects := project.NewStore(cfg.StateDir)

	// Load active project (nil if none set)
	activeProj, _ := projects.GetActive()

	switch os.Args[1] {
	case "agent":
		if len(os.Args) < 3 {
			fatal("usage: orchestra agent <spawn|list|kill|attach>")
		}
		switch os.Args[2] {
		case "spawn":
			cmdAgentSpawn(os.Args[3:], agents, activeProj, projects)
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

	case "project":
		if len(os.Args) < 3 {
			fatal("usage: orchestra project <create|list|set|status>")
		}
		switch os.Args[2] {
		case "create":
			cmdProjectCreate(os.Args[3:], projects)
		case "list":
			cmdProjectList(projects)
		case "set":
			cmdProjectSet(os.Args[3:], projects)
		case "status":
			cmdProjectStatus(projects, agents)
		default:
			fatal("unknown project subcommand: " + os.Args[2])
		}

	case "repo":
		if len(os.Args) < 3 {
			fatal("usage: orchestra repo <add|list|remove>")
		}
		switch os.Args[2] {
		case "add":
			cmdRepoAdd(os.Args[3:], projects, activeProj)
		case "list":
			cmdRepoList(activeProj)
		case "remove":
			cmdRepoRemove(os.Args[3:], projects, activeProj)
		default:
			fatal("unknown repo subcommand: " + os.Args[2])
		}

	case "lead":
		if len(os.Args) < 3 {
			fatal("usage: orchestra lead <start|stop|attach>")
		}
		switch os.Args[2] {
		case "start":
			cmdLeadStart(agents, activeProj)
		case "stop":
			cmdLeadStop(agents)
		case "attach":
			cmdLeadAttach(agents)
		default:
			fatal("unknown lead subcommand: " + os.Args[2])
		}

	case "run":
		cmdRun(os.Args[2:], tasks, agents, activeProj, projects)

	case "help", "--help", "-h":
		printUsage()

	default:
		fatal("unknown command: " + os.Args[1])
	}
}

func printUsage() {
	fmt.Println(`orchestra - multi-agent Claude Code orchestrator (v1)

Usage:
  orchestra project create <name>                    Create a new project
  orchestra project list                             List all projects
  orchestra project set <name>                       Set active project
  orchestra project status                           Show active project details

  orchestra repo add <path> [--name <name>]          Add a repo to active project
  orchestra repo list                                List repos in active project
  orchestra repo remove <name>                       Remove a repo

  orchestra lead start                               Spawn lead + attach (interactive planning)
  orchestra lead stop                                Kill the lead agent
  orchestra lead attach                              Attach to lead's session

  orchestra agent spawn <name> [--repo <repo>]       Spawn a worker agent
  orchestra agent list                               List all agents
  orchestra agent kill <name>                        Kill an agent
  orchestra agent attach <name>                      Attach to agent's session

  orchestra task create --title "..." --desc "..."   Create a task
  orchestra task list                                List all tasks
  orchestra task status <task-id>                    Show task details
  orchestra task assign <task-id> <agent-name>       Assign task to agent

  orchestra run --title "..." --desc "..." [--repo <repo>]  Create + spawn + assign`)
}

// --- Project commands ---

func cmdProjectCreate(args []string, store *project.Store) {
	if len(args) < 1 {
		fatal("usage: orchestra project create <name>")
	}
	p, err := store.Create(args[0])
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created project %q (id: %s, set as active)\n", p.Name, p.ID)
}

func cmdProjectList(store *project.Store) {
	projects, err := store.List()
	if err != nil {
		fatal(err.Error())
	}
	active, _ := store.GetActive()

	if len(projects) == 0 {
		fmt.Println("No projects.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tID\tREPOS\tACTIVE")
	for _, p := range projects {
		marker := ""
		if active != nil && active.ID == p.ID {
			marker = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", p.Name, p.ID, len(p.Repos), marker)
	}
	w.Flush()
}

func cmdProjectSet(args []string, store *project.Store) {
	if len(args) < 1 {
		fatal("usage: orchestra project set <name-or-id>")
	}
	p, err := store.Get(args[0])
	if err != nil {
		fatal(err.Error())
	}
	if err := store.SetActive(p.ID); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Active project: %s\n", p.Name)
}

func cmdProjectStatus(store *project.Store, mgr *agent.Manager) {
	proj, _ := store.GetActive()
	if proj == nil {
		fmt.Println("No active project. Create one with: orchestra project create <name>")
		return
	}

	fmt.Printf("Project: %s (id: %s)\n", proj.Name, proj.ID)
	fmt.Printf("Repos:   %d\n", len(proj.Repos))
	for _, r := range proj.Repos {
		gitLabel := ""
		if r.IsGitRepo {
			gitLabel = " (git)"
		}
		fmt.Printf("  - %s: %s%s\n", r.Name, r.Path, gitLabel)
	}

	agents, _ := mgr.List()
	projectAgents := 0
	for _, a := range agents {
		if a.ProjectID == proj.ID {
			projectAgents++
		}
	}
	fmt.Printf("Agents:  %d\n", projectAgents)
}

// --- Repo commands ---

func requireActiveProject(proj *project.Project) {
	if proj == nil {
		fatal("no active project. Create one with: orchestra project create <name>")
	}
}

func cmdRepoAdd(args []string, store *project.Store, proj *project.Project) {
	requireActiveProject(proj)

	fs := flag.NewFlagSet("repo add", flag.ExitOnError)
	name := fs.String("name", "", "Repo name (defaults to directory basename)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fatal("usage: orchestra repo add <path> [--name <name>]")
	}
	path := fs.Arg(0)

	repoName := *name
	if repoName == "" {
		repoName = filepath.Base(path)
	}

	r, err := store.AddRepo(proj.ID, repoName, path)
	if err != nil {
		fatal(err.Error())
	}

	gitLabel := ""
	if r.IsGitRepo {
		gitLabel = " (git detected)"
	}
	fmt.Printf("Added repo %q: %s%s\n", r.Name, r.Path, gitLabel)
}

func cmdRepoList(proj *project.Project) {
	requireActiveProject(proj)

	if len(proj.Repos) == 0 {
		fmt.Println("No repos in project. Add one with: orchestra repo add <path>")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tGIT")
	for _, r := range proj.Repos {
		gitLabel := "no"
		if r.IsGitRepo {
			gitLabel = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, r.Path, gitLabel)
	}
	w.Flush()
}

func cmdRepoRemove(args []string, store *project.Store, proj *project.Project) {
	requireActiveProject(proj)

	if len(args) < 1 {
		fatal("usage: orchestra repo remove <name>")
	}
	if err := store.RemoveRepo(proj.ID, args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Removed repo %q\n", args[0])
}

// --- Lead commands ---

func cmdLeadStart(mgr *agent.Manager, proj *project.Project) {
	requireActiveProject(proj)
	if err := lead.Start(mgr, proj); err != nil {
		fatal(err.Error())
	}
}

func cmdLeadStop(mgr *agent.Manager) {
	if err := lead.Stop(mgr); err != nil {
		fatal(err.Error())
	}
	fmt.Println("Lead stopped.")
}

func cmdLeadAttach(mgr *agent.Manager) {
	if err := lead.Attach(mgr); err != nil {
		fatal(err.Error())
	}
}

// --- Agent commands ---

func cmdAgentSpawn(args []string, mgr *agent.Manager, proj *project.Project, projStore *project.Store) {
	fs := flag.NewFlagSet("agent spawn", flag.ExitOnError)
	repoName := fs.String("repo", "", "Repo to work in (from active project)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fatal("usage: orchestra agent spawn <name> [--repo <repo-name>]")
	}
	name := fs.Arg(0)

	opts := agent.SpawnOptions{Name: name}

	if *repoName != "" {
		requireActiveProject(proj)
		repo, err := projStore.GetRepo(proj, *repoName)
		if err != nil {
			fatal(err.Error())
		}
		opts.Project = proj
		opts.Repo = repo
	}

	a, err := mgr.Spawn(opts)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned agent %q (session: %s, workdir: %s)\n", a.Name, a.Session, a.WorkDir)
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
	fmt.Fprintln(w, "NAME\tROLE\tSTATUS\tREPO\tTASK\tSESSION")
	for _, a := range agents {
		taskID := valueOr(a.TaskID, "-")
		repo := valueOr(a.RepoName, "-")
		role := string(a.Role)
		if role == "" {
			role = "worker"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", a.Name, role, a.Status, repo, taskID, a.Session)
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
		assignee := valueOr(t.AssignedTo, "-")
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

	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}
	if t.Status != task.StatusPending {
		fatal(fmt.Sprintf("task %s is %s, can only assign pending tasks", taskID, t.Status))
	}

	a, err := mgr.Get(agentName)
	if err != nil {
		fatal(err.Error())
	}
	if a.Status != agent.StatusIdle {
		fatal(fmt.Sprintf("agent %s is %s, can only assign to idle agents", agentName, a.Status))
	}

	prompt := buildPrompt(t)
	if err := mgr.Assign(agentName, prompt); err != nil {
		fatal(err.Error())
	}

	store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusAssigned
		t.AssignedTo = agentName
	})
	mgr.Update(agentName, func(a *agent.Agent) {
		a.Status = agent.StatusWorking
		a.TaskID = taskID
	})

	fmt.Printf("Assigned task %s to agent %s\n", taskID, agentName)
}

// --- Run command (convenience) ---

func cmdRun(args []string, store *task.Store, mgr *agent.Manager, proj *project.Project, projStore *project.Store) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("desc", "", "Task description / prompt")
	agentName := fs.String("agent", "", "Agent name (auto-generated if empty)")
	repoName := fs.String("repo", "", "Repo to work in (from active project)")
	fs.Parse(args)

	if *title == "" {
		fatal("--title is required")
	}

	t, err := store.Create(*title, *desc)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created task %s: %s\n", t.ID, t.Title)

	name := *agentName
	if name == "" {
		name = "agent-" + t.ID
	}

	opts := agent.SpawnOptions{Name: name}
	if *repoName != "" {
		requireActiveProject(proj)
		repo, err := projStore.GetRepo(proj, *repoName)
		if err != nil {
			fatal(err.Error())
		}
		opts.Project = proj
		opts.Repo = repo
	}

	a, err := mgr.Spawn(opts)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned agent %q (session: %s)\n", a.Name, a.Session)

	prompt := buildPrompt(t)
	if err := mgr.Assign(name, prompt); err != nil {
		fatal(err.Error())
	}

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

