package main

import "fmt"

func cmdCompletion(args []string) {
	if len(args) < 1 {
		fatal("usage: orchestra completion <zsh|bash>")
	}
	switch args[0] {
	case "zsh":
		fmt.Print(zshCompletion)
	case "bash":
		fmt.Print(bashCompletion)
	default:
		fatal("unsupported shell: " + args[0] + " (supported: zsh, bash)")
	}
}

const zshCompletion = `#compdef orchestra

_orchestra_workers() {
  local -a workers
  workers=(${(f)"$(orchestra worker list 2>/dev/null | tail -n +2 | awk '{print $1}')"})
  _describe 'worker' workers
}

_orchestra_projects() {
  local -a projects
  projects=(${(f)"$(orchestra project list 2>/dev/null | tail -n +2 | awk '{print $1}')"})
  _describe 'project' projects
}

_orchestra_plans() {
  local -a plans
  plans=(${(f)"$(orchestra plan list 2>/dev/null | tail -n +2 | awk '{print $1}')"})
  _describe 'plan' plans
}

_orchestra_tasks() {
  local -a tasks
  tasks=(${(f)"$(orchestra task list 2>/dev/null | tail -n +2 | awk '{print $1}')"})
  _describe 'task' tasks
}

_orchestra_repos() {
  local -a repos
  repos=(${(f)"$(orchestra repo list 2>/dev/null | tail -n +2 | awk '{print $1}')"})
  _describe 'repo' repos
}

_orchestra() {
  local -a commands
  commands=(
    'project:Manage projects'
    'repo:Manage repos'
    'plan:Manage plans'
    'planner:Manage planner session'
    'worker:Manage workers'
    'msg:Send and receive messages'
    'task:Manage tasks'
    'done:Mark worker and all plan tasks as done'
    'run:Create plan + task + spawn + assign (all-in-one)'
    'completion:Output shell completion script'
    'help:Show usage'
    'version:Show version'
  )

  _arguments -C \
    '1: :->command' \
    '*:: :->args'

  case $state in
    command)
      _describe 'command' commands
      ;;
    args)
      case $words[1] in
        project)
          local -a subcmds
          subcmds=('create:Create a new project' 'list:List all projects' 'set:Set active project' 'status:Show active project details')
          _arguments -C '1: :->subcmd' '*:: :->subargs'
          case $state in
            subcmd) _describe 'subcommand' subcmds ;;
            subargs)
              case $words[1] in
                set) _orchestra_projects ;;
              esac
              ;;
          esac
          ;;
        repo)
          local -a subcmds
          subcmds=('add:Add a repo to active project' 'list:List repos in active project' 'remove:Remove a repo' 'setup:Set/show post-worktree setup command')
          _arguments -C '1: :->subcmd' '*:: :->subargs'
          case $state in
            subcmd) _describe 'subcommand' subcmds ;;
            subargs)
              case $words[1] in
                remove|setup) _orchestra_repos ;;
                add) _files -/ ;;
              esac
              ;;
          esac
          ;;
        plan)
          local -a subcmds
          subcmds=('create:Create a plan' 'list:List all plans with progress' 'show:Show plan tasks' 'assign:Assign plan to a worker')
          _arguments -C '1: :->subcmd' '*:: :->subargs'
          case $state in
            subcmd) _describe 'subcommand' subcmds ;;
            subargs)
              case $words[1] in
                show) _orchestra_plans ;;
                assign)
                  _arguments '1: :_orchestra_plans' '2: :_orchestra_workers'
                  ;;
              esac
              ;;
          esac
          ;;
        planner)
          local -a subcmds
          subcmds=('start:Spawn planner session' 'stop:Kill the planner session' 'attach:Attach to planner session')
          _describe 'subcommand' subcmds
          ;;
        worker)
          local -a subcmds
          subcmds=('spawn:Spawn a worker' 'list:List all workers' 'kill:Kill a worker' 'attach:Attach to worker session' 'peek:See what a worker is doing')
          _arguments -C '1: :->subcmd' '*:: :->subargs'
          case $state in
            subcmd) _describe 'subcommand' subcmds ;;
            subargs)
              case $words[1] in
                kill|attach|peek) _orchestra_workers ;;
              esac
              ;;
          esac
          ;;
        msg)
          local -a subcmds
          subcmds=('send:Send a message' 'inbox:Read unread messages')
          _arguments -C '1: :->subcmd' '*:: :->subargs'
          case $state in
            subcmd) _describe 'subcommand' subcmds ;;
            subargs)
              case $words[1] in
                send|inbox) _orchestra_workers ;;
              esac
              ;;
          esac
          ;;
        task)
          local -a subcmds
          subcmds=('create:Create a task' 'list:List all tasks' 'status:Show task details' 'done:Mark a task as done' 'block:Mark a task as blocked' 'unblock:Unblock a task')
          _arguments -C '1: :->subcmd' '*:: :->subargs'
          case $state in
            subcmd) _describe 'subcommand' subcmds ;;
            subargs)
              case $words[1] in
                status|done|block|unblock) _orchestra_tasks ;;
              esac
              ;;
          esac
          ;;
        done)
          _orchestra_workers
          ;;
        completion)
          _describe 'shell' '(zsh bash)'
          ;;
      esac
      ;;
  esac
}

compdef _orchestra orchestra
`

