package core

import (
	"csd-devtrack/cli/modules/core/projects"
)

// EventType identifies the type of UI event
type EventType string

const (
	// Navigation events
	EventNavigate     EventType = "navigate"
	EventBack         EventType = "back"
	EventRefresh      EventType = "refresh"
	EventQuit         EventType = "quit"

	// Project events
	EventSelectProject   EventType = "select_project"
	EventAddProject      EventType = "add_project"
	EventRemoveProject   EventType = "remove_project"
	EventRefreshProject  EventType = "refresh_project"

	// Build events
	EventStartBuild      EventType = "start_build"
	EventCancelBuild     EventType = "cancel_build"
	EventBuildAll        EventType = "build_all"
	EventSelectComponent EventType = "select_component"

	// Process events
	EventStartProcess    EventType = "start_process"
	EventStopProcess     EventType = "stop_process"
	EventRestartProcess  EventType = "restart_process"
	EventKillProcess     EventType = "kill_process"
	EventPauseProcess    EventType = "pause_process"
	EventViewLogs        EventType = "view_logs"

	// Git events
	EventGitStatus       EventType = "git_status"
	EventGitDiff         EventType = "git_diff"
	EventGitLog          EventType = "git_log"

	// Config events
	EventSaveConfig      EventType = "save_config"
	EventReloadConfig    EventType = "reload_config"

	// Claude events
	EventClaudeCreateSession EventType = "claude_create_session"
	EventClaudeSelectSession EventType = "claude_select_session"
	EventClaudeDeleteSession EventType = "claude_delete_session"
	EventClaudeRenameSession EventType = "claude_rename_session"
	EventClaudeSendMessage   EventType = "claude_send_message"
	EventClaudeStopSession   EventType = "claude_stop_session"
	EventClaudeClearHistory  EventType = "claude_clear_history"
	EventClaudeApprovePlan   EventType = "claude_approve_plan"
	EventClaudeRejectPlan    EventType = "claude_reject_plan"

	// UI state events
	EventFilter          EventType = "filter"
	EventSort            EventType = "sort"
	EventToggle          EventType = "toggle"
	EventScroll          EventType = "scroll"
)

// Event represents a user action in the UI
type Event struct {
	Type      EventType              `json:"type"`
	Target    string                 `json:"target,omitempty"`    // View or element target
	ProjectID string                 `json:"project_id,omitempty"`
	Component projects.ComponentType `json:"component,omitempty"`
	Value     interface{}            `json:"value,omitempty"`     // Generic payload
	Data      map[string]string      `json:"data,omitempty"`      // Additional data
}

// NewEvent creates a new event
func NewEvent(eventType EventType) *Event {
	return &Event{
		Type: eventType,
		Data: make(map[string]string),
	}
}

// WithTarget sets the target
func (e *Event) WithTarget(target string) *Event {
	e.Target = target
	return e
}

// WithProject sets the project ID
func (e *Event) WithProject(projectID string) *Event {
	e.ProjectID = projectID
	return e
}

// WithComponent sets the component
func (e *Event) WithComponent(component projects.ComponentType) *Event {
	e.Component = component
	return e
}

// WithValue sets the value
func (e *Event) WithValue(value interface{}) *Event {
	e.Value = value
	return e
}

// WithData adds data key-value pairs
func (e *Event) WithData(key, value string) *Event {
	if e.Data == nil {
		e.Data = make(map[string]string)
	}
	e.Data[key] = value
	return e
}

// ============================================
// Notification events (from presenter to view)
// ============================================

// NotificationType identifies the type of notification
type NotificationType string

const (
	NotifyInfo    NotificationType = "info"
	NotifySuccess NotificationType = "success"
	NotifyWarning NotificationType = "warning"
	NotifyError   NotificationType = "error"
)

// Notification represents a message to display to the user
type Notification struct {
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	Duration  int              `json:"duration"` // seconds, 0 = persistent
	Dismissable bool           `json:"dismissable"`
}

// NewNotification creates a new notification
func NewNotification(ntype NotificationType, title, message string) *Notification {
	return &Notification{
		Type:        ntype,
		Title:       title,
		Message:     message,
		Duration:    5,
		Dismissable: true,
	}
}

// ============================================
// State update events (from presenter to view)
// ============================================

// StateUpdate represents a state change notification
type StateUpdate struct {
	ViewType   ViewModelType `json:"view_type"`
	ViewModel  ViewModel     `json:"view_model"`
	Partial    bool          `json:"partial"` // If true, merge with existing state
}

// ============================================
// Common event helpers
// ============================================

// NavigateEvent creates a navigation event
func NavigateEvent(target ViewModelType) *Event {
	return NewEvent(EventNavigate).WithTarget(string(target))
}

// BuildEvent creates a build event
func BuildEvent(projectID string, component projects.ComponentType) *Event {
	return NewEvent(EventStartBuild).WithProject(projectID).WithComponent(component)
}

// ProcessEvent creates a process event
func ProcessEvent(eventType EventType, projectID string, component projects.ComponentType) *Event {
	return NewEvent(eventType).WithProject(projectID).WithComponent(component)
}

// FilterEvent creates a filter event
func FilterEvent(filterText string) *Event {
	return NewEvent(EventFilter).WithValue(filterText)
}
