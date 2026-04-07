package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tomy/v1/internal/config"
	"github.com/tomy/v1/internal/msg"
	"github.com/tomy/v1/internal/nudge"
	"github.com/tomy/v1/internal/plan"
	"github.com/tomy/v1/internal/planner"
	"github.com/tomy/v1/internal/project"
	"github.com/tomy/v1/internal/state"
	"github.com/tomy/v1/internal/task"
	"github.com/tomy/v1/internal/tmux"
	"github.com/tomy/v1/internal/worker"
)

const version = "0.2.0"

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	if os.Args[1] == "--version" || os.Args[1] == "version" {
		fmt.Println("tomy v" + version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}

	db, err := state.Open(cfg.DBPath)
	if err != nil {
		fatal(err.Error())
	}
	defer db.Close()

	workers := worker.NewManager(db, cfg.WorkspacesDir, cfg.SessionPrefix)
	tasks := task.NewStore(db)
	plans := plan.NewStore(db)
	projects := project.NewStore(db)
	messages := msg.NewStore(db)
	nudges := nudge.NewQueue(db)

	// Load active project (nil if none set)
	activeProj, _ := projects.GetActive()

	switch os.Args[1] {
	case "worker":
		if len(os.Args) < 3 {
			fatal("usage: tomy worker <spawn|list|kill|attach|peek>")
		}
		switch os.Args[2] {
		case "spawn":
			cmdWorkerSpawn(os.Args[3:], workers, activeProj)
		case "list":
			cmdWorkerList(workers, plans, tasks)
		case "kill":
			cmdWorkerKill(os.Args[3:], workers)
		case "attach":
			cmdWorkerAttach(os.Args[3:], workers)
		case "peek":
			cmdWorkerPeek(os.Args[3:], workers)
		default:
			fatal("unknown worker subcommand: " + os.Args[2])
		}

	case "msg":
		if len(os.Args) < 3 {
			fatal("usage: tomy msg <send|inbox>")
		}
		switch os.Args[2] {
		case "send":
			cmdMsgSend(os.Args[3:], messages, workers, nudges)
		case "inbox":
			cmdMsgInbox(os.Args[3:], messages, nudges)
		default:
			fatal("unknown msg subcommand: " + os.Args[2])
		}

	case "task":
		if len(os.Args) < 3 {
			fatal("usage: tomy task <create|edit|delete|move|list|status|start|done|block|unblock>")
		}
		switch os.Args[2] {
		case "create":
			cmdTaskCreate(os.Args[3:], tasks)
		case "edit":
			cmdTaskEdit(os.Args[3:], tasks)
		case "delete":
			cmdTaskDelete(os.Args[3:], tasks)
		case "move":
			cmdTaskMove(os.Args[3:], tasks)
		case "list":
			cmdTaskList(tasks)
		case "status":
			cmdTaskStatus(os.Args[3:], tasks)
		case "start":
			cmdTaskStart(os.Args[3:], tasks)
		case "done":
			cmdTaskDone(os.Args[3:], tasks, plans, workers, messages, nudges)
		case "block":
			cmdTaskBlock(os.Args[3:], tasks)
		case "unblock":
			cmdTaskUnblock(os.Args[3:], tasks)
		default:
			fatal("unknown task subcommand: " + os.Args[2])
		}

	case "plan":
		if len(os.Args) < 3 {
			fatal("usage: tomy plan <create|edit|list|show|assign>")
		}
		switch os.Args[2] {
		case "create":
			cmdPlanCreate(os.Args[3:], plans)
		case "edit":
			cmdPlanEdit(os.Args[3:], plans)
		case "list":
			cmdPlanList(plans, tasks)
		case "show":
			cmdPlanShow(os.Args[3:], plans, tasks)
		case "assign":
			cmdPlanAssign(os.Args[3:], plans, tasks, workers)
		default:
			fatal("unknown plan subcommand: " + os.Args[2])
		}

	case "project":
		if len(os.Args) < 3 {
			fatal("usage: tomy project <create|list|remove|set|status>")
		}
		switch os.Args[2] {
		case "create":
			cmdProjectCreate(os.Args[3:], projects)
		case "list":
			cmdProjectList(projects)
		case "remove":
			cmdProjectRemove(os.Args[3:], projects)
		case "set":
			cmdProjectSet(os.Args[3:], projects)
		case "status":
			cmdProjectStatus(projects, workers)
		default:
			fatal("unknown project subcommand: " + os.Args[2])
		}

	case "repo":
		if len(os.Args) < 3 {
			fatal("usage: tomy repo <add|list|remove|setup>")
		}
		switch os.Args[2] {
		case "add":
			cmdRepoAdd(os.Args[3:], projects, activeProj)
		case "list":
			cmdRepoList(activeProj)
		case "remove":
			cmdRepoRemove(os.Args[3:], projects, activeProj)
		case "setup":
			cmdRepoSetup(os.Args[3:], projects, activeProj)
		default:
			fatal("unknown repo subcommand: " + os.Args[2])
		}

	case "planner":
		if len(os.Args) < 3 {
			fatal("usage: tomy planner <start|stop|attach>")
		}
		switch os.Args[2] {
		case "start":
			cmdPlannerStart(workers, projects, cfg.PlannerDir)
		case "stop":
			cmdPlannerStop(workers)
		case "attach":
			cmdPlannerAttach(workers)
		default:
			fatal("unknown planner subcommand: " + os.Args[2])
		}

	case "done":
		cmdDone(os.Args[2:], plans, tasks, workers, messages, nudges)

	case "run":
		cmdRun(os.Args[2:], plans, tasks, workers, activeProj)

	case "monitor":
		cmdMonitor(os.Args[2:], plans, tasks, workers, activeProj)

	case "completion":
		cmdCompletion(os.Args[2:])

	case "help", "--help", "-h":
		printUsage()

	default:
		fatal("unknown command: " + os.Args[1])
	}
}

