package plan

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/orchestra/v1/internal/state"
)

type Store struct {
	path     string
	plansDir string
}

func NewStore(stateDir, plansDir string) *Store {
	return &Store{
		path:     filepath.Join(stateDir, "plans.json"),
		plansDir: plansDir,
	}
}

func (s *Store) PlansDir() string {
	return s.plansDir
}

func (s *Store) loadAll() ([]Plan, error) {
	var plans []Plan
	if err := state.ReadJSON(s.path, &plans); err != nil {
		return nil, err
	}
	return plans, nil
}

func (s *Store) saveAll(plans []Plan) error {
	return state.WriteJSON(s.path, plans)
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Create adds a new plan and returns it.
func (s *Store) Create(name string) (*Plan, error) {
	plans, err := s.loadAll()
	if err != nil {
		return nil, err
	}

	id := generateID()
	contentFile := filepath.Join(s.plansDir, id+".md")

	p := Plan{
		ID:          id,
		Name:        name,
		ContentFile: contentFile,
		Status:      StatusDraft,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	plans = append(plans, p)

	if err := s.saveAll(plans); err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns all plans.
func (s *Store) List() ([]Plan, error) {
	return s.loadAll()
}

// Get returns a single plan by ID.
func (s *Store) Get(id string) (*Plan, error) {
	plans, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	for _, p := range plans {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("plan %q not found", id)
}

// Update modifies a plan in-place using the provided function.
func (s *Store) Update(id string, fn func(*Plan)) error {
	plans, err := s.loadAll()
	if err != nil {
		return err
	}
	for i := range plans {
		if plans[i].ID == id {
			fn(&plans[i])
			plans[i].UpdatedAt = time.Now()
			return s.saveAll(plans)
		}
	}
	return fmt.Errorf("plan %q not found", id)
}
