package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/tomy/v1/internal/project"
)

const setupTimeout = 60 * time.Second

// RunSetupCommand executes a repo's setup command in the worktree directory.
// Returns nil if no setup command is configured.
func RunSetupCommand(repo project.Repo, wtPath, workspaceDir, workerName string, workerIndex int) error {
	if repo.SetupCommand == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", repo.SetupCommand)
	cmd.Dir = wtPath
	cmd.Env = append(os.Environ(),
		"TOMY_WORKTREE_PATH="+wtPath,
		"TOMY_REPO_PATH="+repo.Path,
		"TOMY_REPO_NAME="+repo.Name,
		"TOMY_WORKER_NAME="+workerName,
		"TOMY_WORKER_INDEX="+strconv.Itoa(workerIndex),
		"TOMY_WORKSPACE_DIR="+workspaceDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\noutput: %s", err, string(output))
	}
	return nil
}
