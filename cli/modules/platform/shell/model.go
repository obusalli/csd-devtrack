package shell

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// SessionType represents the type of shell session
type SessionType string

const (
	SessionHome    SessionType = "home"    // Home directory session
	SessionProject SessionType = "project" // Project directory session
	SessionSudo    SessionType = "sudo"    // Sudo root session
)

// SessionState represents the state of a shell session
type SessionState string

const (
	SessionIdle    SessionState = "idle"
	SessionRunning SessionState = "running"
	SessionError   SessionState = "error"
)

// Session represents a shell session
type Session struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	CustomName   string       `json:"custom_name,omitempty"`
	Type         SessionType  `json:"type"`
	ProjectID    string       `json:"project_id,omitempty"`
	ProjectName  string       `json:"project_name,omitempty"`
	WorkDir      string       `json:"work_dir"`
	Shell        string       `json:"shell,omitempty"` // Shell to use (empty = default)
	State        SessionState `json:"state"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActiveAt time.Time    `json:"last_active_at"`
}

// ShellInfo represents an available shell
type ShellInfo struct {
	Name string // Display name (bash, zsh, etc.)
	Path string // Full path to the executable
}

// DisplayName returns the display name for the session
func (s *Session) DisplayName() string {
	if s.CustomName != "" {
		return s.CustomName
	}
	return s.Name
}

// GenerateSessionID generates a unique session ID
func GenerateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SessionSummary provides a summary for TreeMenu display
type SessionSummary struct {
	ID          string
	Name        string
	Type        SessionType
	ProjectID   string
	ProjectName string
	Shell       string // Shell name for display (bash, zsh, etc.)
	IsActive    bool
	IsRunning   bool
}
