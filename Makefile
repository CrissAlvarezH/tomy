BINARY    := orchestra
CMD_DIR   := ./cmd
BUILD_DIR := ./build
STATE_DIR := ./state
WORK_DIR  := ./workspaces

GO       := go
GOFLAGS  :=

.PHONY: build run clean reset install uninstall fmt vet test check agents tasks kill-all help

## ---- Build ----

build: ## Build the orchestra binary
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

run: build ## Build and run (pass ARGS, e.g. make run ARGS="agent list")
	$(BUILD_DIR)/$(BINARY) $(ARGS)

install: build ## Copy binary to ~/.local/bin
	mkdir -p $(HOME)/.local/bin
	cp $(BUILD_DIR)/$(BINARY) $(HOME)/.local/bin/$(BINARY)

uninstall: ## Remove binary from ~/.local/bin
	rm -f $(HOME)/.local/bin/$(BINARY)

## ---- Code quality ----

fmt: ## Format all Go files
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

test: ## Run all tests
	$(GO) test ./... -v

check: fmt vet test ## Run fmt + vet + test

## ---- State management ----

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

reset: ## Wipe all state (agents.json, tasks.json) — does NOT kill tmux sessions
	rm -f $(STATE_DIR)/agents.json $(STATE_DIR)/tasks.json
	@echo "State files cleared."

nuke: kill-all ## Kill all sessions + wipe state + remove workspaces
	rm -rf $(STATE_DIR) $(WORK_DIR)
	@echo "Everything wiped clean."

## ---- Agent shortcuts ----

agents: build ## List all agents
	$(BUILD_DIR)/$(BINARY) agent list

tasks: build ## List all tasks
	$(BUILD_DIR)/$(BINARY) task list

kill-all: ## Kill all orchestra tmux sessions
	@tmux list-sessions -F '#{session_name}' 2>/dev/null \
		| grep '^orch-' \
		| while read s; do \
			tmux kill-session -t "$$s" && echo "Killed $$s"; \
		done || echo "No orchestra sessions running."

## ---- Help ----

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) \
		| awk -F ':.*## ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
