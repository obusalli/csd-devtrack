package capabilities

import (
	"strings"
	"sync"
)

// ConfiguredPaths holds user-configured executable paths (from YAML config)
// Empty string means auto-detect
type ConfiguredPaths struct {
	Shell  string
	Claude string
	Codex  string
	Psql   string
	Mysql  string
	Sqlite string
	Git    string
	Go     string
	Node   string
	Npm    string
	Tmux   string
	Sudo   string
}

// Service manages capability detection and caching
type Service struct {
	mu              sync.RWMutex
	capabilities    map[Capability]*CapabilityInfo
	configuredPaths *ConfiguredPaths
}

// NewService creates a new capabilities service and detects all capabilities
func NewService() *Service {
	s := &Service{
		capabilities: make(map[Capability]*CapabilityInfo),
	}
	s.detectAll()
	return s
}

// NewServiceWithConfig creates a service with user-configured paths
func NewServiceWithConfig(paths *ConfiguredPaths) *Service {
	s := &Service{
		capabilities:    make(map[Capability]*CapabilityInfo),
		configuredPaths: paths,
	}
	s.detectAll()
	return s
}

// getConfiguredPath returns the configured path for a capability, or empty if not configured
func (s *Service) getConfiguredPath(cap Capability) string {
	if s.configuredPaths == nil {
		return ""
	}
	switch cap {
	case CapShell:
		return s.configuredPaths.Shell
	case CapClaude:
		return s.configuredPaths.Claude
	case CapCodex:
		return s.configuredPaths.Codex
	case CapPsql:
		return s.configuredPaths.Psql
	case CapMysql:
		return s.configuredPaths.Mysql
	case CapSqlite:
		return s.configuredPaths.Sqlite
	case CapGit:
		return s.configuredPaths.Git
	case CapGo:
		return s.configuredPaths.Go
	case CapNode:
		return s.configuredPaths.Node
	case CapNpm:
		return s.configuredPaths.Npm
	case CapTmux:
		return s.configuredPaths.Tmux
	case CapSudo:
		return s.configuredPaths.Sudo
	default:
		return ""
	}
}

// detectAll detects all capabilities
func (s *Service) detectAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, cap := range AllCapabilities {
		configuredPath := s.getConfiguredPath(cap)
		s.capabilities[cap] = detectWithConfig(cap, configuredPath)
	}
}

// GetSummary returns lists of available and missing capabilities for logging
func (s *Service) GetSummary() (available, missing []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cap := range AllCapabilities {
		if info, ok := s.capabilities[cap]; ok && info.Available {
			available = append(available, string(cap))
		} else {
			missing = append(missing, string(cap))
		}
	}
	return
}

// FormatList formats a list of strings for display
func FormatList(items []string) string {
	return strings.Join(items, ", ")
}

// IsAvailable returns true if the capability is available
func (s *Service) IsAvailable(cap Capability) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if info, ok := s.capabilities[cap]; ok {
		return info.Available
	}
	return false
}

// Get returns the capability info, or nil if not found
func (s *Service) Get(cap Capability) *CapabilityInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if info, ok := s.capabilities[cap]; ok {
		// Return a copy to prevent modification
		copy := *info
		return &copy
	}
	return nil
}

// GetPath returns the path to the capability binary, or empty string if not available
func (s *Service) GetPath(cap Capability) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if info, ok := s.capabilities[cap]; ok && info.Available {
		return info.Path
	}
	return ""
}

// Refresh re-detects all capabilities
func (s *Service) Refresh() {
	s.detectAll()
}

// RefreshOne re-detects a single capability
func (s *Service) RefreshOne(cap Capability) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.capabilities[cap] = detect(cap)
}

// GetMissing returns a list of capabilities that are not available
func (s *Service) GetMissing(caps ...Capability) []Capability {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var missing []Capability
	for _, cap := range caps {
		if info, ok := s.capabilities[cap]; !ok || !info.Available {
			missing = append(missing, cap)
		}
	}
	return missing
}

// AllAvailable returns true if all specified capabilities are available
func (s *Service) AllAvailable(caps ...Capability) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cap := range caps {
		if info, ok := s.capabilities[cap]; !ok || !info.Available {
			return false
		}
	}
	return true
}

// AnyAvailable returns true if any of the specified capabilities are available
func (s *Service) AnyAvailable(caps ...Capability) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cap := range caps {
		if info, ok := s.capabilities[cap]; ok && info.Available {
			return true
		}
	}
	return false
}

// GetAll returns all capability infos
func (s *Service) GetAll() map[Capability]*CapabilityInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[Capability]*CapabilityInfo)
	for k, v := range s.capabilities {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetAvailable returns all available capabilities
func (s *Service) GetAvailable() []Capability {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var available []Capability
	for cap, info := range s.capabilities {
		if info.Available {
			available = append(available, cap)
		}
	}
	return available
}

// ShellInfo holds information about an available shell
type ShellInfo struct {
	Name string // Shell name (bash, zsh, sh, etc.)
	Path string // Full path to the shell
}

// GetAvailableShells returns all available shells (not just the first one found)
func (s *Service) GetAvailableShells() []ShellInfo {
	config, ok := capabilityConfigs[CapShell]
	if !ok {
		return nil
	}

	var shells []ShellInfo
	for _, binary := range config.binaries {
		path := findBinary(binary)
		if path != "" && isExecutable(path) {
			shells = append(shells, ShellInfo{
				Name: binary,
				Path: path,
			})
		}
	}
	return shells
}
