package msg

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/tomy/v1/internal/state"
	bolt "go.etcd.io/bbolt"
)

const bucket = "inbox"

type Store struct {
	db *state.DB
}

func NewStore(db *state.DB) *Store {
	return &Store{db: db}
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Send adds a message to the recipient's inbox.
func (s *Store) Send(from, to, content string) (*Message, error) {
	m := Message{
		ID:        generateID(),
		From:      from,
		To:        to,
		Content:   content,
		Read:      false,
		CreatedAt: time.Now(),
	}

	key := to + "/" + m.ID
	if err := s.db.Put(bucket, key, m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Unread returns all unread messages for a recipient.
func (s *Store) Unread(name string) ([]Message, error) {
	messages, _, err := state.ListByPrefix[Message](s.db, bucket, name+"/")
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
	prefix := name + "/"
	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()
		pfx := []byte(prefix)
		for k, v := c.Seek(pfx); k != nil && len(k) >= len(pfx) && string(k[:len(pfx)]) == prefix; k, v = c.Next() {
			var m Message
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if !m.Read {
				m.Read = true
				data, err := json.Marshal(m)
				if err != nil {
					continue
				}
				if err := b.Put(k, data); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
