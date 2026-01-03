package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration (final merged config)
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	CSDCore  CSDCoreConfig  `yaml:"csd-core"`
	Frontend FrontendConfig `yaml:"frontend"`
	JWT      JWTConfig      `yaml:"jwt"`
	CORS     CORSConfig     `yaml:"cors"`
	Logging  LoggingConfig  `yaml:"logging"`
	CLI      CLIConfig      `yaml:"cli"`
}

// RawConfig represents the YAML file structure with common/backend/frontend/cli sections
type RawConfig struct {
	Common   CommonConfig   `yaml:"common"`
	Backend  BackendConfig  `yaml:"backend"`
	Frontend FrontendConfig `yaml:"frontend"`
	CLI      CLIConfig      `yaml:"cli"`
}

type CommonConfig struct {
	CSDCore CSDCoreConfig `yaml:"csd-core"`
	Logging LoggingConfig `yaml:"logging"`
}

type BackendConfig struct {
	Server  ServerConfig  `yaml:"server"`
	CSDCore CSDCoreConfig `yaml:"csd-core"`
	JWT     JWTConfig     `yaml:"jwt"`
	CORS    CORSConfig    `yaml:"cors"`
	CLI     CLIConfig     `yaml:"cli"`
}

type ServerConfig struct {
	Host           string `yaml:"host"`
	Port           string `yaml:"port"`
	InternalSecret string `yaml:"internal-secret"`
}

type CSDCoreConfig struct {
	URL             string `yaml:"url"`
	GraphQLEndpoint string `yaml:"graphql-endpoint"`
	ServiceToken    string `yaml:"service-token"`
}

// FrontendConfig holds frontend integration settings for Module Federation
type FrontendConfig struct {
	URL             string `yaml:"url"`               // e.g., http://localhost:4044
	RemoteEntryPath string `yaml:"remote-entry-path"` // e.g., /assets/remoteEntry.js
	RoutePath       string `yaml:"route-path"`        // e.g., /devtrack
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	Issuer      string `yaml:"issuer"`
	ExpiryHours int    `yaml:"expiry-hours"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed-origins"`
	AllowedMethods []string `yaml:"allowed-methods"`
	AllowedHeaders []string `yaml:"allowed-headers"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

// CLIConfig holds paths to CLI data directories
type CLIConfig struct {
	DataDir   string `yaml:"data-dir"`   // ~/.csd-devtrack
	ClaudeDir string `yaml:"claude-dir"` // ~/.claude
}

var globalConfig *Config

// Load loads configuration from a YAML file
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		// Try default locations
		candidates := []string{
			"csd-devtrack.yaml",
			"backend/csd-devtrack.yaml",
			filepath.Join(os.Getenv("HOME"), ".config", "csd-devtrack", "csd-devtrack.yaml"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				configPath = candidate
				break
			}
		}
	}

	if configPath == "" {
		return nil, fmt.Errorf("no configuration file found")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var rawCfg RawConfig
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge common with backend-specific config
	cfg := mergeConfig(rawCfg)

	// Override with environment variables for sensitive data
	if envJWTSecret := os.Getenv("CSD_JWT_SECRET"); envJWTSecret != "" {
		cfg.JWT.Secret = envJWTSecret
	}
	if envCoreToken := os.Getenv("CSD_CORE_SERVICE_TOKEN"); envCoreToken != "" {
		cfg.CSDCore.ServiceToken = envCoreToken
	}
	if envCoreURL := os.Getenv("CSD_CORE_URL"); envCoreURL != "" {
		cfg.CSDCore.URL = envCoreURL
	}

	// Set defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "9094"
	}
	if cfg.CSDCore.GraphQLEndpoint == "" {
		cfg.CSDCore.GraphQLEndpoint = "/core/api/latest/query"
	}
	if cfg.JWT.ExpiryHours == 0 {
		cfg.JWT.ExpiryHours = 24
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}

	// CLI defaults - expand ~ to home directory
	home := os.Getenv("HOME")
	if cfg.CLI.DataDir == "" || cfg.CLI.DataDir == "~/.csd-devtrack" {
		cfg.CLI.DataDir = filepath.Join(home, ".csd-devtrack")
	}
	if cfg.CLI.ClaudeDir == "" || cfg.CLI.ClaudeDir == "~/.claude" {
		cfg.CLI.ClaudeDir = filepath.Join(home, ".claude")
	}

	globalConfig = &cfg
	return &cfg, nil
}

// mergeConfig merges common config with backend-specific overrides
func mergeConfig(raw RawConfig) Config {
	cfg := Config{
		// Start with common values
		Logging: raw.Common.Logging,
		CSDCore: raw.Common.CSDCore,

		// Backend-specific values
		Server:   raw.Backend.Server,
		JWT:      raw.Backend.JWT,
		CORS:     raw.Backend.CORS,
		Frontend: raw.Frontend,
		CLI:      raw.Backend.CLI,
	}

	// Override common with backend-specific if set
	if raw.Backend.CSDCore.ServiceToken != "" {
		cfg.CSDCore.ServiceToken = raw.Backend.CSDCore.ServiceToken
	}
	if raw.Backend.CSDCore.URL != "" {
		cfg.CSDCore.URL = raw.Backend.CSDCore.URL
	}
	if raw.Backend.CSDCore.GraphQLEndpoint != "" {
		cfg.CSDCore.GraphQLEndpoint = raw.Backend.CSDCore.GraphQLEndpoint
	}

	return cfg
}

// GetConfig returns the global configuration
func GetConfig() *Config {
	return globalConfig
}

// SetConfig sets the global configuration
func SetConfig(cfg *Config) {
	globalConfig = cfg
}
