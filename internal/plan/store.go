package plan

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tomy/v1/internal/state"
	bolt "go.etcd.io/bbolt"
)

const (
	bucket        = "plans"
	contentBucket = "plan_content"
)

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

// Create adds a new plan and returns it.
func (s *Store) Create(name, projectID string) (*Plan, error) {
	id := generateID()
	p := Plan{
		ID:        id,
		Name:      name,
		ProjectID: projectID,
		Status:    StatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Put(bucket, id, p); err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns all plans.
func (s *Store) List() ([]Plan, error) {
	return state.List[Plan](s.db, bucket)
}

// Get returns a single plan by ID.
func (s *Store) Get(id string) (*Plan, error) {
	var p Plan
	if err := s.db.Get(bucket, id, &p); err != nil {
		return nil, fmt.Errorf("plan %q not found", id)
	}
	return &p, nil
}

// Update modifies a plan in-place using the provided function.
func (s *Store) Update(id string, fn func(*Plan)) error {
	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("plan %q not found", id)
		}
		var p Plan
		if err := json.Unmarshal(v, &p); err != nil {
			return err
		}
		fn(&p)
		p.UpdatedAt = time.Now()
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// GetContent returns the markdown content for a plan.
func (s *Store) GetContent(id string) ([]byte, error) {
	data, err := s.db.GetRaw(contentBucket, id)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SetContent stores the markdown content for a plan.
func (s *Store) SetContent(id string, content []byte) error {
	return s.db.PutRaw(contentBucket, id, content)
}