func printUsage() {
	fmt.Println(`tomy - multi-agent Claude Code tomytor (v1)

Usage:
  tomy project create <name>                        Create a new project
  tomy project list                                 List all projects
  tomy project remove <name|id>                     Remove a project
  tomy project set <name>                           Set active project
  tomy project status                               Show active project details

  tomy repo add <path> [--name <n>] [--setup <cmd>] Add a repo to active project
  tomy repo list                                    List repos in active project
  tomy repo remove <name>                           Remove a repo
  tomy repo setup <name> --cmd <command>             Set/update post-worktree setup command
  tomy repo setup <name>                            Show current setup command

  tomy plan create --name "..." [--desc "..."]       Create a plan
  tomy plan edit <plan-id> --name "..."             Rename a plan
  tomy plan list                                    List all plans with progress
  tomy plan show <plan-id>                          Show plan tasks with completion percentage
  tomy plan assign <plan-id> <worker-name>          Assign plan to a worker

  tomy planner start                                Select project + spawn planner (interactive)
  tomy planner stop                                 Kill the planner session
  tomy planner attach                               Attach to planner's session

  tomy worker spawn <name>                          Spawn a worker (worktrees + run setup commands)
  tomy worker list                                  List all workers with plan progress
  tomy worker peek <name>                           See what a worker is doing right now
  tomy worker kill <name>                           Kill a worker
  tomy worker attach <name>                         Attach to worker's session

  tomy msg send <to> <message> --from <name>        Send a message (idle: direct, busy: queued)
  tomy msg inbox <name>                             Read unread messages
  tomy msg inbox <name> --inject                    Drain nudge queue as system-reminder (for hooks)

  tomy task create --plan <id> --title "..." --desc "..."  Create a task under a plan
  tomy task edit <task-id> --title "..." --desc "..."      Edit a task's title or description
  tomy task delete <task-id>                               Remove a task
  tomy task move <task-id> --before <task-id>              Reorder a task within a plan
  tomy task list                                           List all tasks
  tomy task status <task-id>                               Show task details
  tomy task start <task-id>                                Mark a task as in-progress
  tomy task done <task-id>                                 Mark a task as done
  tomy task block <task-id> --reason "..."                 Mark a task as blocked
  tomy task unblock <task-id>                              Unblock a task (back to in-progress)

  tomy done <worker-name>                           Mark worker and all plan tasks as done

  tomy run --name "..." --title "..." --desc "..."  Create plan + task + spawn + assign (all-in-one)

  tomy monitor [--interval <seconds>]               Live dashboard of plans and tasks

  tomy completion <zsh|bash>                        Output shell completion script

Worktree Setup:
  Git worktrees don't include gitignored files (.env, configs). Attach a setup
  command to a repo and it runs automatically in each new worktree after spawn.

  tomy repo add ./api --setup 'cp "$TOMY_REPO_PATH/.env" .'
  tomy repo setup api --cmd 'docker compose -p "api-$TOMY_WORKER_NAME" up -d'

  Setup commands run via "sh -c" with a 60s timeout. Failures warn but don't
  block worker creation. The following env vars are available:

    TOMY_WORKTREE_PATH   Absolute path to the created worktree
    TOMY_REPO_PATH       Absolute path to the original repo
    TOMY_REPO_NAME       Name of the repo
    TOMY_WORKER_NAME     Name of the worker
    TOMY_WORKER_INDEX    0-based index (for port offsetting)
    TOMY_WORKSPACE_DIR   Worker's workspace root directory`)
}

// --- Project commands ---

func cmdProjectCreate(args []string, store *project.Store) {
	if len(args) < 1 {
		fatal("usage: tomy project create <name>")
	}
	p, err := store.Create(args[0])
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created project %q (id: %s, set as active)\n", p.Name, p.ID)
}

func cmdProjectRemove(args []string, store *project.Store) {
	if len(args) < 1 {
		fatal("usage: tomy project remove <name|id>")
	}
	p, err := store.Get(args[0])
	if err != nil {
		fatal(err.Error())
	}
	if err := store.Remove(args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Removed project %q (id: %s)\n", p.Name, p.ID)
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
		fatal("usage: tomy project set <name-or-id>")
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
		fmt.Println("No active project. Create one with: tomy project create <name>")
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
		fatal("no active project. Create one with: tomy project create <name>")
	}
}

