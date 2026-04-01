package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/orchestra/v1/internal/project"
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
		"ORCH_WORKTREE_PATH="+wtPath,
		"ORCH_REPO_PATH="+repo.Path,
		"ORCH_REPO_NAME="+repo.Name,
		"ORCH_WORKER_NAME="+workerName,
		"ORCH_WORKER_INDEX="+strconv.Itoa(workerIndex),
		"ORCH_WORKSPACE_DIR="+workspaceDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\noutput: %s", err, string(output))
	}
	return nil
}
