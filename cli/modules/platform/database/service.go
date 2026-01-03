package database

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/projects"

	"gopkg.in/yaml.v3"
)

// Service manages database discovery and sessions
type Service struct {
	mu           sync.RWMutex
	databases    map[string]*DatabaseInfo // key: database ID
	sessions     map[string]*Session      // key: session ID
	projectsFunc func() []projects.Project // Function to get current projects
}

// NewService creates a new database service
func NewService(projectsFunc func() []projects.Project) *Service {
	return &Service{
		databases:    make(map[string]*DatabaseInfo),
		sessions:     make(map[string]*Session),
		projectsFunc: projectsFunc,
	}
}

// DiscoverDatabases scans all projects for database configurations
func (s *Service) DiscoverDatabases() ([]*DatabaseInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing databases
	s.databases = make(map[string]*DatabaseInfo)

	projects := s.projectsFunc()
	var allDatabases []*DatabaseInfo
	seenURLs := make(map[string]bool) // Global deduplication by URL

	for _, proj := range projects {
		// Look for YAML config files in cli/ and backend/ directories
		configPaths := []struct {
			path   string
			source string
		}{
			{filepath.Join(proj.Path, "cli", fmt.Sprintf("%s.yaml", proj.ID)), "cli"},
			{filepath.Join(proj.Path, "backend", fmt.Sprintf("%s.yaml", proj.ID)), "backend"},
			// Also try project name variations
			{filepath.Join(proj.Path, "cli", fmt.Sprintf("%s.yaml", proj.Name)), "cli"},
			{filepath.Join(proj.Path, "backend", fmt.Sprintf("%s.yaml", proj.Name)), "backend"},
		}

		seenFiles := make(map[string]bool) // Track unique config files per project

		for _, cp := range configPaths {
			// Resolve to absolute path for deduplication
			absPath, err := filepath.Abs(cp.path)
			if err != nil {
				absPath = cp.path
			}

			if seenFiles[absPath] {
				continue
			}
			seenFiles[absPath] = true

			if _, err := os.Stat(cp.path); os.IsNotExist(err) {
				continue
			}

			databases, err := s.parseConfigFile(cp.path, proj.ID, proj.Name, cp.source)
			if err != nil {
				continue // Skip files that can't be parsed
			}

			for _, db := range databases {
				// Global deduplication: skip if URL already seen
				if seenURLs[db.URL] {
					continue
				}
				seenURLs[db.URL] = true

				s.databases[db.ID] = db
				allDatabases = append(allDatabases, db)
			}
		}
	}

	return allDatabases, nil
}

// parseConfigFile parses a YAML config file and extracts database configurations
func (s *Service) parseConfigFile(path, projectID, projectName, fileSource string) ([]*DatabaseInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ProjectYAMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var databases []*DatabaseInfo

	// Check common section
	if config.Common != nil && config.Common.Database != nil && config.Common.Database.URL != "" {
		db := s.parseDatabase(config.Common.Database, projectID, projectName, "common", path)
		if db != nil {
			databases = append(databases, db)
		}
	}

	// Check cli section (if different from common)
	if config.CLI != nil && config.CLI.Database != nil && config.CLI.Database.URL != "" {
		db := s.parseDatabase(config.CLI.Database, projectID, projectName, "cli", path)
		if db != nil && !s.isDuplicate(databases, db) {
			databases = append(databases, db)
		}
	}

	// Check backend section (if different from common)
	if config.Backend != nil && config.Backend.Database != nil && config.Backend.Database.URL != "" {
		db := s.parseDatabase(config.Backend.Database, projectID, projectName, "backend", path)
		if db != nil && !s.isDuplicate(databases, db) {
			databases = append(databases, db)
		}
	}

	return databases, nil
}

// isDuplicate checks if a database with the same URL already exists in the list
func (s *Service) isDuplicate(databases []*DatabaseInfo, db *DatabaseInfo) bool {
	for _, existing := range databases {
		if existing.URL == db.URL {
			return true
		}
	}
	return false
}

// parseDatabase creates a DatabaseInfo from a DatabaseConfig
func (s *Service) parseDatabase(cfg *DatabaseConfig, projectID, projectName, source, configFile string) *DatabaseInfo {
	dbURL := cfg.URL
	if dbURL == "" {
		return nil
	}

	db := &DatabaseInfo{
		ID:         fmt.Sprintf("%s-%s", projectID, source),
		ProjectID:  projectID,
		ProjectName: projectName,
		Source:     source,
		ConfigFile: configFile,
		URL:        dbURL,
	}

	// Parse the URL to extract components
	s.parseURL(db, dbURL)

	return db
}