func cmdRepoAdd(args []string, store *project.Store, proj *project.Project) {
	requireActiveProject(proj)

	fs := flag.NewFlagSet("repo add", flag.ExitOnError)
	name := fs.String("name", "", "Repo name (defaults to directory basename)")
	setup := fs.String("setup", "", "Shell command to run after worktree creation")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fatal("usage: tomy repo add <path> [--name <name>] [--setup <command>]")
	}
	path := fs.Arg(0)

	repoName := *name
	if repoName == "" {
		repoName = filepath.Base(path)
	}

	r, err := store.AddRepo(proj.ID, repoName, path, *setup)
	if err != nil {
		fatal(err.Error())
	}

	gitLabel := ""
	if r.IsGitRepo {
		gitLabel = " (git detected)"
	}
	fmt.Printf("Added repo %q: %s%s\n", r.Name, r.Path, gitLabel)
	if r.SetupCommand != "" {
		fmt.Printf("  setup: %s\n", r.SetupCommand)
	}
}

func cmdRepoList(proj *project.Project) {
	requireActiveProject(proj)

	if len(proj.Repos) == 0 {
		fmt.Println("No repos in project. Add one with: tomy repo add <path>")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tGIT\tSETUP")
	for _, r := range proj.Repos {
		gitLabel := "no"
		if r.IsGitRepo {
			gitLabel = "yes"
		}
		setupLabel := "-"
		if r.SetupCommand != "" {
			setupLabel = r.SetupCommand
			if len(setupLabel) > 50 {
				setupLabel = setupLabel[:47] + "..."
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Path, gitLabel, setupLabel)
	}
	w.Flush()
}

func cmdRepoSetup(args []string, store *project.Store, proj *project.Project) {
	requireActiveProject(proj)

	fs := flag.NewFlagSet("repo setup", flag.ExitOnError)
	cmd := fs.String("cmd", "", "Shell command to run after worktree creation")
	// Reorder args so flags come before positional args,
	// because Go's flag package stops parsing at the first non-flag argument.
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	if fs.NArg() < 1 {
		fatal("usage: tomy repo setup <repo-name> --cmd <command>")
	}
	repoName := fs.Arg(0)

	// If --cmd not provided, show current setup
	cmdSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "cmd" {
			cmdSet = true
		}
	})

	if !cmdSet {
		r, err := store.GetRepo(proj, repoName)
		if err != nil {
			fatal(err.Error())
		}
		if r.SetupCommand == "" {
			fmt.Printf("No setup command for repo %q\n", repoName)
		} else {
			fmt.Printf("Setup command for %q: %s\n", repoName, r.SetupCommand)
		}
		return
	}

	if err := store.SetRepoSetup(proj.ID, repoName, *cmd); err != nil {
		fatal(err.Error())
	}
	if *cmd == "" {
		fmt.Printf("Cleared setup command for repo %q\n", repoName)
	} else {
		fmt.Printf("Set setup command for %q: %s\n", repoName, *cmd)
	}
}

