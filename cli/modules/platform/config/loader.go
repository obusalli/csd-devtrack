package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"csd-devtrack/cli/modules/core/projects"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigFileName is the default config file name
	DefaultConfigFileName = "csd-devtrack.yaml"
)

var (
	// globalConfig is the globally loaded configuration
	globalConfig *Config
	// globalConfigPath is the path to the loaded config file
	globalConfigPath string
	// configMutex protects config access
	configMutex sync.RWMutex
)

// Loader handles configuration loading and saving
type Loader struct {
	configPath string
}

// NewLoader creates a new config loader
func NewLoader(configPath string) *Loader {
	return &Loader{
		configPath: configPath,
	}
}

// Load loads configuration from file
func (l *Loader) Load() (*Config, error) {
	return l.LoadWithCreate(false)
}

// LoadWithCreate loads configuration from file, optionally creating it if missing
func (l *Loader) LoadWithCreate(createIfMissing bool) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		// Create default config
		config := DefaultConfig()

		// Create the file if requested
		if createIfMissing {
			if err := l.Save(config); err != nil {
				return nil, fmt.Errorf("failed to create config file: %w", err)
			}
		}

		return config, nil
	}

	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing fields
	if config.Settings == nil {
		config.Settings = DefaultSettings()
	} else {
		// Ensure Logger config exists
		if config.Settings.Logger == nil {
			config.Settings.Logger = config.Settings.GetLoggerConfig()
		}
	}

	// Apply default widget profiles if missing
	if config.WidgetProfiles == nil || len(config.WidgetProfiles) == 0 {
		config.WidgetProfiles = DefaultWidgetProfiles()
	}

	// Apply default build profiles if missing
	if config.BuildProfiles == nil || len(config.BuildProfiles) == 0 {
		config.BuildProfiles = DefaultBuildProfiles()
	}

	return &config, nil
}

// Save saves configuration to file
func (l *Loader) Save(config *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(l.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(l.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetPath returns the config file path
func (l *Loader) GetPath() string {
	return l.configPath
}

// Exists checks if config file exists
func (l *Loader) Exists() bool {
	_, err := os.Stat(l.configPath)
	return err == nil
}

// FindConfigFile searches for config file in standard locations
func FindConfigFile() string {
	// Priority order:
	// 1. Current directory
	// 2. Executable directory
	// 3. User config directory

	// 1. Current directory
	cwd, err := os.Getwd()
	if err == nil {
		configPath := filepath.Join(cwd, DefaultConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// 2. Executable directory
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		configPath := filepath.Join(execDir, DefaultConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// 3. User config directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".config", "csd-devtrack", DefaultConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Default to current directory
	if cwd != "" {
		return filepath.Join(cwd, DefaultConfigFileName)
	}

	return DefaultConfigFileName
}

// LoadGlobal loads configuration globally
func LoadGlobal(configPath string) error {
	return LoadGlobalWithCreate(configPath, false)
}

// LoadGlobalWithCreate loads configuration globally, optionally creating the file if missing
func LoadGlobalWithCreate(configPath string, createIfMissing bool) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if configPath == "" {
		configPath = FindConfigFile()
	}

	loader := NewLoader(configPath)
	config, err := loader.LoadWithCreate(createIfMissing)
	if err != nil {
		return err
	}

	// Auto-add csd-devtrack as self project if in correct directory
	// If added, save to YAML so it persists and can be customized
	if ensureSelfProject(config, configPath) {
		// Save the config with the new self project
		_ = loader.Save(config)
	}

	globalConfig = config
	globalConfigPath = configPath

	return nil
}

// ensureSelfProject adds csd-devtrack itself to the project list if detected
// Returns true if a self project was added (config was modified)
func ensureSelfProject(cfg *Config, configPath string) bool {
	// Get the directory containing the config file
	configDir := filepath.Dir(configPath)
	if configDir == "" || configDir == "." {
		configDir, _ = os.Getwd()
	}
	configDir, _ = filepath.Abs(configDir)

	// Check if a project with the same path already exists
	for _, p := range cfg.Projects {
		projectPath, _ := filepath.Abs(p.Path)
		if projectPath == configDir {
			return false // Already has a project for this directory
		}
	}

	// Try to detect csd-devtrack project structure
	detector := projects.NewDetector()
	selfProject, err := detector.DetectProject(configDir)
	if err != nil {
		return false // Not a valid project directory
	}

	// Verify it's actually csd-devtrack (has cli with csd-devtrack.go)
	cliComp := selfProject.GetComponent(projects.ComponentCLI)
	if cliComp == nil || cliComp.EntryPoint != "csd-devtrack.go" {
		return false // Not csd-devtrack
	}

	// Set ID and name, add to config
	// Note: Self flag is computed dynamically at load time based on path
	selfProject.ID = "csd-devtrack"
	selfProject.Name = "csd-devtrack"

	// Insert at the beginning of the project list
	cfg.Projects = append([]projects.Project{*selfProject}, cfg.Projects...)
	return true
}

// GetGlobal returns the global configuration
func GetGlobal() *Config {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if globalConfig == nil {
		return DefaultConfig()
	}
	return globalConfig
}

// GetGlobalPath returns the global config file path
func GetGlobalPath() string {
	configMutex.RLock()
	defer configMutex.RUnlock()

	return globalConfigPath
}

// SaveGlobal saves the global configuration
func SaveGlobal() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("no global config loaded")
	}

	if globalConfigPath == "" {
		return fmt.Errorf("no config path set")
	}

	loader := NewLoader(globalConfigPath)
	return loader.Save(globalConfig)
}

// SetGlobal sets the global configuration
func SetGlobal(config *Config, configPath string) {
	configMutex.Lock()
	defer configMutex.Unlock()

	globalConfig = config
	globalConfigPath = configPath
}

// UpdateSettings updates global settings
func UpdateSettings(settings *Settings) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if globalConfig == nil {
		globalConfig = DefaultConfig()
	}

	globalConfig.Settings = settings
	return nil
}
