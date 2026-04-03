package msg

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"time"

	"github.com/tomy/v1/internal/state"
)

type Store struct {
	inboxDir string
}

func NewStore(inboxDir string) *Store {
	return &Store{inboxDir: inboxDir}
}

func (s *Store) inboxPath(name string) string {
	return filepath.Join(s.inboxDir, name+".json")
}

func (s *Store) loadInbox(name string) ([]Message, error) {
	var messages []Message
	if err := state.ReadJSON(s.inboxPath(name), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *Store) saveInbox(name string, messages []Message) error {
	return state.WriteJSON(s.inboxPath(name), messages)
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Send adds a message to the recipient's inbox.
func (s *Store) Send(from, to, content string) (*Message, error) {
	messages, err := s.loadInbox(to)
	if err != nil {
		return nil, err
	}

	m := Message{
		ID:        generateID(),
		From:      from,
		To:        to,
		Content:   content,
		Read:      false,
		CreatedAt: time.Now(),
	}

	messages = append(messages, m)
	if err := s.saveInbox(to, messages); err != nil {
		return nil, err
	}
	return &m, nil
}

// Unread returns all unread messages for a recipient.
func (s *Store) Unread(name string) ([]Message, error) {
	messages, err := s.loadInbox(name)
	if err != nil {
		return nil, err
	}

	var unread []Message
	for _, m := range messages {
		if !m.Read {
			unread = append(unread, m)
		}
	}
	return unread, nil
}

// MarkAllRead marks all messages as read for a recipient.
func (s *Store) MarkAllRead(name string) error {
	messages, err := s.loadInbox(name)
	if err != nil {
		return err
	}

	for i := range messages {
		messages[i].Read = true
	}
	return s.saveInbox(name, messages)
}