func cmdRepoRemove(args []string, store *project.Store, proj *project.Project) {
	requireActiveProject(proj)

	if len(args) < 1 {
		fatal("usage: tomy repo remove <name>")
	}
	if err := store.RemoveRepo(proj.ID, args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Removed repo %q\n", args[0])
}

// --- Plan commands ---

func cmdPlanCreate(args []string, store *plan.Store) {
	fs := flag.NewFlagSet("plan create", flag.ExitOnError)
	name := fs.String("name", "", "Plan name (required)")
	desc := fs.String("desc", "", "Plan description (written to the plan's markdown file)")
	fs.Parse(args)

	if *name == "" {
		fatal("--name is required")
	}

	p, err := store.Create(*name)
	if err != nil {
		fatal(err.Error())
	}

	// Initialize plan content with header and optional description
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Plan: %s\n\nID: %s\n", p.Name, p.ID))
	if *desc != "" {
		content.WriteString(fmt.Sprintf("\n%s\n", *desc))
	}
	if err := store.SetContent(p.ID, []byte(content.String())); err != nil {
		fatal(fmt.Sprintf("write plan content: %v", err))
	}

	fmt.Printf("Created plan %s: %s\n", p.ID, p.Name)
	fmt.Printf("\nAdd tasks: tomy task create --plan %s --title \"...\"\n", p.ID)
}

func cmdPlanEdit(args []string, store *plan.Store) {
	fs := flag.NewFlagSet("plan edit", flag.ExitOnError)
	name := fs.String("name", "", "New plan name")
	// Reorder args so flags come before positional args
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	if fs.NArg() < 1 {
		fatal("usage: tomy plan edit <plan-id> --name \"...\"")
	}
	planID := fs.Arg(0)

	if *name == "" {
		fatal("--name is required")
	}

	err := store.Update(planID, func(p *plan.Plan) {
		p.Name = *name
	})
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf("Updated plan %s\n", planID)
}

func cmdPlanList(store *plan.Store, tasks *task.Store) {
	allPlans, err := store.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(allPlans) == 0 {
		fmt.Println("No plans.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tWORKER\tPROGRESS")
	for _, p := range allPlans {
		workerName := valueOr(p.WorkerName, "-")
		progress := "-"
		planTasks, _ := tasks.ListByPlan(p.ID)
		if len(planTasks) > 0 {
			doneCount := 0
			for _, t := range planTasks {
				if t.Status == task.StatusDone {
					doneCount++
				}
			}
			pct := (doneCount * 100) / len(planTasks)
			progress = fmt.Sprintf("%d/%d (%d%%)", doneCount, len(planTasks), pct)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.ID, p.Name, p.Status, workerName, progress)
	}
	w.Flush()
}

func cmdPlanShow(args []string, store *plan.Store, tasks *task.Store) {
	if len(args) < 1 {
		fatal("usage: tomy plan show <plan-id>")
	}
	planID := args[0]

	p, err := store.Get(planID)
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf("Plan:   %s (%s)\n", p.Name, p.ID)
	fmt.Printf("Status: %s\n", p.Status)
	fmt.Printf("Worker: %s\n", valueOr(p.WorkerName, "-"))

	planTasks, _ := tasks.ListByPlan(p.ID)
	if len(planTasks) == 0 {
		fmt.Println("\nNo tasks in this plan.")
		return
	}

	doneCount := 0
	blockedCount := 0
	for _, t := range planTasks {
		if t.Status == task.StatusDone {
			doneCount++
		} else if t.Status == task.StatusBlocked {
			blockedCount++
		}
	}
	pct := (doneCount * 100) / len(planTasks)
	progress := fmt.Sprintf("\nProgress: %d/%d tasks done", doneCount, len(planTasks))
	if blockedCount > 0 {
		progress += fmt.Sprintf(", %d blocked", blockedCount)
	}
	progress += fmt.Sprintf(" (%d%%)\n\n", pct)
	fmt.Print(progress)

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tID\tTITLE")
	for _, t := range planTasks {
		marker := fmt.Sprintf("[%s]", t.Status)
		title := t.Title
		if t.Status == task.StatusBlocked && t.BlockedReason != "" {
			title += " — " + t.BlockedReason
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", marker, t.ID, title)
	}
	tw.Flush()
}

func cmdPlanAssign(args []string, store *plan.Store, tasks *task.Store, mgr *worker.Manager) {
	if len(args) < 2 {
		fatal("usage: tomy plan assign <plan-id> <worker-name>")
	}
	planID := args[0]
	workerName := args[1]

	p, err := store.Get(planID)
	if err != nil {
		fatal(err.Error())
	}
	if p.Status != plan.StatusDraft {
		fatal(fmt.Sprintf("plan %s is %s, can only assign draft plans", planID, p.Status))
	}

	w, err := mgr.Get(workerName)
	if err != nil {
		fatal(err.Error())
	}
	if w.Status != worker.StatusIdle {
		fatal(fmt.Sprintf("worker %s is %s, can only assign to idle workers", workerName, w.Status))
	}

	// Build the plan content file with all tasks
	planTasks, _ := tasks.ListByPlan(planID)
	if len(planTasks) == 0 {
		fatal("plan has no tasks — add tasks first with: tomy task create --plan " + planID + " --title \"...\"")
	}

	prompt := buildPlanPrompt(p, planTasks, workerName, store)
	if err := store.SetContent(planID, []byte(prompt)); err != nil {
		fatal(fmt.Sprintf("write plan content: %v", err))
	}

	// Deliver the plan to the worker
	if err := mgr.Assign(workerName, []byte(prompt)); err != nil {
		fatal(err.Error())
	}

	// Update plan
	store.Update(planID, func(p *plan.Plan) {
		p.Status = plan.StatusAssigned
		p.WorkerName = workerName
	})

	// Update all tasks
	for _, t := range planTasks {
		tasks.Update(t.ID, func(t *task.Task) {
			t.Status = task.StatusAssigned
		})
	}

	// Update worker
	mgr.Update(workerName, func(w *worker.Worker) {
		w.Status = worker.StatusWorking
		w.PlanID = planID
	})

	fmt.Printf("Assigned plan %q (%d tasks) to worker %s\n", p.Name, len(planTasks), workerName)
}

// --- Planner commands ---

func cmdPlannerStart(mgr *worker.Manager, projStore *project.Store, plannerDir string) {
	projects, err := projStore.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(projects) == 0 {
		fatal("no projects. Create one with: tomy project create <name>")
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

	if err := planner.Start(mgr, proj, plannerDir); err != nil {
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
		fatal("usage: tomy worker spawn <name>")
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

func cmdWorkerList(mgr *worker.Manager, plans *plan.Store, tasks *task.Store) {
	allWorkers, err := mgr.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(allWorkers) == 0 {
		fmt.Println("No workers running.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPLAN\tPROGRESS\tWORKTREES\tSESSION")
	for _, wk := range allWorkers {
		planInfo := "-"
		progress := "-"
		if wk.PlanID != "" {
			p, err := plans.Get(wk.PlanID)
			if err == nil {
				planInfo = p.Name
				planTasks, _ := tasks.ListByPlan(p.ID)
				if len(planTasks) > 0 {
					doneCount := 0
					for _, t := range planTasks {
						if t.Status == task.StatusDone {
							doneCount++
						}
					}
					pct := (doneCount * 100) / len(planTasks)
					progress = fmt.Sprintf("%d/%d (%d%%)", doneCount, len(planTasks), pct)
				}
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", wk.Name, wk.Status, planInfo, progress, len(wk.WorktreeDirs), wk.Session)
	}
	w.Flush()
}

func cmdWorkerKill(args []string, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: tomy worker kill <name>")
	}
	if err := mgr.Kill(args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Killed worker %q\n", args[0])
}

func cmdWorkerAttach(args []string, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: tomy worker attach <name>")
	}
	if err := mgr.Attach(args[0]); err != nil {
		fatal(err.Error())
	}
}

func cmdWorkerPeek(args []string, mgr *worker.Manager) {
	if len(args) < 1 {
		fatal("usage: tomy worker peek <name>")
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

// --- Message commands ---

func cmdMsgSend(args []string, store *msg.Store, mgr *worker.Manager, nq *nudge.Queue) {
	fs := flag.NewFlagSet("msg send", flag.ExitOnError)
	from := fs.String("from", "unknown", "Sender name")
	// Reorder args so flags come before positional args,
	// because Go's flag package stops parsing at the first non-flag argument.
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	if fs.NArg() < 2 {
		fatal("usage: tomy msg send <to> <message> --from <name>")
	}
	to := fs.Arg(0)
	content := strings.Join(fs.Args()[1:], " ")

	// Write to inbox
	m, err := store.Send(*from, to, content)
	if err != nil {
		fatal(fmt.Sprintf("send failed: %v", err))
	}
	fmt.Printf("Message %s sent to %s.\n", m.ID, to)

	// Notify the recipient
	session := mgr.SessionName(to)
	if !tmux.HasSession(session) {
		return
	}

	// If the session is idle, deliver directly via send-keys
	if tmux.IsIdle(session, 3*time.Second) {
		notification := fmt.Sprintf("You have a new message from %s. Run: tomy msg inbox %s", *from, to)
		tmux.SendKeys(session, notification)
		fmt.Printf("Delivered directly (session was idle).\n")
		return
	}

	// Session is busy — queue the nudge for pickup at next turn boundary
	if err := nq.Enqueue(*from, to, content); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not queue nudge: %v\n", err)
		return
	}
	fmt.Printf("Session busy — nudge queued (will be delivered at next turn).\n")
}

func cmdMsgInbox(args []string, store *msg.Store, nq *nudge.Queue) {
	fs := flag.NewFlagSet("msg inbox", flag.ExitOnError)
	inject := fs.Bool("inject", false, "Drain nudge queue and output as system-reminder (for hooks)")
	// Reorder args so flags come before positional args
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	if fs.NArg() < 1 {
		fatal("usage: tomy msg inbox <name> [--inject]")
	}
	name := fs.Arg(0)

	if *inject {
		// Hook mode: drain queued nudges and print as system-reminder
		nudges, err := nq.Drain(name)
		if err != nil {
			// Silently fail in hook mode — don't break the prompt
			return
		}
		output := nudge.FormatForInjection(nudges)
		if output != "" {
			fmt.Print(output)
		}
		return
	}

	// Normal interactive mode
	unread, err := store.Unread(name)
	if err != nil {
		fatal(err.Error())
	}

	if len(unread) == 0 {
		fmt.Println("No new messages.")
		return
	}

	for _, m := range unread {
		ago := time.Since(m.CreatedAt).Truncate(time.Second)
		fmt.Printf("[%s] from %s (%s ago):\n  %s\n\n", m.ID, m.From, ago, m.Content)
	}

	// Mark all as read
	if err := store.MarkAllRead(name); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not mark messages as read: %v\n", err)
	}
}

// --- Task commands ---

func cmdTaskCreate(args []string, store *task.Store) {
	fs := flag.NewFlagSet("task create", flag.ExitOnError)
	planID := fs.String("plan", "", "Plan ID (required)")
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("desc", "", "Task description / prompt for Claude")
	fs.Parse(args)

	if *planID == "" {
		fatal("--plan is required")
	}
	if *title == "" {
		fatal("--title is required")
	}

	t, err := store.Create(*title, *desc)
	if err != nil {
		fatal(err.Error())
	}

	// Link task to plan
	store.Update(t.ID, func(t *task.Task) {
		t.PlanID = *planID
	})

	fmt.Printf("Created task %s: %s (plan: %s)\n", t.ID, t.Title, *planID)
}

func cmdTaskEdit(args []string, store *task.Store) {
	fs := flag.NewFlagSet("task edit", flag.ExitOnError)
	title := fs.String("title", "", "New task title")
	desc := fs.String("desc", "", "New task description")
	// Reorder args so flags come before positional args
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	if fs.NArg() < 1 {
		fatal("usage: tomy task edit <task-id> [--title \"...\"] [--desc \"...\"]")
	}
	taskID := fs.Arg(0)

	if *title == "" && *desc == "" {
		fatal("at least one of --title or --desc is required")
	}

	err := store.Update(taskID, func(t *task.Task) {
		if *title != "" {
			t.Title = *title
		}
		if *desc != "" {
			t.Description = *desc
		}
	})
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf("Updated task %s\n", taskID)
}

func cmdTaskDelete(args []string, store *task.Store) {
	if len(args) < 1 {
		fatal("usage: tomy task delete <task-id>")
	}
	taskID := args[0]

	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}

	if err := store.Delete(taskID); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Deleted task %s: %s\n", taskID, t.Title)
}

func cmdTaskMove(args []string, store *task.Store) {
	fs := flag.NewFlagSet("task move", flag.ExitOnError)
	before := fs.String("before", "", "Task ID to place this task before")
	// Reorder args so flags come before positional args
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))

	if fs.NArg() < 1 {
		fatal("usage: tomy task move <task-id> --before <target-task-id>")
	}
	taskID := fs.Arg(0)

	if *before == "" {
		fatal("--before is required")
	}

	if err := store.Move(taskID, *before); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Moved task %s before %s\n", taskID, *before)
}

func cmdTaskList(store *task.Store) {
	allTasks, err := store.List()
	if err != nil {
		fatal(err.Error())
	}
	if len(allTasks) == 0 {
		fmt.Println("No tasks.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tPLAN")
	for _, t := range allTasks {
		planID := valueOr(t.PlanID, "-")
		title := t.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, title, t.Status, planID)
	}
	w.Flush()
}

func cmdTaskStatus(args []string, store *task.Store) {
	if len(args) < 1 {
		fatal("usage: tomy task status <task-id>")
	}

	t, err := store.Get(args[0])
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf("ID:      %s\n", t.ID)
	fmt.Printf("Title:   %s\n", t.Title)
	fmt.Printf("Status:  %s\n", t.Status)
	if t.Status == task.StatusBlocked && t.BlockedReason != "" {
		fmt.Printf("Blocked: %s\n", t.BlockedReason)
	}
	fmt.Printf("Plan:    %s\n", valueOr(t.PlanID, "-"))
	fmt.Printf("Created: %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))
	if t.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", t.Description)
	}
	if t.Result != "" {
		fmt.Printf("\nResult:\n%s\n", t.Result)
	}
}

func cmdTaskStart(args []string, store *task.Store) {
	if len(args) < 1 {
		fatal("usage: tomy task start <task-id>")
	}
	taskID := args[0]

	err := store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusInProgress
	})
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Task %s is now in-progress.\n", taskID)
}

func cmdTaskBlock(args []string, store *task.Store) {
	fs := flag.NewFlagSet("task block", flag.ExitOnError)
	reason := fs.String("reason", "", "Why the task is blocked")
	// Reorder args so flags come before positional args
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}
	fs.Parse(append(flagArgs, posArgs...))
	remaining := fs.Args()

	if len(remaining) < 1 {
		fatal("usage: tomy task block <task-id> --reason \"...\"")
	}
	if *reason == "" {
		fatal("--reason is required")
	}

	taskID := remaining[0]
	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}
	if t.Status == task.StatusDone {
		fatal(fmt.Sprintf("task %s is already done", taskID))
	}

	store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusBlocked
		t.BlockedReason = *reason
	})
	fmt.Printf("Task %s blocked: %s\n", taskID, *reason)
}

