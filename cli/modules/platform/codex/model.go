package codex

import (
	"time"

	"github.com/google/uuid"
)

// SessionState represents the state of a Codex session
type SessionState string

const (
	SessionIdle    SessionState = "idle"
	SessionRunning SessionState = "running"
	SessionWaiting SessionState = "waiting" // Waiting for user input
	SessionError   SessionState = "error"
)

// Message represents a single message in a Codex conversation
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Partial   bool      `json:"partial"` // True if streaming in progress
}

// Session represents a Codex session
type Session struct {
	ID           string       `json:"id"`             // UUID for session
	Name         string       `json:"name"`           // Display name
	CustomName   string       `json:"custom_name"`    // User-defined custom name
	ProjectID    string       `json:"project_id"`     // Associated project
	ProjectName  string       `json:"project_name"`   // For display
	WorkDir      string       `json:"work_dir"`       // Working directory
	State        SessionState `json:"state"`
	Messages     []Message    `json:"messages"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActiveAt time.Time    `json:"last_active_at"`
	Error        string       `json:"error,omitempty"`

	// Session file info
	SessionFile string `json:"session_file"` // Path to session file
}

// GenerateSessionID generates a new UUID for a session
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
	WorkDir      string       `json:"work_dir"`
	State        SessionState `json:"state"`
	MessageCount int          `json:"message_count"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActiveAt time.Time    `json:"last_active_at"`
}

// ToSummary converts a Session to SessionSummary
func (s *Session) ToSummary() SessionSummary {
	return SessionSummary{
		ID:           s.ID,
		Name:         s.DisplayName(),
		ProjectID:    s.ProjectID,
		ProjectName:  s.ProjectName,
		WorkDir:      s.WorkDir,
		State:        s.State,
		MessageCount: len(s.Messages),
		CreatedAt:    s.CreatedAt,
		LastActiveAt: s.LastActiveAt,
	}
}
