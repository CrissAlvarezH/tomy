package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
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

// CapturePane captures the visible content of a tmux pane.
func CapturePane(name string, lines int) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	start := fmt.Sprintf("-%d", lines)
	return run("capture-pane", "-t", name, "-p", "-S", start)
}

// AcceptStartupDialogs polls the tmux pane for Claude's workspace trust dialog
// and bypass permissions warning, accepting them by pressing Enter.
// This is the same approach gastown uses (poll + send Enter).
func AcceptStartupDialogs(name string) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		content, err := CapturePane(name, 30)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Detect trust dialog
		if strings.Contains(content, "Quick safety check") ||
			strings.Contains(content, "trust this folder") {
			// "Yes, I trust this folder" is pre-selected — just press Enter
			run("send-keys", "-t", name, "Enter")
			time.Sleep(500 * time.Millisecond)
			// Continue polling — bypass permissions warning may follow
			continue
		}

		// Detect bypass permissions warning
		if strings.Contains(content, "Bypass Permissions mode") {
			run("send-keys", "-t", name, "Enter")
			time.Sleep(500 * time.Millisecond)
			return nil
		}

		// If we see the Claude prompt, startup is done
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == ">" || strings.HasSuffix(trimmed, "❯") {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil // timeout — proceed anyway
}

// AttachSession switches to an existing session. If already inside tmux,
// uses switch-client; otherwise uses attach-session.
func AttachSession(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	action := "attach-session"
	if os.Getenv("TMUX") != "" {
		action = "switch-client"
	}

	cmd := exec.Command("tmux", action, "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