func cmdTaskUnblock(args []string, store *task.Store) {
	if len(args) < 1 {
		fatal("usage: tomy task unblock <task-id>")
	}
	taskID := args[0]

	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}
	if t.Status != task.StatusBlocked {
		fatal(fmt.Sprintf("task %s is not blocked (status: %s)", taskID, t.Status))
	}

	store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusInProgress
		t.BlockedReason = ""
	})
	fmt.Printf("Task %s unblocked (now in-progress).\n", taskID)
}

func cmdTaskDone(args []string, store *task.Store, plans *plan.Store, mgr *worker.Manager, inbox *msg.Store, nq *nudge.Queue) {
	if len(args) < 1 {
		fatal("usage: tomy task done <task-id>")
	}
	taskID := args[0]

	t, err := store.Get(taskID)
	if err != nil {
		fatal(err.Error())
	}
	if t.Status == task.StatusDone {
		fatal(fmt.Sprintf("task %s is already done", taskID))
	}

	store.Update(taskID, func(t *task.Task) {
		t.Status = task.StatusDone
	})
	fmt.Printf("Task %s marked as done: %s\n", taskID, t.Title)

	// Check if all tasks in the plan are done
	if t.PlanID == "" {
		return
	}

	p, err := plans.Get(t.PlanID)
	if err != nil {
		return
	}

	planTasks, _ := store.ListByPlan(p.ID)
	total := len(planTasks)
	doneCount := 0
	allDone := true
	for _, pt := range planTasks {
		if pt.Status == task.StatusDone {
			doneCount++
		} else {
			allDone = false
		}
	}

	pct := 0
	if total > 0 {
		pct = (doneCount * 100) / total
	}
	fmt.Printf("Plan %q progress: %d/%d tasks done (%d%%)\n", p.Name, doneCount, total, pct)

	if allDone && total > 0 {
		// Mark plan as done
		plans.Update(p.ID, func(p *plan.Plan) {
			p.Status = plan.StatusDone
		})
		fmt.Printf("Plan %q complete!\n", p.Name)

		// Mark worker as done if assigned
		if p.WorkerName != "" {
			mgr.Update(p.WorkerName, func(w *worker.Worker) {
				w.Status = worker.StatusDone
			})
			fmt.Printf("Worker %s marked as done.\n", p.WorkerName)

			// Notify planner
			notification := fmt.Sprintf("Worker %s has finished plan %q (%d tasks).", p.WorkerName, p.Name, total)
			m, err := inbox.Send(p.WorkerName, "planner", notification)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not notify planner: %v\n", err)
				return
			}
			fmt.Printf("Planner notified (msg %s).\n", m.ID)

			notifyPlanner(mgr, nq, p.WorkerName, notification)
		}
	}
}

