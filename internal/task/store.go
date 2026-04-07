package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/tomy/v1/internal/state"
	bolt "go.etcd.io/bbolt"
)

const bucket = "tasks"

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

// Create adds a new task and returns it.
func (s *Store) Create(title, description string) (*Task, error) {
	// Determine next order value
	tasks, err := state.List[Task](s.db, bucket)
	if err != nil {
		return nil, err
	}
	maxOrder := 0
	for _, t := range tasks {
		if t.Order > maxOrder {
			maxOrder = t.Order
		}
	}

	t := Task{
		ID:          generateID(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		Order:       maxOrder + 1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Put(bucket, t.ID, t); err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns all tasks.
func (s *Store) List() ([]Task, error) {
	tasks, err := state.List[Task](s.db, bucket)
	if err != nil {
		return nil, err
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})
	return tasks, nil
}

// Get returns a single task by ID.
func (s *Store) Get(id string) (*Task, error) {
	var t Task
	if err := s.db.Get(bucket, id, &t); err != nil {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return &t, nil
}

// ListByPlan returns all tasks belonging to a plan, sorted by order.
func (s *Store) ListByPlan(planID string) ([]Task, error) {
	tasks, err := state.List[Task](s.db, bucket)
	if err != nil {
		return nil, err
	}
	var result []Task
	for _, t := range tasks {
		if t.PlanID == planID {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Order < result[j].Order
	})
	return result, nil
}

// Delete removes a task by ID.
func (s *Store) Delete(id string) error {
	// Verify it exists
	if _, err := s.Get(id); err != nil {
		return err
	}
	return s.db.Delete(bucket, id)
}

// Move repositions a task within a plan, placing it before the target task.
func (s *Store) Move(id, beforeID string) error {
	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))

		// Load the two tasks
		var moving, target Task
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("task %q not found", id)
		}
		if err := json.Unmarshal(v, &moving); err != nil {
			return err
		}

		v = b.Get([]byte(beforeID))
		if v == nil {
			return fmt.Errorf("target task %q not found", beforeID)
		}
		if err := json.Unmarshal(v, &target); err != nil {
			return err
		}

		// Load all tasks, sort by order
		var tasks []Task
		b.ForEach(func(k, v []byte) error {
			var t Task
			json.Unmarshal(v, &t)
			tasks = append(tasks, t)
			return nil
		})
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Order < tasks[j].Order
		})

		// Remove moving task from list, insert before target
		var reordered []Task
		for _, t := range tasks {
			if t.ID == id {
				continue
			}
			if t.ID == beforeID {
				reordered = append(reordered, moving)
			}
			reordered = append(reordered, t)
		}

		// Reassign order values
		for i := range reordered {
			reordered[i].Order = i + 1
			reordered[i].UpdatedAt = time.Now()
			data, err := json.Marshal(reordered[i])
			if err != nil {
				return err
			}
			if err := b.Put([]byte(reordered[i].ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// Update modifies a task in-place using the provided function.
func (s *Store) Update(id string, fn func(*Task)) error {
	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("task %q not found", id)
		}
		var t Task
		if err := json.Unmarshal(v, &t); err != nil {
			return err
		}
		fn(&t)
		t.UpdatedAt = time.Now()
		data, err := json.Marshal(t)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}
