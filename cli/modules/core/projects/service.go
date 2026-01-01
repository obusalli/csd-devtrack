package projects

import (
	"fmt"
	"path/filepath"
)

// Service provides project management operations
type Service struct {
	repo     *Repository
	detector *Detector
}

// NewService creates a new project service
func NewService(repo *Repository) *Service {
	return &Service{
		repo:     repo,
		detector: NewDetector(),
	}
}

// Load loads projects from config
func (s *Service) Load() error {
	return s.repo.Load()
}

// Save saves projects to config
func (s *Service) Save() error {
	return s.repo.Save()
}

// ListProjects returns all projects
func (s *Service) ListProjects() []*Project {
	return s.repo.GetAll()
}

// GetProject returns a project by ID
func (s *Service) GetProject(id string) (*Project, error) {
	return s.repo.GetByID(id)
}

// AddProject adds a new project from a path (auto-detects structure)
func (s *Service) AddProject(path string) (*Project, error) {
	// Detect project structure
	project, err := s.detector.DetectProject(path)
	if err != nil {
		return nil, fmt.Errorf("failed to detect project: %w", err)
	}

	// Check if project already exists
	if s.repo.Exists(project.ID) {
		// Update existing project
		if err := s.repo.Update(project); err != nil {
			return nil, fmt.Errorf("failed to update project: %w", err)
		}
	} else {
		// Add new project
		if err := s.repo.Add(project); err != nil {
			return nil, fmt.Errorf("failed to add project: %w", err)
		}
	}

	// Save config
	if err := s.repo.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return project, nil
}

// AddProjectWithName adds a project with a custom name
func (s *Service) AddProjectWithName(path, name string) (*Project, error) {
	project, err := s.AddProject(path)
	if err != nil {
		return nil, err
	}

	project.Name = name
	if err := s.repo.Update(project); err != nil {
		return nil, fmt.Errorf("failed to update project name: %w", err)
	}

	if err := s.repo.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return project, nil
}

// RemoveProject removes a project by ID
func (s *Service) RemoveProject(id string) error {
	if err := s.repo.Remove(id); err != nil {
		return err
	}

	return s.repo.Save()
}

// RefreshProject re-detects a project's structure
func (s *Service) RefreshProject(id string) (*Project, error) {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Re-detect from the same path
	updated, err := s.detector.DetectProject(existing.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh project: %w", err)
	}

	// Preserve ID and name
	updated.ID = existing.ID
	updated.Name = existing.Name
	updated.Self = existing.Self

	if err := s.repo.Update(updated); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	if err := s.repo.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return updated, nil
}

// AddSelfProject adds csd-devtrack itself as a managed project
func (s *Service) AddSelfProject(basePath string) (*Project, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	project := &Project{
		ID:   "csd-devtrack",
		Name: "CSD DevTrack (self)",
		Path: absPath,
		Type: ProjectTypeBackendOnly,
		Self: true,
		Components: map[ComponentType]*Component{
			ComponentCLI: {
				Type:       ComponentCLI,
				Path:       "cli/",
				EntryPoint: "csd-devtrack.go",
				Binary:     "csd-devtrack",
				Enabled:    true,
			},
		},
	}

	if err := s.repo.AddOrUpdate(project); err != nil {
		return nil, fmt.Errorf("failed to add self project: %w", err)
	}

	if err := s.repo.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return project, nil
}

// GetProjectSummaries returns lightweight summaries of all projects
func (s *Service) GetProjectSummaries() []*ProjectSummary {
	projects := s.repo.GetAll()
	summaries := make([]*ProjectSummary, len(projects))
	for i, p := range projects {
		summaries[i] = p.ToSummary()
	}
	return summaries
}

// ValidateProject validates a project configuration
func (s *Service) ValidateProject(project *Project) []string {
	var errors []string

	if project.ID == "" {
		errors = append(errors, "project ID is required")
	}

	if project.Name == "" {
		errors = append(errors, "project name is required")
	}

	if project.Path == "" {
		errors = append(errors, "project path is required")
	}

	if len(project.Components) == 0 {
		errors = append(errors, "project has no components")
	}

	for ct, comp := range project.Components {
		if comp.Enabled && comp.Path == "" && ct != ComponentCLI {
			errors = append(errors, fmt.Sprintf("component %s has no path", ct))
		}
	}

	return errors
}