// --- Run command (convenience) ---

func cmdRun(args []string, plans *plan.Store, tasks *task.Store, mgr *worker.Manager, proj *project.Project) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	name := fs.String("name", "", "Plan name (required)")
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("desc", "", "Task description / prompt")
	workerName := fs.String("worker", "", "Worker name (auto-generated if empty)")
	fs.Parse(args)

	if *name == "" {
		fatal("--name is required")
	}
	if *title == "" {
		fatal("--title is required")
	}

	requireActiveProject(proj)

	// Create plan
	p, err := plans.Create(*name)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created plan %s: %s\n", p.ID, p.Name)

	// Create task under plan
	t, err := tasks.Create(*title, *desc)
	if err != nil {
		fatal(err.Error())
	}
	tasks.Update(t.ID, func(t *task.Task) {
		t.PlanID = p.ID
	})
	fmt.Printf("Created task %s: %s\n", t.ID, t.Title)

	// Spawn worker
	wName := *workerName
	if wName == "" {
		wName = "worker-" + p.ID
	}

	w, err := mgr.Spawn(worker.SpawnOptions{
		Name:    wName,
		Project: proj,
	})
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Spawned worker %q (session: %s)\n", w.Name, w.Session)

	// Build and deliver plan
	planTasks := []task.Task{*t}
	prompt := buildPlanPrompt(p, planTasks, wName, plans)
	if err := plans.SetContent(p.ID, []byte(prompt)); err != nil {
		fatal(fmt.Sprintf("write plan content: %v", err))
	}
	if err := mgr.Assign(wName, []byte(prompt)); err != nil {
		fatal(err.Error())
	}

	// Update statuses
	plans.Update(p.ID, func(p *plan.Plan) {
		p.Status = plan.StatusAssigned
		p.WorkerName = wName
	})
	tasks.Update(t.ID, func(t *task.Task) {
		t.Status = task.StatusAssigned
	})
	mgr.Update(wName, func(w *worker.Worker) {
		w.Status = worker.StatusWorking
		w.PlanID = p.ID
	})

	fmt.Printf("Assigned plan %q to worker %s\n", p.Name, wName)
	fmt.Printf("\nAttach with: tomy worker attach %s\n", wName)
}

