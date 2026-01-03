package config

import (
	"csd-devtrack/cli/modules/core/projects"
)

// Config represents the main configuration
type Config struct {
	Version        string                    `yaml:"version"`
	Settings       *Settings                 `yaml:"settings"`
	Projects       []projects.Project        `yaml:"projects"`
	BuildProfiles  map[string]*BuildProfile  `yaml:"build_profiles,omitempty"`
	WidgetProfiles map[string]*WidgetProfile `yaml:"widget_profiles,omitempty"`
}

// WidgetType represents the type of widget
type WidgetType string

const (
	WidgetLogs             WidgetType = "logs"
	WidgetClaudeSessions   WidgetType = "claude_sessions"
	WidgetDatabaseSessions WidgetType = "database_sessions"
	WidgetProcesses        WidgetType = "processes"
	WidgetBuildStatus      WidgetType = "build_status"
	WidgetGitStatus        WidgetType = "git_status"
	WidgetDashStats        WidgetType = "dashboard_stats"
)

// WidgetConfig represents the configuration for a single widget
type WidgetConfig struct {
	ID             string `yaml:"id" json:"id"`
	Type           string `yaml:"type" json:"type"` // logs, processes, build_status, etc.
	Title          string `yaml:"title,omitempty" json:"title,omitempty"`
	Row            int    `yaml:"row" json:"row"`
	Col            int    `yaml:"col" json:"col"`
	RowSpan        int    `yaml:"row_span,omitempty" json:"row_span,omitempty"` // Default: 1
	ColSpan        int    `yaml:"col_span,omitempty" json:"col_span,omitempty"` // Default: 1
	Width          int    `yaml:"width,omitempty" json:"width,omitempty"`       // Custom width in characters (0 = auto)
	Height         int    `yaml:"height,omitempty" json:"height,omitempty"`     // Custom height in lines (0 = auto)
	ProjectFilter  string `yaml:"project_filter,omitempty" json:"project_filter,omitempty"`
	LogTypeFilter  string `yaml:"log_type_filter,omitempty" json:"log_type_filter,omitempty"`   // build, process
	LogLevelFilter string `yaml:"log_level_filter,omitempty" json:"log_level_filter,omitempty"` // error, warn, info
	SessionID      string `yaml:"session_id,omitempty" json:"session_id,omitempty"`             // For Claude sessions widget
}

// WidgetProfile represents a named layout configuration
type WidgetProfile struct {
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Rows        int            `yaml:"rows" json:"rows"` // 1-4
	Cols        int            `yaml:"cols" json:"cols"` // 1-4
	Widgets     []WidgetConfig `yaml:"widgets" json:"widgets"`
}

// BuildProfile represents a build configuration profile
type BuildProfile struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	EnvVars     map[string]string `yaml:"env_vars,omitempty" json:"env_vars,omitempty"`
	BuildFlags  []string          `yaml:"build_flags,omitempty" json:"build_flags,omitempty"`
	LDFlags     string            `yaml:"ld_flags,omitempty" json:"ld_flags,omitempty"`
	Tags        []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Race        bool              `yaml:"race,omitempty" json:"race,omitempty"`
	Verbose     bool              `yaml:"verbose,omitempty" json:"verbose,omitempty"`
	Optimize    bool              `yaml:"optimize,omitempty" json:"optimize,omitempty"` // -O for production
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Level         string `yaml:"level" json:"level"`                   // debug, info, warn, error
	FilePath      string `yaml:"file_path" json:"file_path"`           // Log file path (empty = no file)
	MaxSizeMB     int    `yaml:"max_size_mb" json:"max_size_mb"`       // Max log file size before rotation
	BufferSize    int    `yaml:"buffer_size" json:"buffer_size"`       // Log buffer size for TUI
	CaptureStderr bool   `yaml:"capture_stderr" json:"capture_stderr"` // Capture stderr output
	CaptureStdout bool   `yaml:"capture_stdout" json:"capture_stdout"` // Capture stdout output
}

// DefaultLoggerConfig returns default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:         "info",
		FilePath:      "", // Will default to ~/.csd-devtrack/daemon.log
		MaxSizeMB:     10,
		BufferSize:    10000,
		CaptureStderr: true,
		CaptureStdout: false,
	}
}

