package config

import (
	"csd-devtrack/cli/modules/core/projects"
)

// Config represents the main configuration
type Config struct {
	Version       string              `yaml:"version"`
	Settings      *Settings           `yaml:"settings"`
	Projects      []projects.Project  `yaml:"projects"`
	BuildProfiles map[string]*BuildProfile `yaml:"build_profiles,omitempty"`
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

// Settings represents global application settings
type Settings struct {
	// Auto-detection
	AutoDetect bool `yaml:"auto_detect" json:"auto_detect"`

	// Build settings
	ParallelBuilds int `yaml:"parallel_builds" json:"parallel_builds"`

	// Logging
	LogBufferSize int    `yaml:"log_buffer_size" json:"log_buffer_size"`
	LogLevel      string `yaml:"log_level" json:"log_level"`

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
}

// DefaultSettings returns default configuration settings
func DefaultSettings() *Settings {
	return &Settings{
		// Auto-detection
		AutoDetect: true,

		// Build settings
		ParallelBuilds: 4,

		// Logging
		LogBufferSize: 10000,
		LogLevel:      "info",

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
		RefreshRate:    1000,
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

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Version:       "1.0",
		Settings:      DefaultSettings(),
		Projects:      []projects.Project{},
		BuildProfiles: DefaultBuildProfiles(),
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