// --- Done command ---

func cmdDone(args []string, plans *plan.Store, store *task.Store, mgr *worker.Manager, inbox *msg.Store, nq *nudge.Queue) {
	if len(args) < 1 {
		fatal("usage: tomy done <worker-name>")
	}
	workerName := args[0]

	w, err := mgr.Get(workerName)
	if err != nil {
		fatal(err.Error())
	}

	// Mark all plan tasks as done
	if w.PlanID != "" {
		planTasks, _ := store.ListByPlan(w.PlanID)
		doneCount := 0
		for _, t := range planTasks {
			if t.Status != task.StatusDone {
				store.Update(t.ID, func(t *task.Task) {
					t.Status = task.StatusDone
				})
				doneCount++
			}
		}
		if doneCount > 0 {
			fmt.Printf("Marked %d task(s) as done.\n", doneCount)
		}

		// Mark plan as done
		plans.Update(w.PlanID, func(p *plan.Plan) {
			p.Status = plan.StatusDone
		})
	}

	// Mark the worker as done
	mgr.Update(workerName, func(w *worker.Worker) {
		w.Status = worker.StatusDone
	})
	fmt.Printf("Worker %s marked as done.\n", workerName)

	// Notify the planner
	notification := fmt.Sprintf("Worker %s has finished.", workerName)
	m, err := inbox.Send(workerName, "planner", notification)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not notify planner: %v\n", err)
		return
	}
	fmt.Printf("Planner notified (msg %s).\n", m.ID)

	notifyPlanner(mgr, nq, workerName, notification)
}

// --- Helpers ---

func buildPlanPrompt(p *plan.Plan, planTasks []task.Task, workerName string, store *plan.Store) string {
	var b strings.Builder

	// Read existing plan content (has the description from plan create)
	existing, err := store.GetContent(p.ID)
	if err == nil && len(existing) > 0 {
		b.Write(existing)
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("# Plan: %s\n\nID: %s\n\n", p.Name, p.ID))
	}

	b.WriteString(fmt.Sprintf("Worker: %s\n\n", workerName))
	b.WriteString("## Tasks\n\n")
	for i, t := range planTasks {
		b.WriteString(fmt.Sprintf("### %d. %s (task: %s)\n\n", i+1, t.Title, t.ID))
		if t.Description != "" {
			b.WriteString(t.Description)
			b.WriteString("\n\n")
		}
		b.WriteString(fmt.Sprintf("When done: `tomy task done %s`\n\n", t.ID))
	}
	b.WriteString("---\n")
	b.WriteString("Follow your CLAUDE.md for completion and communication instructions.\n")
	b.WriteString(fmt.Sprintf("When all tasks are done: `tomy done %s`\n", workerName))
	return b.String()
}

func notifyPlanner(mgr *worker.Manager, nq *nudge.Queue, fromWorker, notification string) {
	session := mgr.SessionName("planner")
	if !tmux.HasSession(session) {
		return
	}
	if tmux.IsIdle(session, 3*time.Second) {
		hint := fmt.Sprintf("Worker %s is done. Run: tomy msg inbox planner", fromWorker)
		tmux.SendKeys(session, hint)
		fmt.Printf("Delivered directly (planner was idle).\n")
		return
	}
	if err := nq.Enqueue(fromWorker, "planner", notification); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not queue nudge: %v\n", err)
		return
	}
	fmt.Printf("Planner busy — nudge queued.\n")
}

// --- monitor command ---

const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
)

func statusColor(s string) string {
	switch s {
	case "done":
		return colorGreen
	case "in-progress":
		return colorYellow
	case "failed":
		return colorRed
	case "blocked":
		return colorMagenta
	case "assigned":
		return colorCyan
	default:
		return colorGray
	}
}

