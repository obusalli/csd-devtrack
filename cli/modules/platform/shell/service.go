package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Service manages shell sessions
type Service struct {
	mu sync.RWMutex

	// Sessions indexed by ID
	sessions map[string]*Session

	// Custom names for sessions (persisted locally)
	customNames     map[string]string
	customNamesFile string

	// Default shell path (first detected)
	shellPath string

	// All available shells
	availableShells []ShellInfo

	// Whether sudo is available
	hasSudo bool

	// Home directory
	homeDir string
}

// NewService creates a new shell service
func NewService() *Service {
	homeDir, _ := os.UserHomeDir()
	return &Service{
		sessions:        make(map[string]*Session),
		customNames:     make(map[string]string),
		customNamesFile: filepath.Join(homeDir, ".csd-devtrack", "shell-session-names.json"),
		homeDir:         homeDir,
	}
}

// Initialize sets up the service
func (s *Service) Initialize(shellPath string, hasSudo bool, availableShells []ShellInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shellPath = shellPath
	s.hasSudo = hasSudo
	s.availableShells = availableShells

	// Load custom names
	s.loadCustomNames()

	return nil
}

// GetPath returns the shell path
func (s *Service) GetPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shellPath
}

// HasSudo returns whether sudo is available
func (s *Service) HasSudo() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasSudo
}

// GetAvailableShells returns all available shells
func (s *Service) GetAvailableShells() []ShellInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.availableShells
}

// GetShellPath returns the path for a specific shell name, or the default if not found
func (s *Service) GetShellPath(shellName string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if shellName == "" {
		return s.shellPath
	}

	for _, shell := range s.availableShells {
		if shell.Name == shellName {
			return shell.Path
		}
	}

	return s.shellPath
}

// GetSessionShellPath returns the shell path for a specific session
func (s *Service) GetSessionShellPath(sessionID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session, exists := s.sessions[sessionID]; exists && session.Shell != "" {
		for _, shell := range s.availableShells {
			if shell.Name == session.Shell {
				return shell.Path
			}
		}
	}

	return s.shellPath
}

// SetSessionShell sets the shell for a session
func (s *Service) SetSessionShell(sessionID, shellName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil
	}

	session.Shell = shellName
	return nil
}

// CycleSessionShell cycles to the next available shell for a session
func (s *Service) CycleSessionShell(sessionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists || len(s.availableShells) == 0 {
		return ""
	}

	// Find current shell index
	currentIdx := -1
	currentShell := session.Shell
	if currentShell == "" && len(s.availableShells) > 0 {
		currentShell = s.availableShells[0].Name
	}

	for i, shell := range s.availableShells {
		if shell.Name == currentShell {
			currentIdx = i
			break
		}
	}

	// Cycle to next
	nextIdx := (currentIdx + 1) % len(s.availableShells)
	session.Shell = s.availableShells[nextIdx].Name

	return session.Shell
}

// GetSessions returns all sessions
func (s *Service) GetSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}

	// Sort by last active time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActiveAt.After(sessions[j].LastActiveAt)
	})

	return sessions
}

// GetSession returns a session by ID
func (s *Service) GetSession(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

// CreateSession creates a new session
func (s *Service) CreateSession(sessionType SessionType, projectID, projectName, workDir string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := GenerateSessionID()
	now := time.Now()

	var name string
	switch sessionType {
	case SessionHome:
		name = "Home"
		workDir = s.homeDir
	case SessionSudo:
		name = "Root (sudo)"
		workDir = "/root"
	case SessionProject:
		name = projectName
	}

	session := &Session{
		ID:           id,
		Name:         name,
		Type:         sessionType,
		ProjectID:    projectID,
		ProjectName:  projectName,
		WorkDir:      workDir,
		State:        SessionIdle,
		CreatedAt:    now,
		LastActiveAt: now,
	}

	s.sessions[id] = session

	return session, nil
}

// DeleteSession deletes a session
func (s *Service) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; !exists {
		return nil
	}

	delete(s.sessions, id)
	delete(s.customNames, id)

	// Save custom names
	s.saveCustomNames()

	return nil
}

// RenameSession renames a session
func (s *Service) RenameSession(id, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return nil
	}

	session.CustomName = newName
	s.customNames[id] = newName

	// Save custom names
	s.saveCustomNames()

	return nil
}

// UpdateSessionState updates a session's state
func (s *Service) UpdateSessionState(id string, state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[id]; exists {
		session.State = state
		session.LastActiveAt = time.Now()
	}
}

// GetHomeDir returns the home directory
func (s *Service) GetHomeDir() string {
	return s.homeDir
}

// loadCustomNames loads custom session names from disk
func (s *Service) loadCustomNames() {
	data, err := os.ReadFile(s.customNamesFile)
	if err != nil {
		return
	}

	json.Unmarshal(data, &s.customNames)
}

// saveCustomNames saves custom session names to disk
func (s *Service) saveCustomNames() {
	// Ensure directory exists
	dir := filepath.Dir(s.customNamesFile)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(s.customNames, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(s.customNamesFile, data, 0644)
}
