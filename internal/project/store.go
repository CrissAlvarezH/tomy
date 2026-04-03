package project

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tomy/v1/internal/git"
	"github.com/tomy/v1/internal/state"
)

type Store struct {
	projectsPath string
	activePath   string
}

func NewStore(stateDir string) *Store {
	return &Store{
		projectsPath: filepath.Join(stateDir, "projects.json"),
		activePath:   filepath.Join(stateDir, "active_project.json"),
	}
}

func (s *Store) loadAll() ([]Project, error) {
	var projects []Project
	if err := state.ReadJSON(s.projectsPath, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (s *Store) saveAll(projects []Project) error {
	return state.WriteJSON(s.projectsPath, projects)
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Create adds a new project and sets it as active.
func (s *Store) Create(name string) (*Project, error) {
	projects, err := s.loadAll()
	if err != nil {
		return nil, err
	}

	// Check for duplicate names
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
	projects = append(projects, p)

	if err := s.saveAll(projects); err != nil {
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
	return s.loadAll()
}

// Get returns a project by ID or name.
func (s *Store) Get(idOrName string) (*Project, error) {
	projects, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.ID == idOrName || p.Name == idOrName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found", idOrName)
}

// SetActive sets the active project by ID.
func (s *Store) SetActive(projectID string) error {
	return state.WriteJSON(s.activePath, ActiveProject{ProjectID: projectID})
}

// GetActive returns the currently active project, or nil if none.
func (s *Store) GetActive() (*Project, error) {
	var active ActiveProject
	if err := state.ReadJSON(s.activePath, &active); err != nil {
		return nil, err
	}
	if active.ProjectID == "" {
		return nil, nil
	}
	p, err := s.Get(active.ProjectID)
	if err != nil {
		return nil, nil // active project was deleted, treat as no active
	}
	return p, nil
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

	projects, err := s.loadAll()
	if err != nil {
		return nil, err
	}

	for i := range projects {
		if projects[i].ID != projectID {
			continue
		}

		// Check for duplicate repo names
		for _, r := range projects[i].Repos {
			if r.Name == name {
				return nil, fmt.Errorf("repo %q already exists in project %q", name, projects[i].Name)
			}
		}

		repo := Repo{
			Name:         name,
			Path:         absPath,
			IsGitRepo:    git.IsGitRepo(absPath),
			SetupCommand: setupCmd,
		}
		projects[i].Repos = append(projects[i].Repos, repo)

		if err := s.saveAll(projects); err != nil {
			return nil, err
		}
		return &repo, nil
	}

	return nil, fmt.Errorf("project %q not found", projectID)
}

// RemoveRepo removes a repo from a project by name.
func (s *Store) RemoveRepo(projectID string, repoName string) error {
	projects, err := s.loadAll()
	if err != nil {
		return err
	}

	for i := range projects {
		if projects[i].ID != projectID {
			continue
		}

		found := false
		var remaining []Repo
		for _, r := range projects[i].Repos {
			if r.Name == repoName {
				found = true
			} else {
				remaining = append(remaining, r)
			}
		}
		if !found {
			return fmt.Errorf("repo %q not found in project %q", repoName, projects[i].Name)
		}
		projects[i].Repos = remaining
		return s.saveAll(projects)
	}

	return fmt.Errorf("project %q not found", projectID)
}

// SetRepoSetup updates the setup command for a repo.
func (s *Store) SetRepoSetup(projectID string, repoName string, setupCmd string) error {
	projects, err := s.loadAll()
	if err != nil {
		return err
	}

	for i := range projects {
		if projects[i].ID != projectID {
			continue
		}
		for j := range projects[i].Repos {
			if projects[i].Repos[j].Name == repoName {
				projects[i].Repos[j].SetupCommand = setupCmd
				return s.saveAll(projects)
			}
		}
		return fmt.Errorf("repo %q not found in project %q", repoName, projects[i].Name)
	}

	return fmt.Errorf("project %q not found", projectID)
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
