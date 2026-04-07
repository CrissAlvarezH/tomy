package nudge

import (
	"fmt"
	"strings"
	"time"

	"github.com/tomy/v1/internal/state"
)

const bucket = "nudges"

// Nudge represents a queued notification for an agent.
type Nudge struct {
	From      string    `json:"from"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Queue manages nudges in bbolt.
type Queue struct {
	db *state.DB
}

// NewQueue creates a queue backed by the given database.
func NewQueue(db *state.DB) *Queue {
	return &Queue{db: db}
}

// Enqueue writes a nudge for the given recipient.
func (q *Queue) Enqueue(from, to, content string) error {
	n := Nudge{
		From:      from,
		Content:   content,
		CreatedAt: time.Now(),
	}

	// Composite key: recipient/timestamp_nano — lexicographic = chronological
	key := fmt.Sprintf("%s/%d", to, time.Now().UnixNano())
	return q.db.Put(bucket, key, n)
}

// Drain reads and removes all queued nudges for a recipient.
// Returns them sorted by creation time (keys are already sorted).
func (q *Queue) Drain(name string) ([]Nudge, error) {
	return state.DrainByPrefix[Nudge](q.db, bucket, name+"/")
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
