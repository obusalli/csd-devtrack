package claude

import (
	"time"
)

// SessionState represents the state of a Claude session
type SessionState string

const (
	SessionIdle    SessionState = "idle"
	SessionRunning SessionState = "running"
	SessionWaiting SessionState = "waiting" // Waiting for user input
	SessionError   SessionState = "error"
)

// Message represents a single message in a Claude conversation
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Partial   bool      `json:"partial"` // True if streaming in progress
}

// Session represents a Claude Code session
type Session struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`          // User-friendly name
	ProjectID    string       `json:"project_id"`    // Associated project
	ProjectName  string       `json:"project_name"`  // For display
	WorkDir      string       `json:"work_dir"`      // Working directory for Claude
	State        SessionState `json:"state"`
	Messages     []Message    `json:"messages"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActiveAt time.Time    `json:"last_active_at"`
	Error        string       `json:"error,omitempty"`

	// Interactive state
	Interactive *InteractiveState `json:"interactive,omitempty"`
}

// SessionSummary is a lightweight version for listing
type SessionSummary struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProjectID    string       `json:"project_id"`
	ProjectName  string       `json:"project_name"`
	State        SessionState `json:"state"`
	MessageCount int          `json:"message_count"`
	LastActiveAt time.Time    `json:"last_active_at"`
}

// ToSummary converts a Session to SessionSummary
func (s *Session) ToSummary() SessionSummary {
	return SessionSummary{
		ID:           s.ID,
		Name:         s.Name,
		ProjectID:    s.ProjectID,
		ProjectName:  s.ProjectName,
		State:        s.State,
		MessageCount: len(s.Messages),
		LastActiveAt: s.LastActiveAt,
	}
}

// ClaudeOutput represents output from Claude CLI
type ClaudeOutput struct {
	Type    string `json:"type"`    // "text", "tool_use", "tool_result", "thinking", "permission_request", "permission_denied", "question", "plan", "error", "end"
	Content string `json:"content"`
	Tool    string `json:"tool,omitempty"`     // Tool name for tool_use
	ToolID  string `json:"tool_id,omitempty"`  // Tool use ID for matching results
	IsEnd   bool   `json:"is_end"`             // True when turn is complete

	// Interactive fields
	WaitingForInput bool              `json:"waiting_for_input,omitempty"` // Claude is waiting for user response
	InputType       string            `json:"input_type,omitempty"`        // "permission", "question", "plan"
	Options         []string          `json:"options,omitempty"`           // Available options for questions
	FilePath        string            `json:"file_path,omitempty"`         // File path for permission requests
	PlanItems       []PlanItem        `json:"plan_items,omitempty"`        // Plan items for plan mode
}

// PlanItem represents an item in Claude's plan
type PlanItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // "pending", "in_progress", "completed"
}

// InteractiveState tracks the current interactive state
type InteractiveState struct {
	Type        string   `json:"type"`         // "none", "permission", "question", "plan"
	ToolName    string   `json:"tool_name"`    // Tool requesting permission
	ToolID      string   `json:"tool_id"`      // Tool use ID to respond to
	FilePath    string   `json:"file_path"`    // File for permission
	Question    string   `json:"question"`     // Question text
	Options     []string `json:"options"`      // Options for question
	PlanContent string   `json:"plan_content"` // Plan content awaiting approval
}
