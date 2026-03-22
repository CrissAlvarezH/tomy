package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsGitRepo checks if a directory is a git repository.
func IsGitRepo(path string) bool {
	info, err := os.Stat(path + "/.git")
	if err != nil {
		return false
	}
	// .git can be a directory (normal repo) or a file (worktree)
	return info.IsDir() || info.Mode().IsRegular()
}

// WorktreeAdd creates a new git worktree with a new branch.
func WorktreeAdd(repoPath, worktreePath, branchName string) error {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branchName, worktreePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// WorktreeRemove removes a git worktree.
func WorktreeRemove(repoPath, worktreePath string) error {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "remove", worktreePath, "--force")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}
