package git

import (
	"fmt"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/projects"
)

// statusCacheEntry holds cached git status with expiry
type statusCacheEntry struct {
	status    *Status
	timestamp time.Time
}

// Service provides git operations for projects
type Service struct {
	projectService *projects.Service
	repos          map[string]*Repository

	// Status cache with TTL
	statusCache   map[string]*statusCacheEntry
	statusCacheMu sync.RWMutex
	statusTTL     time.Duration // How long status cache is valid
}

// NewService creates a new git service
func NewService(projectService *projects.Service) *Service {
	return &Service{
		projectService: projectService,
		repos:          make(map[string]*Repository),
		statusCache:    make(map[string]*statusCacheEntry),
		statusTTL:      2 * time.Second, // Cache status for 2 seconds
	}
}

// GetRepository returns the git repository for a project
func (s *Service) GetRepository(projectID string) (*Repository, error) {
	// Check cache
	if repo, ok := s.repos[projectID]; ok {
		return repo, nil
	}

	// Get project
	project, err := s.projectService.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	// Open repository
	repo, err := OpenRepository(project.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	// Cache it
	s.repos[projectID] = repo

	return repo, nil
}

// GetStatus returns the git status for a project (with caching)
func (s *Service) GetStatus(projectID string) (*Status, error) {
	// Check cache first
	s.statusCacheMu.RLock()
	if entry, ok := s.statusCache[projectID]; ok {
		if time.Since(entry.timestamp) < s.statusTTL {
			s.statusCacheMu.RUnlock()
			return entry.status, nil
		}
	}
	s.statusCacheMu.RUnlock()

	// Cache miss or expired - fetch fresh status
	repo, err := s.GetRepository(projectID)
	if err != nil {
		return nil, err
	}

	status, err := repo.GetStatus()
	if err != nil {
		return nil, err
	}

	// Update cache
	s.statusCacheMu.Lock()
	s.statusCache[projectID] = &statusCacheEntry{
		status:    status,
		timestamp: time.Now(),
	}
	s.statusCacheMu.Unlock()

	return status, nil
}

// InvalidateStatusCache clears the status cache for a project
// Call this after git operations that change status (commit, add, etc.)
func (s *Service) InvalidateStatusCache(projectID string) {
	s.statusCacheMu.Lock()
	delete(s.statusCache, projectID)
	s.statusCacheMu.Unlock()
}

// InvalidateAllStatusCache clears all status cache
func (s *Service) InvalidateAllStatusCache() {
	s.statusCacheMu.Lock()
	s.statusCache = make(map[string]*statusCacheEntry)
	s.statusCacheMu.Unlock()
}

// GetLog returns the git log for a project
func (s *Service) GetLog(projectID string, opts LogOptions) ([]Commit, error) {
	repo, err := s.GetRepository(projectID)
	if err != nil {
		return nil, err
	}

	return repo.GetLog(opts)
}

// GetDiff returns the git diff for a project
func (s *Service) GetDiff(projectID string, opts DiffOptions) (*Diff, error) {
	repo, err := s.GetRepository(projectID)
	if err != nil {
		return nil, err
	}

	return repo.GetDiff(opts)
}

// GetAllStatus returns git status for all projects
func (s *Service) GetAllStatus() map[string]*Status {
	results := make(map[string]*Status)

	for _, project := range s.projectService.ListProjects() {
		status, err := s.GetStatus(project.ID)
		if err == nil {
			results[project.ID] = status
		}
	}

	return results
}

// EnrichProject adds git information to a project
func (s *Service) EnrichProject(project *projects.Project) error {
	repo, err := s.GetRepository(project.ID)
	if err != nil {
		return err
	}

	status, err := repo.GetStatus()
	if err != nil {
		return err
	}

	project.GitBranch = status.Branch
	project.GitDirty = !status.IsClean || status.HasUntracked
	project.GitAhead = status.Ahead
	project.GitBehind = status.Behind
	project.GitRemote = status.Remote

	return nil
}

// EnrichAllProjects adds git information to all projects
func (s *Service) EnrichAllProjects() {
	for _, project := range s.projectService.ListProjects() {
		s.EnrichProject(project)
	}
}

// IsGitRepository checks if a project is a git repository
func (s *Service) IsGitRepository(projectID string) bool {
	project, err := s.projectService.GetProject(projectID)
	if err != nil {
		return false
	}

	return IsRepository(project.Path)
}

// GetBranches returns all branches for a project
func (s *Service) GetBranches(projectID string) ([]Branch, error) {
	repo, err := s.GetRepository(projectID)
	if err != nil {
		return nil, err
	}

	return repo.GetBranches()
}

// GetRemotes returns all remotes for a project
func (s *Service) GetRemotes(projectID string) ([]Remote, error) {
	repo, err := s.GetRepository(projectID)
	if err != nil {
		return nil, err
	}

	return repo.GetRemotes()
}

// GetHead returns the HEAD commit for a project
func (s *Service) GetHead(projectID string) (*Commit, error) {
	repo, err := s.GetRepository(projectID)
	if err != nil {
		return nil, err
	}

	return repo.GetHead()
}
