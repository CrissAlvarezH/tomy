package task

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/orchestra/v1/internal/state"
)

type Store struct {
	path string
}

func NewStore(stateDir string) *Store {
	return &Store{path: filepath.Join(stateDir, "tasks.json")}
}

func (s *Store) loadAll() ([]Task, error) {
	var tasks []Task
	if err := state.ReadJSON(s.path, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) saveAll(tasks []Task) error {
	return state.WriteJSON(s.path, tasks)
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Create adds a new task and returns it.
func (s *Store) Create(title, description string) (*Task, error) {
	tasks, err := s.loadAll()
	if err != nil {
		return nil, err
	}

	t := Task{
		ID:          generateID(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	tasks = append(tasks, t)

	if err := s.saveAll(tasks); err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns all tasks.
func (s *Store) List() ([]Task, error) {
	return s.loadAll()
}

// Get returns a single task by ID.
func (s *Store) Get(id string) (*Task, error) {
	tasks, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("task %q not found", id)
}

// Update modifies a task in-place using the provided function.
func (s *Store) Update(id string, fn func(*Task)) error {
	tasks, err := s.loadAll()
	if err != nil {
		return err
	}
	for i := range tasks {
		if tasks[i].ID == id {
			fn(&tasks[i])
			tasks[i].UpdatedAt = time.Now()
			return s.saveAll(tasks)
		}
	}
	return fmt.Errorf("task %q not found", id)
}
