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
	VMClaude    ViewModelType = "claude"
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

// ClaudeSessionVM represents a Claude session for display
type ClaudeSessionVM struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProjectID    string `json:"project_id"`
	ProjectName  string `json:"project_name"`
	WorkDir      string `json:"work_dir"`       // Working directory for Claude
	State        string `json:"state"` // idle, running, waiting, error
	MessageCount int    `json:"message_count"`
	LastActive   string `json:"last_active"`
	IsActive     bool   `json:"is_active"`      // Currently selected session
	IsPersistent bool   `json:"is_persistent"`  // Has active persistent process (fast mode)
	IsWatching   bool   `json:"is_watching"`    // Watching external session for updates
}

// ClaudeMessageVM represents a message for display
type ClaudeMessageVM struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // user, assistant
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	TimeStr   string    `json:"time_str"` // Format: YYMMDD - HH:MM:SS
	IsPartial bool      `json:"is_partial"` // Streaming in progress
}

// ClaudePlanItemVM represents a plan item for display
type ClaudePlanItemVM struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending, in_progress, completed
	ActiveForm string `json:"active_form"`
}

// ClaudeUsageVM represents token/cost usage stats
type ClaudeUsageVM struct {
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	TotalTokens    int     `json:"total_tokens"`
	CacheReadTokens  int   `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int   `json:"cache_write_tokens,omitempty"`
	CostUSD        float64 `json:"cost_usd,omitempty"` // Estimated cost
}

// ClaudeSettingsVM represents Claude settings for the UI
type ClaudeSettingsVM struct {
	AllowedTools    []string `json:"allowed_tools"`    // Tools Claude can use
	AutoApprove     bool     `json:"auto_approve"`     // Auto-approve safe operations
	OutputFormat    string   `json:"output_format"`    // text, json, stream-json
	MaxTurns        int      `json:"max_turns"`        // Max conversation turns
	PlanModeEnabled bool     `json:"plan_mode_enabled"` // Use plan mode for complex tasks
}

// ClaudeInteractiveVM represents the current interactive state
type ClaudeInteractiveVM struct {
	Type        string   `json:"type"`          // "none", "permission", "question", "plan"
	ToolName    string   `json:"tool_name"`     // Tool requesting permission
	ToolID      string   `json:"tool_id"`       // Tool use ID
	FilePath    string   `json:"file_path"`     // File for permission
	Question    string   `json:"question"`      // Question text
	Options     []string `json:"options"`       // Available options
	PlanContent string   `json:"plan_content"`  // Plan content
}

// ClaudeVM is the view model for the Claude AI view
type ClaudeVM struct {
	BaseViewModel
	IsInstalled     bool              `json:"is_installed"`
	ClaudePath      string            `json:"claude_path,omitempty"`
	Sessions        []ClaudeSessionVM `json:"sessions"`
	ActiveSessionID string            `json:"active_session_id,omitempty"`
	ActiveSession   *ClaudeSessionVM  `json:"active_session,omitempty"`
	Messages        []ClaudeMessageVM `json:"messages,omitempty"`
	InputText       string            `json:"input_text"`      // Current input being typed
	IsTyping        bool              `json:"is_typing"`       // User is typing
	IsProcessing    bool              `json:"is_processing"`   // Claude is processing
	FilterProject   string            `json:"filter_project"`  // Filter sessions by project

	// Plan mode
	PlanMode        bool               `json:"plan_mode"`        // Claude is in plan mode
	PlanItems       []ClaudePlanItemVM `json:"plan_items"`       // Current plan items
	PlanPending     bool               `json:"plan_pending"`     // Plan awaiting approval

	// Interactive state (permission requests, questions, etc.)
	Interactive     *ClaudeInteractiveVM `json:"interactive,omitempty"`
	WaitingForInput bool                 `json:"waiting_for_input"` // Claude is waiting for user response

	// Usage stats (for current session)
	Usage           *ClaudeUsageVM    `json:"usage,omitempty"`

	// Settings
	Settings        *ClaudeSettingsVM `json:"settings,omitempty"`
}
