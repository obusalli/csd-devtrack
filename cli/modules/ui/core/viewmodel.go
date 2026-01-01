package core

import (
	"time"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/git"
)

// ViewModelType identifies the type of view model
type ViewModelType string

const (
	VMDashboard ViewModelType = "dashboard"
	VMProjects  ViewModelType = "projects"
	VMBuild     ViewModelType = "build"
	VMProcesses ViewModelType = "processes"
	VMLogs      ViewModelType = "logs"
	VMGit       ViewModelType = "git"
	VMConfig    ViewModelType = "config"
)

// ViewModel is the base interface for all view models
type ViewModel interface {
	Type() ViewModelType
	LastUpdated() time.Time
}

// BaseViewModel provides common fields for all view models
type BaseViewModel struct {
	VMType      ViewModelType `json:"type"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Error       string        `json:"error,omitempty"`
	IsLoading   bool          `json:"is_loading"`
}

func (vm *BaseViewModel) Type() ViewModelType  { return vm.VMType }
func (vm *BaseViewModel) LastUpdated() time.Time { return vm.UpdatedAt }

// ProjectVM represents a project for display
type ProjectVM struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Path           string              `json:"path"`
	Type           projects.ProjectType `json:"type"`
	IsSelf         bool                `json:"is_self"`
	Components     []ComponentVM       `json:"components"`
	GitBranch      string              `json:"git_branch"`
	GitDirty       bool                `json:"git_dirty"`
	GitAhead       int                 `json:"git_ahead"`
	GitBehind      int                 `json:"git_behind"`
	RunningCount   int                 `json:"running_count"`
	LastBuildTime  *time.Time          `json:"last_build_time,omitempty"`
	LastBuildOK    bool                `json:"last_build_ok"`
}

// ComponentVM represents a component for display
type ComponentVM struct {
	Type        projects.ComponentType `json:"type"`
	Path        string                 `json:"path"`
	Binary      string                 `json:"binary"`
	Port        int                    `json:"port,omitempty"`
	Enabled     bool                   `json:"enabled"`
	IsRunning   bool                   `json:"is_running"`
	PID         int                    `json:"pid,omitempty"`
	Uptime      string                 `json:"uptime,omitempty"`
	LastBuildOK bool                   `json:"last_build_ok"`
}

// ProcessVM represents a process for display
type ProcessVM struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id"`
	ProjectName string                 `json:"project_name"`
	Component   projects.ComponentType `json:"component"`
	State       processes.ProcessState `json:"state"`
	PID         int                    `json:"pid"`
	Uptime      string                 `json:"uptime"`
	Restarts    int                    `json:"restarts"`
	LastError   string                 `json:"last_error,omitempty"`
	LogLines    []string               `json:"log_lines,omitempty"`
	IsSelf      bool                   `json:"is_self,omitempty"` // Is this csd-devtrack itself?
}

// BuildVM represents a build for display
type BuildVM struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id"`
	ProjectName string                 `json:"project_name"`
	Component   projects.ComponentType `json:"component"`
	Status      builds.BuildStatus     `json:"status"`
	Progress    int                    `json:"progress"` // 0-100
	Duration    string                 `json:"duration"`
	StartedAt   time.Time              `json:"started_at"`
	Output      []string               `json:"output"`
	Errors      []string               `json:"errors"`
	Warnings    []string               `json:"warnings"`
	Artifact    string                 `json:"artifact,omitempty"`
}

// GitStatusVM represents git status for display
type GitStatusVM struct {
	ProjectID   string   `json:"project_id"`
	ProjectName string   `json:"project_name"`
	Branch      string   `json:"branch"`
	IsClean     bool     `json:"is_clean"`
	Ahead       int      `json:"ahead"`
	Behind      int      `json:"behind"`
	Staged      []string `json:"staged"`
	Modified    []string `json:"modified"`
	Untracked   []string `json:"untracked"`
	Deleted     []string `json:"deleted"`
}

// CommitVM represents a commit for display
type CommitVM struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"short_hash"`
	Author    string    `json:"author"`
	Date      time.Time `json:"date"`
	DateStr   string    `json:"date_str"`
	Subject   string    `json:"subject"`
}

// LogLineVM represents a log line for display
type LogLineVM struct {
	Timestamp time.Time `json:"timestamp"`
	TimeStr   string    `json:"time_str"`
	Source    string    `json:"source"` // project/component
	Level     string    `json:"level"`  // info, warn, error
	Message   string    `json:"message"`
}

// ============================================
// Composite View Models (for each view/page)
// ============================================

// DashboardVM is the view model for the dashboard
type DashboardVM struct {
	BaseViewModel
	ProjectCount    int          `json:"project_count"`
	RunningCount    int          `json:"running_count"`
	BuildingCount   int          `json:"building_count"`
	ErrorCount      int          `json:"error_count"`
	Projects        []ProjectVM  `json:"projects"`
	RecentBuilds    []BuildVM    `json:"recent_builds"`
	RunningProcesses []ProcessVM `json:"running_processes"`
	GitSummary      []GitStatusVM `json:"git_summary"`
}

// ProjectsVM is the view model for the projects list
type ProjectsVM struct {
	BaseViewModel
	Projects       []ProjectVM `json:"projects"`
	SelectedIndex  int         `json:"selected_index"`
	FilterText     string      `json:"filter_text"`
}

// BuildsVM is the view model for the build view
type BuildsVM struct {
	BaseViewModel
	Projects       []ProjectVM `json:"projects"`
	SelectedProject string     `json:"selected_project"`
	SelectedComponents []projects.ComponentType `json:"selected_components"`
	CurrentBuild   *BuildVM    `json:"current_build,omitempty"`
	BuildHistory   []BuildVM   `json:"build_history"`
	IsBuilding     bool        `json:"is_building"`
}

// ProcessesVM is the view model for the processes view
type ProcessesVM struct {
	BaseViewModel
	Processes      []ProcessVM `json:"processes"`
	SelectedIndex  int         `json:"selected_index"`
	FilterProject  string      `json:"filter_project"`
}

// LogsVM is the view model for the logs view
type LogsVM struct {
	BaseViewModel
	Lines          []LogLineVM `json:"lines"`
	FilterProject  string      `json:"filter_project"`
	FilterComponent string     `json:"filter_component"`
	FilterLevel    string      `json:"filter_level"`
	AutoScroll     bool        `json:"auto_scroll"`
	MaxLines       int         `json:"max_lines"`
}

// GitVM is the view model for the git view
type GitVM struct {
	BaseViewModel
	Projects       []GitStatusVM `json:"projects"`
	SelectedProject string       `json:"selected_project"`
	Commits        []CommitVM    `json:"commits"`
	DiffFiles      []git.FileDiff `json:"diff_files"`
	ShowDiff       bool          `json:"show_diff"`
}

// ConfigVM is the view model for the config view
type ConfigVM struct {
	BaseViewModel
	ConfigPath     string                 `json:"config_path"`
	Settings       map[string]interface{} `json:"settings"`
	Projects       []ProjectVM            `json:"projects"`
	IsEditing      bool                   `json:"is_editing"`
}
