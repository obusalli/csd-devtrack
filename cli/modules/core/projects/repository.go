package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Repository handles project persistence to YAML
type Repository struct {
	configPath string
	projects   map[string]*Project
	mu         sync.RWMutex
}

// NewRepository creates a new project repository
func NewRepository(configPath string) *Repository {
	return &Repository{
		configPath: configPath,
		projects:   make(map[string]*Project),
	}
}

// Load loads projects from the YAML config file
func (r *Repository) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if config file exists
	if _, err := os.Stat(r.configPath); os.IsNotExist(err) {
		// No config file yet, that's fine
		r.projects = make(map[string]*Project)
		return nil
	}

	data, err := os.ReadFile(r.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config ConfigFile
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Get config directory for self-detection
	configDir := filepath.Dir(r.configPath)
	if configDir == "" || configDir == "." {
		configDir, _ = os.Getwd()
	}
	configDir, _ = filepath.Abs(configDir)

	// Convert to map and compute Self flag
	r.projects = make(map[string]*Project)
	for _, p := range config.Projects {
		project := p // Create a copy
		// Compute Self: project is self if its path matches config directory
		projectPath, _ := filepath.Abs(project.Path)
		project.Self = (projectPath == configDir)
		r.projects[project.ID] = &project
	}

	return nil
}

// Save saves all projects to the YAML config file
func (r *Repository) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(r.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Convert map to slice
	projects := make([]Project, 0, len(r.projects))
	for _, p := range r.projects {
		projects = append(projects, *p)
	}

	config := ConfigFile{
		Version:  "1.0",
		Settings: DefaultSettings(),
		Projects: projects,
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(r.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetAll returns all projects
func (r *Repository) GetAll() []*Project {
	r.mu.RLock()
	defer r.mu.RUnlock()

	projects := make([]*Project, 0, len(r.projects))
	for _, p := range r.projects {
		projects = append(projects, p)
	}
	return projects
}

// GetByID returns a project by ID
func (r *Repository) GetByID(id string) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.projects[id]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", id)
	}
	return p, nil
}

// Add adds a new project
func (r *Repository) Add(project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[project.ID]; exists {
		return fmt.Errorf("project already exists: %s", project.ID)
	}

	r.projects[project.ID] = project
	return nil
}

// Update updates an existing project
func (r *Repository) Update(project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[project.ID]; !exists {
		return fmt.Errorf("project not found: %s", project.ID)
	}

	r.projects[project.ID] = project
	return nil
}

// AddOrUpdate adds a project if it doesn't exist, or updates it if it does
func (r *Repository) AddOrUpdate(project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.projects[project.ID] = project
	return nil
}

// Remove removes a project by ID
func (r *Repository) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[id]
	if !exists {
		return fmt.Errorf("project not found: %s", id)
	}

	// Prevent removal of self project
	if project.Self {
		return fmt.Errorf("cannot remove self project: %s", id)
	}

	delete(r.projects, id)
	return nil
}

// Exists checks if a project exists
func (r *Repository) Exists(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.projects[id]
	return exists
}

// Count returns the number of projects
func (r *Repository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.projects)
}

// ConfigFile represents the YAML config file structure
type ConfigFile struct {
	Version  string    `yaml:"version"`
	Settings *Settings `yaml:"settings"`
	Projects []Project `yaml:"projects"`
}

// Settings represents global settings
type Settings struct {
	AutoDetect      bool `yaml:"auto_detect"`
	ParallelBuilds  int  `yaml:"parallel_builds"`
	LogBufferSize   int  `yaml:"log_buffer_size"`
	WebEnabled      bool `yaml:"web_enabled"`
	WebPort         int  `yaml:"web_port"`
	WebSocketPort   int  `yaml:"websocket_port"`
}

// DefaultSettings returns default settings
func DefaultSettings() *Settings {
	return &Settings{
		AutoDetect:     true,
		ParallelBuilds: 4,
		LogBufferSize:  10000,
		WebEnabled:     true,
		WebPort:        9099,
		WebSocketPort:  9098,
	}
}
