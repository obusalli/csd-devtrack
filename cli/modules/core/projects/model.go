package projects

import (
	"time"
)

// ProjectType represents the type of project structure
type ProjectType string

const (
	ProjectTypeFullStack    ProjectType = "full-stack"    // agent + cli + backend + frontend
	ProjectTypeBackendOnly  ProjectType = "backend-only"  // cli + backend
	ProjectTypeFrontendOnly ProjectType = "frontend-only" // frontend only
	ProjectTypeDaemon       ProjectType = "daemon"        // single go binary
	ProjectTypeCustom       ProjectType = "custom"        // custom combination
)

// ComponentType represents a project component type
type ComponentType string

const (
	ComponentAgent    ComponentType = "agent"
	ComponentCLI      ComponentType = "cli"
	ComponentBackend  ComponentType = "backend"
	ComponentFrontend ComponentType = "frontend"
)

// AllComponentTypes returns all component types in build order
func AllComponentTypes() []ComponentType {
	return []ComponentType{
		ComponentBackend,  // Backend first (CLI may depend on it)
		ComponentCLI,      // CLI second
		ComponentAgent,    // Agent third
		ComponentFrontend, // Frontend last
	}
}

// Component represents a buildable component of a project
type Component struct {
	Type       ComponentType `yaml:"type" json:"type"`
	Path       string        `yaml:"path" json:"path"`               // Relative path (e.g., "cli/")
	EntryPoint string        `yaml:"entry_point" json:"entry_point"` // E.g., "csd-corectl.go"
	Binary     string        `yaml:"binary" json:"binary"`           // E.g., "csd-corectl"
	BuildCmd   string        `yaml:"build_cmd" json:"build_cmd"`     // Override build command
	RunCmd     string        `yaml:"run_cmd" json:"run_cmd"`         // Override run command
	Port       int           `yaml:"port" json:"port"`               // Port if applicable
	Enabled    bool          `yaml:"enabled" json:"enabled"`

	// Runtime state (not persisted)
	LastBuildTime   *time.Time `yaml:"-" json:"last_build_time,omitempty"`
	LastBuildStatus string     `yaml:"-" json:"last_build_status,omitempty"`
}

// Project represents a managed project
type Project struct {
	ID         string                   `yaml:"id" json:"id"`
	Name       string                   `yaml:"name" json:"name"`
	Path       string                   `yaml:"path" json:"path"` // Absolute or relative path
	Type       ProjectType              `yaml:"type" json:"type"`
	Self       bool                     `yaml:"self,omitempty" json:"self,omitempty"` // Is this csd-devtrack itself?
	Components map[ComponentType]*Component `yaml:"components" json:"components"`

	// Git info (computed, not persisted)
	GitBranch  string `yaml:"-" json:"git_branch,omitempty"`
	GitDirty   bool   `yaml:"-" json:"git_dirty,omitempty"`
	GitAhead   int    `yaml:"-" json:"git_ahead,omitempty"`
	GitBehind  int    `yaml:"-" json:"git_behind,omitempty"`
	GitRemote  string `yaml:"-" json:"git_remote,omitempty"`
}

// GetEnabledComponents returns all enabled components in build order
func (p *Project) GetEnabledComponents() []*Component {
	var components []*Component
	for _, ct := range AllComponentTypes() {
		if comp, ok := p.Components[ct]; ok && comp.Enabled {
			components = append(components, comp)
		}
	}
	return components
}

// GetComponent returns a component by type
func (p *Project) GetComponent(ct ComponentType) *Component {
	return p.Components[ct]
}

// HasComponent checks if project has a specific component type
func (p *Project) HasComponent(ct ComponentType) bool {
	comp, ok := p.Components[ct]
	return ok && comp != nil && comp.Enabled
}

// IsGoComponent checks if a component is a Go component
func IsGoComponent(ct ComponentType) bool {
	return ct == ComponentAgent || ct == ComponentCLI || ct == ComponentBackend
}

// IsFrontendComponent checks if a component is a frontend component
func IsFrontendComponent(ct ComponentType) bool {
	return ct == ComponentFrontend
}

// ProjectSummary is a lightweight project summary for listing
type ProjectSummary struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Type           ProjectType   `json:"type"`
	ComponentCount int           `json:"component_count"`
	GitBranch      string        `json:"git_branch"`
	GitDirty       bool          `json:"git_dirty"`
	RunningCount   int           `json:"running_count"`
}

// ToSummary converts a project to a summary
func (p *Project) ToSummary() *ProjectSummary {
	return &ProjectSummary{
		ID:             p.ID,
		Name:           p.Name,
		Type:           p.Type,
		ComponentCount: len(p.GetEnabledComponents()),
		GitBranch:      p.GitBranch,
		GitDirty:       p.GitDirty,
	}
}