// Settings represents global application settings
type Settings struct {
	// Auto-detection
	AutoDetect bool `yaml:"auto_detect" json:"auto_detect"`

	// Build settings
	ParallelBuilds int `yaml:"parallel_builds" json:"parallel_builds"`

	// Logging (legacy fields for backwards compatibility)
	LogBufferSize int    `yaml:"log_buffer_size,omitempty" json:"log_buffer_size,omitempty"`
	LogLevel      string `yaml:"log_level,omitempty" json:"log_level,omitempty"`

	// Logger configuration
	Logger *LoggerConfig `yaml:"logger,omitempty" json:"logger,omitempty"`

	// Web server
	WebEnabled    bool `yaml:"web_enabled" json:"web_enabled"`
	WebPort       int  `yaml:"web_port" json:"web_port"`
	WebSocketPort int  `yaml:"websocket_port" json:"websocket_port"`

	// CSD-Core integration
	CSDCoreEnabled    bool   `yaml:"csd_core_enabled" json:"csd_core_enabled"`
	CSDCoreURL        string `yaml:"csd_core_url" json:"csd_core_url"`
	CSDCoreFederation bool   `yaml:"csd_core_federation" json:"csd_core_federation"`

	// UI settings
	Theme          string `yaml:"theme" json:"theme"` // dark, light, auto
	RefreshRate    int    `yaml:"refresh_rate" json:"refresh_rate"` // ms
	ShowTimestamps bool   `yaml:"show_timestamps" json:"show_timestamps"`

	// Browser settings
	BrowserPath string `yaml:"browser_path,omitempty" json:"browser_path,omitempty"` // Default path for file browser (default: home directory)

	// Claude AI integration
	Claude *ClaudeConfig `yaml:"claude,omitempty" json:"claude,omitempty"`

	// External executables configuration
	Executables *ExecutablesConfig `yaml:"executables,omitempty" json:"executables,omitempty"`

	// Widgets view
	ActiveWidgetProfile string `yaml:"active_widget_profile,omitempty" json:"active_widget_profile,omitempty"`
}

// ExecutablesConfig allows overriding auto-detected executable paths
// Empty string = auto-detect, explicit path = use that path
type ExecutablesConfig struct {
	// Shell: bash, zsh, sh (Unix) or pwsh, powershell, cmd (Windows)
	Shell string `yaml:"shell,omitempty" json:"shell,omitempty"`

	// AI assistants
	Claude string `yaml:"claude,omitempty" json:"claude,omitempty"`
	Codex  string `yaml:"codex,omitempty" json:"codex,omitempty"`

	// Database clients
	Psql   string `yaml:"psql,omitempty" json:"psql,omitempty"`
	Mysql  string `yaml:"mysql,omitempty" json:"mysql,omitempty"`
	Sqlite string `yaml:"sqlite,omitempty" json:"sqlite,omitempty"`

	// Development tools
	Git  string `yaml:"git,omitempty" json:"git,omitempty"`
	Go   string `yaml:"go,omitempty" json:"go,omitempty"`
	Node string `yaml:"node,omitempty" json:"node,omitempty"`
	Npm  string `yaml:"npm,omitempty" json:"npm,omitempty"`

	// System tools
	Tmux string `yaml:"tmux,omitempty" json:"tmux,omitempty"`
	Sudo string `yaml:"sudo,omitempty" json:"sudo,omitempty"`
}

// ClaudeConfig represents Claude AI integration settings
type ClaudeConfig struct {
	// Path to Claude CLI binary (empty = auto-detect)
	Path string `yaml:"path,omitempty" json:"path,omitempty"`

	// Allowed tools for Claude to use
	AllowedTools []string `yaml:"allowed_tools,omitempty" json:"allowed_tools,omitempty"`

	// Auto-approve safe operations (read-only tools)
	AutoApprove bool `yaml:"auto_approve,omitempty" json:"auto_approve,omitempty"`

	// Output format: text, json, stream-json
	OutputFormat string `yaml:"output_format,omitempty" json:"output_format,omitempty"`

	// Max conversation turns
	MaxTurns int `yaml:"max_turns,omitempty" json:"max_turns,omitempty"`

	// Enable plan mode for complex tasks
	PlanModeEnabled bool `yaml:"plan_mode_enabled,omitempty" json:"plan_mode_enabled,omitempty"`

	// Sessions data directory (for storing session history)
	SessionsDir string `yaml:"sessions_dir,omitempty" json:"sessions_dir,omitempty"`
}