// parseURL parses a database URL and populates the DatabaseInfo fields
func (s *Service) parseURL(db *DatabaseInfo, dbURL string) {
	// Detect database type from URL scheme
	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		db.Type = DatabasePostgres
	} else if strings.HasPrefix(dbURL, "mysql://") {
		db.Type = DatabaseMySQL
	} else if strings.HasPrefix(dbURL, "sqlite://") || strings.HasPrefix(dbURL, "sqlite3://") || strings.HasSuffix(dbURL, ".db") || strings.HasSuffix(dbURL, ".sqlite") {
		db.Type = DatabaseSQLite
	} else {
		db.Type = DatabaseUnknown
	}

	// Parse URL components
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return
	}

	db.Host = parsed.Hostname()
	if port := parsed.Port(); port != "" {
		db.Port, _ = strconv.Atoi(port)
	} else {
		// Default ports
		switch db.Type {
		case DatabasePostgres:
			db.Port = 5432
		case DatabaseMySQL:
			db.Port = 3306
		}
	}

	if parsed.User != nil {
		db.User = parsed.User.Username()
	}

	// Database name is the path without leading slash
	db.DatabaseName = strings.TrimPrefix(parsed.Path, "/")

	// Parse query parameters for SSL mode
	if sslmode := parsed.Query().Get("sslmode"); sslmode != "" {
		db.SSLMode = sslmode
	}
}

// GetDatabases returns all discovered databases
func (s *Service) GetDatabases() []*DatabaseInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var databases []*DatabaseInfo
	for _, db := range s.databases {
		databases = append(databases, db)
	}
	return databases
}

// GetDatabasesByProject returns databases for a specific project
func (s *Service) GetDatabasesByProject(projectID string) []*DatabaseInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var databases []*DatabaseInfo
	for _, db := range s.databases {
		if db.ProjectID == projectID {
			databases = append(databases, db)
		}
	}
	return databases
}

// GetDatabase returns a database by ID
func (s *Service) GetDatabase(id string) *DatabaseInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.databases[id]
}

// GetCLICommand returns the CLI command to connect to a database
func (s *Service) GetCLICommand(db *DatabaseInfo) (string, []string) {
	switch db.Type {
	case DatabasePostgres:
		return "psql", []string{db.URL}
	case DatabaseMySQL:
		args := []string{
			"-h", db.Host,
			"-P", strconv.Itoa(db.Port),
			"-u", db.User,
			db.DatabaseName,
		}
		return "mysql", args
	case DatabaseSQLite:
		return "sqlite3", []string{db.DatabaseName}
	default:
		return "", nil
	}
}

// Session management

// CreateSession creates a new database session
func (s *Service) CreateSession(databaseID, projectID, projectName string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db := s.databases[databaseID]
	if db == nil {
		return nil, fmt.Errorf("database not found: %s", databaseID)
	}

	session := &Session{
		ID:           GenerateSessionID(),
		Name:         fmt.Sprintf("%s (%s)", db.DatabaseName, db.Source),
		DatabaseID:   databaseID,
		ProjectID:    projectID,
		ProjectName:  projectName,
		DatabaseInfo: db,
		State:        SessionIdle,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}

	s.sessions[session.ID] = session
	return session, nil
}

// GetSession returns a session by ID
func (s *Service) GetSession(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

// GetSessions returns all sessions
func (s *Service) GetSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*Session
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

// GetSessionsByProject returns sessions for a specific project
func (s *Service) GetSessionsByProject(projectID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*Session
	for _, sess := range s.sessions {
		if sess.ProjectID == projectID {
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

// UpdateSessionState updates the state of a session
func (s *Service) UpdateSessionState(id string, state SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess := s.sessions[id]
	if sess == nil {
		return fmt.Errorf("session not found: %s", id)
	}

	sess.State = state
	sess.LastActiveAt = time.Now()
	return nil
}

// RenameSession renames a session
func (s *Service) RenameSession(id, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess := s.sessions[id]
	if sess == nil {
		return fmt.Errorf("session not found: %s", id)
	}

	sess.CustomName = newName
	return nil
}

// DeleteSession removes a session
func (s *Service) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	delete(s.sessions, id)
	return nil
}