const bashCompletion = `_orchestra_workers() {
  orchestra worker list 2>/dev/null | tail -n +2 | awk '{print $1}'
}

_orchestra_projects() {
  orchestra project list 2>/dev/null | tail -n +2 | awk '{print $1}'
}

_orchestra_plans() {
  orchestra plan list 2>/dev/null | tail -n +2 | awk '{print $1}'
}

_orchestra_tasks() {
  orchestra task list 2>/dev/null | tail -n +2 | awk '{print $1}'
}

_orchestra_repos() {
  orchestra repo list 2>/dev/null | tail -n +2 | awk '{print $1}'
}

_orchestra() {
  local cur prev words cword
  _init_completion || return

  local commands="project repo plan planner worker msg task done run completion help version"

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    return
  fi

  local cmd="${words[1]}"
  local subcmd="${words[2]}"

  case "$cmd" in
    project)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "create list set status" -- "$cur"))
      elif [[ $cword -eq 3 ]]; then
        case "$subcmd" in
          set) COMPREPLY=($(compgen -W "$(_orchestra_projects)" -- "$cur")) ;;
        esac
      fi
      ;;
    repo)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "add list remove setup" -- "$cur"))
      elif [[ $cword -eq 3 ]]; then
        case "$subcmd" in
          remove|setup) COMPREPLY=($(compgen -W "$(_orchestra_repos)" -- "$cur")) ;;
          add) COMPREPLY=($(compgen -d -- "$cur")) ;;
        esac
      fi
      ;;
    plan)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "create list show assign" -- "$cur"))
      elif [[ $cword -eq 3 ]]; then
        case "$subcmd" in
          show|assign) COMPREPLY=($(compgen -W "$(_orchestra_plans)" -- "$cur")) ;;
        esac
      elif [[ $cword -eq 4 ]]; then
        case "$subcmd" in
          assign) COMPREPLY=($(compgen -W "$(_orchestra_workers)" -- "$cur")) ;;
        esac
      fi
      ;;
    planner)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "start stop attach" -- "$cur"))
      fi
      ;;
    worker)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "spawn list kill attach peek" -- "$cur"))
      elif [[ $cword -eq 3 ]]; then
        case "$subcmd" in
          kill|attach|peek) COMPREPLY=($(compgen -W "$(_orchestra_workers)" -- "$cur")) ;;
        esac
      fi
      ;;
    msg)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "send inbox" -- "$cur"))
      elif [[ $cword -eq 3 ]]; then
        case "$subcmd" in
          send|inbox) COMPREPLY=($(compgen -W "$(_orchestra_workers)" -- "$cur")) ;;
        esac
      fi
      ;;
    task)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "create list status done block unblock" -- "$cur"))
      elif [[ $cword -eq 3 ]]; then
        case "$subcmd" in
          status|done|block|unblock) COMPREPLY=($(compgen -W "$(_orchestra_tasks)" -- "$cur")) ;;
        esac
      fi
      ;;
    done)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "$(_orchestra_workers)" -- "$cur"))
      fi
      ;;
    completion)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "zsh bash" -- "$cur"))
      fi
      ;;
  esac
}

complete -F _orchestra orchestra
`
