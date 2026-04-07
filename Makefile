BINARY    := tomy
CMD_DIR   := ./cmd
BUILD_DIR := ./build
TOMY_HOME := $(HOME)/.tomy
STATE_DIR := $(TOMY_HOME)/state
WORK_DIR  := $(TOMY_HOME)/workspaces

GO       := go
GOFLAGS  :=

.PHONY: build run clean reset install install-completion uninstall fmt vet test check workers tasks kill-all help

## ---- Build ----

build: ## Build the tomy binary
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

run: build ## Build and run (pass ARGS, e.g. make run ARGS="worker list")
	$(BUILD_DIR)/$(BINARY) $(ARGS)

install: build install-completion ## Copy binary to ~/.local/bin + install completions
	@mkdir -p $(HOME)/.local/bin
	@$(GO) build $(GOFLAGS) -o $(HOME)/.local/bin/$(BINARY) $(CMD_DIR)
	@echo ""
	@echo "  \033[32m✓\033[0m Binary installed to \033[1m~/.local/bin/$(BINARY)\033[0m"
	@echo ""
	@echo "  Make sure \033[1m~/.local/bin\033[0m is in your PATH."
	@echo "  Restart your shell or run:"
	@echo ""
	@echo "    \033[36meval \"\$$(tomy completion zsh)\"\033[0m   (zsh)"
	@echo "    \033[36meval \"\$$(tomy completion bash)\"\033[0m  (bash)"
	@echo ""

install-completion: build ## Install shell completion for zsh and bash
	@mkdir -p $(HOME)/.local/share/zsh/site-functions
	@$(BUILD_DIR)/$(BINARY) completion zsh > $(HOME)/.local/share/zsh/site-functions/_$(BINARY)
	@echo "  \033[32m✓\033[0m zsh  \033[2m→ ~/.local/share/zsh/site-functions/_$(BINARY)\033[0m"
	@mkdir -p $(HOME)/.local/share/bash-completion/completions
	@$(BUILD_DIR)/$(BINARY) completion bash > $(HOME)/.local/share/bash-completion/completions/$(BINARY)
	@echo "  \033[32m✓\033[0m bash \033[2m→ ~/.local/share/bash-completion/completions/$(BINARY)\033[0m"

uninstall: ## Remove binary and completions
	@rm -f $(HOME)/.local/bin/$(BINARY)
	@echo "  \033[31m✗\033[0m Removed \033[2m~/.local/bin/$(BINARY)\033[0m"
	@rm -f $(HOME)/.local/share/zsh/site-functions/_$(BINARY)
	@echo "  \033[31m✗\033[0m Removed \033[2m~/.local/share/zsh/site-functions/_$(BINARY)\033[0m"
	@rm -f $(HOME)/.local/share/bash-completion/completions/$(BINARY)
	@echo "  \033[31m✗\033[0m Removed \033[2m~/.local/share/bash-completion/completions/$(BINARY)\033[0m"

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

reset: ## Wipe state database (~/.tomy/state/) — does NOT kill tmux sessions
	rm -f $(STATE_DIR)/tomy.db
	@echo "State database cleared."

nuke: kill-all ## Kill all sessions + wipe ~/.tomy entirely
	rm -rf $(TOMY_HOME)
	@echo "Everything wiped clean."

## ---- Worker shortcuts ----

workers: build ## List all workers
	$(BUILD_DIR)/$(BINARY) worker list

tasks: build ## List all tasks
	$(BUILD_DIR)/$(BINARY) task list

kill-all: ## Kill all tomy tmux sessions
	@tmux list-sessions -F '#{session_name}' 2>/dev/null \
		| grep '^tomy-' \
		| while read s; do \
			tmux kill-session -t "$$s" && echo "Killed $$s"; \
		done || echo "No tomy sessions running."

## ---- Help ----

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) \
		| awk -F ':.*## ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
