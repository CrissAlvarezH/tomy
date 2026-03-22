package tmux

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid session name %q: must be alphanumeric, underscore, or dash", name)
	}
	return nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux %s: %s", strings.Join(args, " "), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// NewSession creates a new detached tmux session.
func NewSession(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := run("new-session", "-d", "-s", name)
	return err
}

// KillSession destroys a tmux session.
func KillSession(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := run("kill-session", "-t", name)
	return err
}

// HasSession checks if a tmux session exists.
func HasSession(name string) bool {
	if err := validateName(name); err != nil {
		return false
	}
	_, err := run("has-session", "-t", name)
	return err == nil
}

// SendKeys sends keystrokes to a tmux session.
func SendKeys(name string, keys string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := run("send-keys", "-t", name, keys, "Enter")
	return err
}

// ListSessions returns all tmux session names.
func ListSessions() ([]string, error) {
	out, err := run("list-sessions", "-F", "#{session_name}")
	if err != nil {
		// tmux returns error if no server running (no sessions)
		if strings.Contains(err.Error(), "no server running") || strings.Contains(err.Error(), "no sessions") {
			return nil, nil
		}
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// AttachSession attaches to an existing session (replaces current process).
func AttachSession(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	cmd := exec.Command("tmux", "attach-session", "-t", name)
	cmd.Stdin = nil // inherit from parent handled by syscall
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
