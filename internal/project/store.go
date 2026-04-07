package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tomy/v1/internal/git"
	"github.com/tomy/v1/internal/state"
	bolt "go.etcd.io/bbolt"
)

const (
	bucket     = "projects"
	metaBucket = "meta"
	activeKey  = "active_project"
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

// Create adds a new project and sets it as active.
func (s *Store) Create(name string) (*Project, error) {
	// Check for duplicate names
	projects, err := state.List[Project](s.db, bucket)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == name {
			return nil, fmt.Errorf("project %q already exists", name)
		}
	}

	p := Project{
		ID:        generateID(),
		Name:      name,
		Repos:     []Repo{},
		Status:    StatusActive,
		CreatedAt: time.Now(),
	}

	if err := s.db.Put(bucket, p.ID, p); err != nil {
		return nil, err
	}

	// Auto-set as active
	if err := s.SetActive(p.ID); err != nil {
		return nil, err
	}

	return &p, nil
}

// List returns all projects.
func (s *Store) List() ([]Project, error) {
	return state.List[Project](s.db, bucket)
}

// Get returns a project by ID or name.
func (s *Store) Get(idOrName string) (*Project, error) {
	// Try by ID first
	var p Project
	if err := s.db.Get(bucket, idOrName, &p); err == nil {
		return &p, nil
	}

	// Fall back to name search
	projects, err := state.List[Project](s.db, bucket)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == idOrName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found", idOrName)
}

// Remove deletes a project by ID or name.
func (s *Store) Remove(idOrName string) error {
	p, err := s.Get(idOrName)
	if err != nil {
		return err
	}

	if err := s.db.Delete(bucket, p.ID); err != nil {
		return err
	}

	// Clear active project if it was the one removed
	active, _ := s.GetActive()
	if active != nil && active.ID == p.ID {
		return s.SetActive("")
	}
	return nil
}

// SetActive sets the active project by ID.
func (s *Store) SetActive(projectID string) error {
	return s.db.Put(metaBucket, activeKey, projectID)
}

// GetActive returns the currently active project, or nil if none.
func (s *Store) GetActive() (*Project, error) {
	var activeID string
	if err := s.db.Get(metaBucket, activeKey, &activeID); err != nil {
		return nil, nil
	}
	if activeID == "" {
		return nil, nil
	}
	var p Project
	if err := s.db.Get(bucket, activeID, &p); err != nil {
		return nil, nil // active project was deleted, treat as no active
	}
	return &p, nil
}

// AddRepo adds a repo to a project.
func (s *Store) AddRepo(projectID string, name string, path string, setupCmd string) (*Repo, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Verify directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path %q does not exist", absPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", absPath)
	}

	return s.updateProject(projectID, func(p *Project) (*Repo, error) {
		// Check for duplicate repo names
		for _, r := range p.Repos {
			if r.Name == name {
				return nil, fmt.Errorf("repo %q already exists in project %q", name, p.Name)
			}
		}

		repo := Repo{
			Name:         name,
			Path:         absPath,
			IsGitRepo:    git.IsGitRepo(absPath),
			SetupCommand: setupCmd,
		}
		p.Repos = append(p.Repos, repo)
		return &repo, nil
	})
}

// RemoveRepo removes a repo from a project by name.
func (s *Store) RemoveRepo(projectID string, repoName string) error {
	_, err := s.updateProject(projectID, func(p *Project) (*Repo, error) {
		found := false
		var remaining []Repo
		for _, r := range p.Repos {
			if r.Name == repoName {
				found = true
			} else {
				remaining = append(remaining, r)
			}
		}
		if !found {
			return nil, fmt.Errorf("repo %q not found in project %q", repoName, p.Name)
		}
		p.Repos = remaining
		return nil, nil
	})
	return err
}

// SetRepoSetup updates the setup command for a repo.
func (s *Store) SetRepoSetup(projectID string, repoName string, setupCmd string) error {
	_, err := s.updateProject(projectID, func(p *Project) (*Repo, error) {
		for j := range p.Repos {
			if p.Repos[j].Name == repoName {
				p.Repos[j].SetupCommand = setupCmd
				return nil, nil
			}
		}
		return nil, fmt.Errorf("repo %q not found in project %q", repoName, p.Name)
	})
	return err
}

// GetRepo returns a repo by name from a project.
func (s *Store) GetRepo(proj *Project, repoName string) (*Repo, error) {
	for _, r := range proj.Repos {
		if r.Name == repoName {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("repo %q not found in project %q", repoName, proj.Name)
}

// updateProject is a helper that does a read-modify-write on a project in a single transaction.
func (s *Store) updateProject(projectID string, fn func(*Project) (*Repo, error)) (*Repo, error) {
	var result *Repo
	err := s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(projectID))
		if v == nil {
			return fmt.Errorf("project %q not found", projectID)
		}
		var p Project
		if err := json.Unmarshal(v, &p); err != nil {
			return err
		}
		r, err := fn(&p)
		if err != nil {
			return err
		}
		result = r
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return b.Put([]byte(projectID), data)
	})
	return result, err
}
