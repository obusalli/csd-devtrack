package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Service manages Codex sessions
type Service struct {
	mu sync.RWMutex

	// Sessions indexed by ID
	sessions map[string]*Session

	// Custom names for sessions (persisted locally)
	customNames     map[string]string
	customNamesFile string

	// Codex CLI path
	codexPath string

	// Base directory for sessions (~/.codex or similar)
	baseDir string
}

// NewService creates a new Codex service
func NewService() *Service {
	homeDir, _ := os.UserHomeDir()
	return &Service{
		sessions:        make(map[string]*Session),
		customNames:     make(map[string]string),
		customNamesFile: filepath.Join(homeDir, ".csd-devtrack", "codex-session-names.json"),
		baseDir:         filepath.Join(homeDir, ".codex"),
	}
}

// Initialize sets up the service
func (s *Service) Initialize(codexPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.codexPath = codexPath

	// Load custom names
	s.loadCustomNames()

	// Discover existing sessions
	s.discoverSessions()

	return nil
}

// GetPath returns the Codex CLI path
func (s *Service) GetPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.codexPath
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
func (s *Service) CreateSession(projectID, projectName, workDir string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := GenerateSessionID()
	now := time.Now()

	session := &Session{
		ID:           id,
		Name:         "Session " + id[:8],
		ProjectID:    projectID,
		ProjectName:  projectName,
		WorkDir:      workDir,
		State:        SessionIdle,
		Messages:     []Message{},
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

// Refresh refreshes session data
func (s *Service) Refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.discoverSessions()
}

// discoverSessions discovers existing sessions from ~/.codex/
func (s *Service) discoverSessions() {
	// Check if base directory exists
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return
	}

	// Look for session files in projects subdirectory
	projectsDir := filepath.Join(s.baseDir, "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return
	}

	// Walk through project directories
	filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Look for session files (*.jsonl)
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			s.loadSessionFile(path)
		}

		return nil
	})
}

// loadSessionFile loads a session from a JSONL file
func (s *Service) loadSessionFile(path string) {
	// Extract session ID from filename
	filename := filepath.Base(path)
	sessionID := strings.TrimSuffix(filename, ".jsonl")

	// Skip if already loaded
	if _, exists := s.sessions[sessionID]; exists {
		return
	}

	// Get file info for timestamps
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	// Determine project from path
	// Path format: ~/.codex/projects/<project-name>/<session-id>.jsonl
	dir := filepath.Dir(path)
	projectName := filepath.Base(dir)

	session := &Session{
		ID:           sessionID,
		Name:         "Session " + sessionID[:8],
		ProjectName:  projectName,
		WorkDir:      "", // Will be determined later
		State:        SessionIdle,
		Messages:     []Message{},
		CreatedAt:    info.ModTime(),
		LastActiveAt: info.ModTime(),
		SessionFile:  path,
	}

	// Apply custom name if exists
	if customName, ok := s.customNames[sessionID]; ok {
		session.CustomName = customName
	}

	s.sessions[sessionID] = session
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