// DefaultClaudeConfig returns default Claude configuration
func DefaultClaudeConfig() *ClaudeConfig {
	return &ClaudeConfig{
		Path:            "", // Auto-detect
		AllowedTools:    []string{"Read", "Glob", "Grep", "Bash"},
		AutoApprove:     false,
		OutputFormat:    "stream-json",
		MaxTurns:        10,
		PlanModeEnabled: true,
		SessionsDir:     "", // Will default to ~/.csd-devtrack/claude-sessions/
	}
}

// GetLoggerConfig returns the logger config, applying defaults and legacy field migration
func (s *Settings) GetLoggerConfig() *LoggerConfig {
	if s.Logger != nil {
		return s.Logger
	}
	// Migrate legacy fields
	cfg := DefaultLoggerConfig()
	if s.LogLevel != "" {
		cfg.Level = s.LogLevel
	}
	if s.LogBufferSize > 0 {
		cfg.BufferSize = s.LogBufferSize
	}
	return cfg
}

// DefaultSettings returns default configuration settings
func DefaultSettings() *Settings {
	return &Settings{
		// Auto-detection
		AutoDetect: true,

		// Build settings
		ParallelBuilds: 4,

		// Logger configuration
		Logger: DefaultLoggerConfig(),

		// Web server
		WebEnabled:    true,
		WebPort:       9099,
		WebSocketPort: 9098,

		// CSD-Core integration
		CSDCoreEnabled:    false,
		CSDCoreURL:        "http://localhost:8080",
		CSDCoreFederation: true,

		// UI settings
		Theme:          "dark",
		RefreshRate:    5000, // 5 seconds
		ShowTimestamps: true,
	}
}

// DefaultBuildProfiles returns the default build profiles
func DefaultBuildProfiles() map[string]*BuildProfile {
	return map[string]*BuildProfile{
		"dev": {
			Name:        "dev",
			Description: "Development build with debug symbols",
			EnvVars: map[string]string{
				"CGO_ENABLED": "0",
			},
			BuildFlags: []string{"-v"},
			Race:       false,
			Verbose:    true,
			Optimize:   false,
		},
		"test": {
			Name:        "test",
			Description: "Test build with race detection",
			EnvVars: map[string]string{
				"CGO_ENABLED": "1",
			},
			BuildFlags: []string{"-v"},
			Race:       true,
			Verbose:    true,
			Optimize:   false,
		},
		"prod": {
			Name:        "prod",
			Description: "Production build, optimized",
			EnvVars: map[string]string{
				"CGO_ENABLED": "0",
			},
			LDFlags:  "-s -w",
			Tags:     []string{"production"},
			Race:     false,
			Verbose:  false,
			Optimize: true,
		},
	}
}

// DefaultWidgetProfiles returns empty widget profiles (user configures from scratch)
func DefaultWidgetProfiles() map[string]*WidgetProfile {
	return map[string]*WidgetProfile{}
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Version:        "1.0",
		Settings:       DefaultSettings(),
		Projects:       []projects.Project{},
		BuildProfiles:  DefaultBuildProfiles(),
		WidgetProfiles: DefaultWidgetProfiles(),
	}
}

// Validate validates the configuration
func (c *Config) Validate() []string {
	var errors []string

	if c.Settings == nil {
		errors = append(errors, "settings is required")
		return errors
	}

	if c.Settings.ParallelBuilds < 1 {
		errors = append(errors, "parallel_builds must be at least 1")
	}

	if c.Settings.LogBufferSize < 100 {
		errors = append(errors, "log_buffer_size must be at least 100")
	}

	if c.Settings.WebEnabled {
		if c.Settings.WebPort < 1 || c.Settings.WebPort > 65535 {
			errors = append(errors, "web_port must be between 1 and 65535")
		}
		if c.Settings.WebSocketPort < 1 || c.Settings.WebSocketPort > 65535 {
			errors = append(errors, "websocket_port must be between 1 and 65535")
		}
		if c.Settings.WebPort == c.Settings.WebSocketPort {
			errors = append(errors, "web_port and websocket_port must be different")
		}
	}

	return errors
}

// Merge merges another config into this one (other takes precedence)
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	if other.Version != "" {
		c.Version = other.Version
	}

	if other.Settings != nil {
		c.Settings = other.Settings
	}

	if len(other.Projects) > 0 {
		c.Projects = other.Projects
	}
}