func statusIcon(s string) string {
	switch s {
	case "done":
		return "✓"
	case "in-progress":
		return "◆"
	case "failed":
		return "✗"
	case "blocked":
		return "⊘"
	case "assigned":
		return "○"
	default:
		return "·"
	}
}

func planIcon(s plan.Status) string {
	switch s {
	case plan.StatusDone:
		return "✓"
	case plan.StatusFailed:
		return "✗"
	default:
		return "⏵"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func sortPlans(plans []plan.Plan) {
	priority := map[plan.Status]int{
		plan.StatusInProgress: 0,
		plan.StatusAssigned:   1,
		plan.StatusDraft:      2,
		plan.StatusDone:       3,
		plan.StatusFailed:     4,
	}
	sort.Slice(plans, func(i, j int) bool {
		pi, pj := priority[plans[i].Status], priority[plans[j].Status]
		if pi != pj {
			return pi < pj
		}
		return plans[i].UpdatedAt.After(plans[j].UpdatedAt)
	})
}

func sortTasks(tasks []task.Task) {
	priority := map[task.Status]int{
		task.StatusInProgress: 0,
		task.StatusBlocked:    1,
		task.StatusAssigned:   2,
		task.StatusPending:    3,
		task.StatusDone:       4,
		task.StatusFailed:     5,
	}
	sort.Slice(tasks, func(i, j int) bool {
		pi, pj := priority[tasks[i].Status], priority[tasks[j].Status]
		if pi != pj {
			return pi < pj
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
}

func renderPlan(b *strings.Builder, p plan.Plan, planTasks []task.Task) {
	doneCount := 0
	for _, t := range planTasks {
		if t.Status == task.StatusDone {
			doneCount++
		}
	}

	color := statusColor(string(p.Status))
	icon := planIcon(p.Status)

	// Plan header
	fmt.Fprintf(b, "%s%s %s%s  %s[%s]%s", color, icon, colorBold, p.Name, color, p.Status, colorReset)
	if p.WorkerName != "" {
		fmt.Fprintf(b, "  worker: %s%s%s", colorCyan, p.WorkerName, colorReset)
	}
	if len(planTasks) > 0 {
		fmt.Fprintf(b, "  ── %s%d/%d%s done", colorBold, doneCount, len(planTasks), colorReset)
	}
	b.WriteString("\n")

	if len(planTasks) == 0 {
		fmt.Fprintf(b, "  %s(no tasks)%s\n", colorGray, colorReset)
		return
	}

	sortTasks(planTasks)

	for i, t := range planTasks {
		prefix := "├─"
		if i == len(planTasks)-1 {
			prefix = "└─"
		}
		tc := statusColor(string(t.Status))
		ti := statusIcon(string(t.Status))
		title := truncate(t.Title, 45)
		fmt.Fprintf(b, "  %s ● #%s  %-45s %s%s %s%s", prefix, t.ID, title, tc, ti, t.Status, colorReset)
		if t.Status == task.StatusBlocked && t.BlockedReason != "" {
			fmt.Fprintf(b, " — %s", t.BlockedReason)
		}
		b.WriteString("\n")
	}
}

func renderMonitor(b *strings.Builder, planStore *plan.Store, taskStore *task.Store, workers *worker.Manager, activeProj *project.Project, interval int) {
	b.Reset()
	b.WriteString("\033[H\033[2J")

	allPlans, err := planStore.List()
	if err != nil {
		fmt.Fprintf(b, "Error loading plans: %v\n", err)
		return
	}

	// Filter plans to active project
	if activeProj != nil {
		allWorkers, _ := workers.List()
		projectWorkers := make(map[string]bool)
		for _, w := range allWorkers {
			if w.ProjectID == activeProj.ID {
				projectWorkers[w.Name] = true
			}
		}
		var filtered []plan.Plan
		for _, p := range allPlans {
			if projectWorkers[p.WorkerName] {
				filtered = append(filtered, p)
			}
		}
		allPlans = filtered
	}

	if activeProj != nil {
		fmt.Fprintf(b, "%stomy monitor%s  [%s]  (every %ds, Ctrl+C to exit)\n", colorBold, colorReset, activeProj.Name, interval)
	} else {
		fmt.Fprintf(b, "%stomy monitor%s  (every %ds, Ctrl+C to exit)\n", colorBold, colorReset, interval)
	}
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	if len(allPlans) == 0 {
		fmt.Fprintf(b, "%sNo plans found.%s\n", colorGray, colorReset)
	} else {
		sortPlans(allPlans)
		for _, p := range allPlans {
			planTasks, _ := taskStore.ListByPlan(p.ID)
			renderPlan(b, p, planTasks)
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(b, "%sLast updated: %s%s\n", colorGray, time.Now().Format("15:04:05"), colorReset)
}

func cmdMonitor(args []string, plans *plan.Store, tasks *task.Store, workers *worker.Manager, activeProj *project.Project) {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	interval := fs.Int("interval", 2, "refresh interval in seconds")
	fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var b strings.Builder

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	// Render immediately
	renderMonitor(&b, plans, tasks, workers, activeProj, *interval)
	fmt.Print(b.String())

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nMonitor stopped.")
			return
		case <-ticker.C:
			renderMonitor(&b, plans, tasks, workers, activeProj, *interval)
			fmt.Print(b.String())
		}
	}
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
