package database

import (
	"time"

	"github.com/google/uuid"
)

// SessionState represents the state of a database session
type SessionState string

const (
	SessionIdle    SessionState = "idle"
	SessionRunning SessionState = "running"
	SessionError   SessionState = "error"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabasePostgres DatabaseType = "postgres"
	DatabaseMySQL    DatabaseType = "mysql"
	DatabaseSQLite   DatabaseType = "sqlite"
	DatabaseUnknown  DatabaseType = "unknown"
)

// DatabaseConfig represents a database configuration from YAML
type DatabaseConfig struct {
	URL      string `yaml:"url" json:"url"`
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
	Database string `yaml:"database" json:"database"`
	SSLMode  string `yaml:"sslmode" json:"sslmode"`
}

// ProjectYAMLConfig represents the structure of a project's YAML config
type ProjectYAMLConfig struct {
	Common   *SectionConfig `yaml:"common" json:"common"`
	CLI      *SectionConfig `yaml:"cli" json:"cli"`
	Backend  *SectionConfig `yaml:"backend" json:"backend"`
	Frontend *SectionConfig `yaml:"frontend" json:"frontend"`
}

// SectionConfig represents a section in the YAML config (common, cli, backend)
type SectionConfig struct {
	Database *DatabaseConfig `yaml:"database" json:"database"`
}

// DatabaseInfo represents a discovered database from project configs
type DatabaseInfo struct {
	ID           string       `json:"id"`            // Unique identifier
	ProjectID    string       `json:"project_id"`    // Associated project
	ProjectName  string       `json:"project_name"`  // Project display name
	Source       string       `json:"source"`        // "common", "cli", "backend"
	ConfigFile   string       `json:"config_file"`   // Path to YAML file
	Type         DatabaseType `json:"type"`          // postgres, mysql, etc.
	URL          string       `json:"url"`           // Connection URL
	Host         string       `json:"host"`          // Parsed host
	Port         int          `json:"port"`          // Parsed port
	DatabaseName string       `json:"database_name"` // Database name
	User         string       `json:"user"`          // Username
	SSLMode      string       `json:"sslmode"`       // SSL mode
}

// Session represents a database CLI session (psql, mysql, etc.)
type Session struct {
	ID            string       `json:"id"`             // UUID for the session
	Name          string       `json:"name"`           // Display name
	CustomName    string       `json:"custom_name"`    // User-defined name
	DatabaseID    string       `json:"database_id"`    // Associated database info ID
	ProjectID     string       `json:"project_id"`     // Associated project
	ProjectName   string       `json:"project_name"`   // For display
	DatabaseInfo  *DatabaseInfo `json:"database_info"` // Database connection info
	State         SessionState `json:"state"`
	CreatedAt     time.Time    `json:"created_at"`
	LastActiveAt  time.Time    `json:"last_active_at"`
	Error         string       `json:"error,omitempty"`
}

// GenerateSessionID generates a new UUID for a database session
func GenerateSessionID() string {
	return uuid.New().String()
}

// DisplayName returns the custom name if set, otherwise the default name
func (s *Session) DisplayName() string {
	if s.CustomName != "" {
		return s.CustomName
	}
	return s.Name
}

// SessionSummary is a lightweight version for listing
type SessionSummary struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProjectID    string       `json:"project_id"`
	ProjectName  string       `json:"project_name"`
	DatabaseName string       `json:"database_name"`
	DatabaseType DatabaseType `json:"database_type"`
	State        SessionState `json:"state"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActiveAt time.Time    `json:"last_active_at"`
}

// ToSummary converts a Session to SessionSummary
func (s *Session) ToSummary() SessionSummary {
	dbName := ""
	dbType := DatabaseUnknown
	if s.DatabaseInfo != nil {
		dbName = s.DatabaseInfo.DatabaseName
		dbType = s.DatabaseInfo.Type
	}
	return SessionSummary{
		ID:           s.ID,
		Name:         s.DisplayName(),
		ProjectID:    s.ProjectID,
		ProjectName:  s.ProjectName,
		DatabaseName: dbName,
		DatabaseType: dbType,
		State:        s.State,
		CreatedAt:    s.CreatedAt,
		LastActiveAt: s.LastActiveAt,
	}
}
