package nudge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Nudge represents a queued notification for an agent.
type Nudge struct {
	From      string    `json:"from"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Queue manages a filesystem-based nudge queue.
// Each recipient has a directory; each nudge is a timestamped JSON file.
type Queue struct {
	baseDir string
}

// NewQueue creates a queue rooted at baseDir.
func NewQueue(baseDir string) *Queue {
	return &Queue{baseDir: baseDir}
}

func (q *Queue) recipientDir(name string) string {
	return filepath.Join(q.baseDir, name)
}

// Enqueue writes a nudge to the recipient's queue directory.
func (q *Queue) Enqueue(from, to, content string) error {
	dir := q.recipientDir(to)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create nudge dir: %w", err)
	}

	n := Nudge{
		From:      from,
		Content:   content,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(n)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%d.json", time.Now().UnixNano())
	return os.WriteFile(filepath.Join(dir, filename), data, 0644)
}

// Drain reads and removes all queued nudges for a recipient.
// Returns them sorted by creation time.
func (q *Queue) Drain(name string) ([]Nudge, error) {
	dir := q.recipientDir(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var nudges []Nudge
	var files []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var n Nudge
		if err := json.Unmarshal(data, &n); err != nil {
			continue
		}
		nudges = append(nudges, n)
		files = append(files, path)
	}

	// Sort by creation time
	sort.Slice(nudges, func(i, j int) bool {
		return nudges[i].CreatedAt.Before(nudges[j].CreatedAt)
	})

	// Remove consumed files
	for _, f := range files {
		os.Remove(f)
	}

	return nudges, nil
}

// FormatForInjection formats drained nudges as a system-reminder block
// that Claude Code hooks can inject into the conversation context.
func FormatForInjection(nudges []Nudge) string {
	if len(nudges) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("You have pending messages:\n\n")
	for _, n := range nudges {
		ago := time.Since(n.CreatedAt).Truncate(time.Second)
		b.WriteString(fmt.Sprintf("From %s (%s ago):\n  %s\n\n", n.From, ago, n.Content))
	}
	b.WriteString("Review and respond to these messages using: tomy msg inbox\n")
	b.WriteString("</system-reminder>")
	return b.String()
}
