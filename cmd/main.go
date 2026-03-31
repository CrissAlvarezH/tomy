package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/orchestra/v1/internal/config"
	"github.com/orchestra/v1/internal/planner"
	"github.com/orchestra/v1/internal/project"
	"github.com/orchestra/v1/internal/task"
	"github.com/orchestra/v1/internal/tmux"
	"github.com/orchestra/v1/internal/worker"
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

	workers := worker.NewManager(cfg.StateDir, cfg.WorkspacesDir, cfg.SessionPrefix)
	tasks := task.NewStore(cfg.StateDir)
	projects := project.NewStore(cfg.StateDir)

	// Load active project (nil if none set)
	activeProj, _ := projects.GetActive()

	switch os.Args[1] {
	case "worker":
		if len(os.Args) < 3 {
			fatal("usage: orchestra worker <spawn|list|kill|attach>")
		}
		switch os.Args[2] {
		case "spawn":
			cmdWorkerSpawn(os.Args[3:], workers, activeProj)
		case "list":
			cmdWorkerList(workers)
		case "kill":
			cmdWorkerKill(os.Args[3:], workers)
		case "attach":
			cmdWorkerAttach(os.Args[3:], workers)
		case "peek":
			cmdWorkerPeek(os.Args[3:], workers)
		default:
			fatal("unknown worker subcommand: " + os.Args[2])
		}

	case "nudge":
		cmdNudge(os.Args[2:], workers)

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
			cmdTaskAssign(os.Args[3:], tasks, workers)
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
			cmdProjectStatus(projects, workers)
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

	case "planner":
		if len(os.Args) < 3 {
			fatal("usage: orchestra planner <start|stop|attach>")
		}
		switch os.Args[2] {
		case "start":
			cmdPlannerStart(workers, projects)
		case "stop":
			cmdPlannerStop(workers)
		case "attach":
			cmdPlannerAttach(workers)
		default:
			fatal("unknown planner subcommand: " + os.Args[2])
		}

	case "done":
		cmdDone(os.Args[2:], tasks, workers)

	case "run":
		cmdRun(os.Args[2:], tasks, workers, activeProj)

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

  orchestra planner start                            Select project + spawn planner (interactive)
  orchestra planner stop                             Kill the planner session
  orchestra planner attach                           Attach to planner's session

  orchestra worker spawn <name>                      Spawn a worker (worktrees for all project repos)
  orchestra worker list                              List all workers
  orchestra worker peek <name>                       See what a worker is doing right now
  orchestra worker kill <name>                       Kill a worker
  orchestra worker attach <name>                     Attach to worker's session

  orchestra nudge <name> <message>                   Send a message into a session

  orchestra task create --title "..." --desc "..."   Create a task
  orchestra task list                                List all tasks
  orchestra task status <task-id>                    Show task details
  orchestra task assign <task-id> <worker-name>      Assign task to worker

  orchestra done <worker-name>                       Mark worker and its task as done

  orchestra run --title "..." --desc "..."           Create + spawn + assign (all-in-one)`)
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

func cmdProjectStatus(store *project.Store, mgr *worker.Manager) {
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

	workers, _ := mgr.List()
	projectWorkers := 0
	for _, w := range workers {
		if w.ProjectID == proj.ID {
			projectWorkers++
		}
	}
	fmt.Printf("Workers: %d\n", projectWorkers)
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

// --- Planner commands ---

func cmdPlannerStart(mgr *worker.Manager, projStore *project.Store) {
	projects, err := projStore.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(projects) == 0 {
		fatal("no projects. Create one with: orchestra project create <name>")
	}

	var proj *project.Project
	if len(projects) == 1 {
		proj = &projects[0]
		fmt.Printf("Using project: %s\n", proj.Name)
	} else {
		fmt.Println("Select a project:")
		for i, p := range projects {
			fmt.Printf("  %d) %s (%d repos)\n", i+1, p.Name, len(p.Repos))
		}
		fmt.Print("\nEnter number: ")
		var choice int
		if _, err := fmt.Scan(&choice); err != nil {
			fatal("invalid input")
		}
		if choice < 1 || choice > len(projects) {
			fatal("invalid selection")
		}
		proj = &projects[choice-1]
	}

	// Set as active project for worker commands
	projStore.SetActive(proj.ID)

	if err := planner.Start(mgr, proj); err != nil {
		fatal(err.Error())
	}
}

func cmdPlannerStop(mgr *worker.Manager) {
	if err := planner.Stop(mgr); err != nil {
		fatal(err.Error())
	}
	fmt.Println("Planner stopped.")
}

func cmdPlannerAttach(mgr *worker.Manager) {
	if err := planner.Attach(mgr); err != nil {
		fatal(err.Error())
	}
}

// --- Worker commands ---

func cmdWorkerSpawn(args []string, mgr *worker.Manager, proj *project.Project) {
	if len(args) < 1 {
		fatal("usage: orchestra worker spawn <name>")
	}
	name := args[0]

	requireActiveProject(proj)

	w, err := mgr.Spawn(worker.SpawnOptions{
		Name:    name,
		Project: proj,
	})
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned worker %q (session: %s, workdir: %s, worktrees: %d)\n",
		w.Name, w.Session, w.WorkDir, len(w.WorktreeDirs))
}

func cmdWorkerList(mgr *worker.Manager) {
	workers, err := mgr.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(workers) == 0 {
		fmt.Println("No workers running.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tTASK\tWORKTREES\tSESSION")
	for _, wk := range workers {
		taskID := valueOr(wk.TaskID, "-")
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", wk.Name, wk.Status, taskID, len(wk.WorktreeDirs), wk.Session)
	}
	w.Flush()
}

func cmdWorkerKill(args []string, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra worker kill <name>")
	}
	if err := mgr.Kill(args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Killed worker %q\n", args[0])
}

func cmdWorkerAttach(args []string, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra worker attach <name>")
	}
	if err := mgr.Attach(args[0]); err != nil {
		fatal(err.Error())
	}
}

func cmdWorkerPeek(args []string, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra worker peek <name>")
	}
	name := args[0]

	w, err := mgr.Get(name)
	if err != nil {
		fatal(err.Error())
	}
	if !tmux.HasSession(w.Session) {
		fatal(fmt.Sprintf("worker %q session is not running", name))
	}

	output, err := tmux.CapturePane(w.Session, 100)
	if err != nil {
		fatal(fmt.Sprintf("capture pane: %v", err))
	}
	fmt.Println(output)
}

func cmdNudge(args []string, mgr *worker.Manager) {
	if len(args) < 2 {
		fatal("usage: orchestra nudge <name> <message>")
	}
	name := args[0]
	message := strings.Join(args[1:], " ")

	session := mgr.SessionName(name)
	if !tmux.HasSession(session) {
		fatal(fmt.Sprintf("session %q is not running", name))
	}

	if err := tmux.SendKeys(session, message); err != nil {
		fatal(fmt.Sprintf("nudge failed: %v", err))
	}
	fmt.Printf("Nudged %s.\n", name)
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

func cmdTaskAssign(args []string, store *task.Store, mgr *worker.Manager) {
	if len(args) < 2 {
		fatal("usage: orchestra task assign <task-id> <worker-name>")
	}
	taskID := args[0]
	workerName := args[1]

	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}
	if t.Status != task.StatusPending {
		fatal(fmt.Sprintf("task %s is %s, can only assign pending tasks", taskID, t.Status))
	}

	w, err := mgr.Get(workerName)
	if err != nil {
		fatal(err.Error())
	}
	if w.Status != worker.StatusIdle {
		fatal(fmt.Sprintf("worker %s is %s, can only assign to idle workers", workerName, w.Status))
	}

	prompt := buildPrompt(t, workerName)
	if err := mgr.Assign(workerName, prompt); err != nil {
		fatal(err.Error())
	}

	store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusAssigned
		t.AssignedTo = workerName
	})
	mgr.Update(workerName, func(w *worker.Worker) {
		w.Status = worker.StatusWorking
		w.TaskID = taskID
	})

	fmt.Printf("Assigned task %s to worker %s\n", taskID, workerName)
}

// --- Run command (convenience) ---

func cmdRun(args []string, store *task.Store, mgr *worker.Manager, proj *project.Project) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("desc", "", "Task description / prompt")
	workerName := fs.String("worker", "", "Worker name (auto-generated if empty)")
	fs.Parse(args)

	if *title == "" {
		fatal("--title is required")
	}

	requireActiveProject(proj)

	t, err := store.Create(*title, *desc)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created task %s: %s\n", t.ID, t.Title)

	name := *workerName
	if name == "" {
		name = "worker-" + t.ID
	}

	w, err := mgr.Spawn(worker.SpawnOptions{
		Name:    name,
		Project: proj,
	})
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned worker %q (session: %s)\n", w.Name, w.Session)

	prompt := buildPrompt(t, name)
	if err := mgr.Assign(name, prompt); err != nil {
		fatal(err.Error())
	}

	store.Update(t.ID, func(t *task.Task) {
		t.Status = task.StatusAssigned
		t.AssignedTo = name
	})
	mgr.Update(name, func(w *worker.Worker) {
		w.Status = worker.StatusWorking
		w.TaskID = t.ID
	})

	fmt.Printf("Assigned task %s to worker %s\n", t.ID, name)
	fmt.Printf("\nAttach with: orchestra worker attach %s\n", name)
}

// --- Done command ---

func cmdDone(args []string, store *task.Store, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: orchestra done <worker-name>")
	}
	workerName := args[0]

	w, err := mgr.Get(workerName)
	if err != nil {
		fatal(err.Error())
	}

	// Mark the worker's task as done
	if w.TaskID != "" {
		store.Update(w.TaskID, func(t *task.Task) {
			t.Status = task.StatusDone
		})
		fmt.Printf("Task %s marked as done.\n", w.TaskID)
	}

	// Mark the worker as idle
	mgr.Update(workerName, func(w *worker.Worker) {
		w.Status = worker.StatusDone
		w.TaskID = ""
	})
	fmt.Printf("Worker %s marked as done.\n", workerName)
}

// --- Helpers ---

func buildPrompt(t *task.Task, workerName string) string {
	var b strings.Builder
	b.WriteString(t.Title)
	if t.Description != "" {
		b.WriteString("\n\n")
		b.WriteString(t.Description)
	}
	b.WriteString("\n\n---\n")
	b.WriteString("IMPORTANT: When you have completed this task:\n")
	b.WriteString("1. Commit your changes in each repo you modified\n")
	b.WriteString("2. Push your branches: git push -u origin HEAD (in each repo)\n")
	b.WriteString("3. Create a PR targeting develop: gh pr create --base develop --fill\n")
	b.WriteString("4. Mark yourself as done: orchestra done " + workerName + "\n")
	b.WriteString("\nIf you need help or want to report progress, message the planner:\n")
	b.WriteString("  orchestra nudge planner \"your message here\"\n")
	return b.String()
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
