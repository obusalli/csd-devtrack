package core

import (
	"context"
)

// View is the interface that all UI implementations must satisfy
// This is what TUI, WebUI, and GUI implementations will implement
type View interface {
	// Initialize sets up the view
	Initialize(presenter Presenter) error

	// Run starts the view's main loop (blocking)
	Run(ctx context.Context) error

	// Stop gracefully stops the view
	Stop() error

	// UpdateState updates the view with new state
	UpdateState(update StateUpdate)

	// ShowNotification displays a notification
	ShowNotification(notification *Notification)

	// GetCurrentView returns the current active view type
	GetCurrentView() ViewModelType
}

// Presenter handles the business logic and prepares view models
// It's the bridge between the domain services and the views
type Presenter interface {
	// Initialize sets up the presenter with services
	Initialize(ctx context.Context) error

	// HandleEvent processes a user event
	HandleEvent(event *Event) error

	// GetViewModel returns the current view model for a view type
	GetViewModel(viewType ViewModelType) (ViewModel, error)

	// Subscribe registers a callback for state updates
	Subscribe(callback func(StateUpdate))

	// SubscribeNotifications registers a callback for notifications
	SubscribeNotifications(callback func(*Notification))

	// Refresh forces a refresh of all data
	Refresh() error

	// Shutdown cleans up resources
	Shutdown() error
}

// ViewFactory creates views of different types
type ViewFactory interface {
	// CreateView creates a view of the specified type
	CreateView(viewType string, presenter Presenter) (View, error)

	// AvailableTypes returns the list of available view types
	AvailableTypes() []string
}

// ============================================
// View-specific interfaces (optional, for type safety)
// ============================================

// DashboardView is the interface for dashboard-specific functionality
type DashboardView interface {
	View
	SetSummaryStats(projects, running, building, errors int)
}

// ProjectsView is the interface for projects-specific functionality
type ProjectsView interface {
	View
	SetSelectedProject(projectID string)
	GetSelectedProject() string
}

// BuildView is the interface for build-specific functionality
type BuildView interface {
	View
	SetBuildProgress(progress int, output []string)
	SetBuildComplete(success bool, errors []string)
}

// ProcessesView is the interface for processes-specific functionality
type ProcessesView interface {
	View
	SetProcessState(processID string, state string)
	AppendLog(processID string, line string)
}

// LogsView is the interface for logs-specific functionality
type LogsView interface {
	View
	AppendLines(lines []LogLineVM)
	Clear()
	SetFilter(project, component, level string)
}

// GitView is the interface for git-specific functionality
type GitView interface {
	View
	SetDiff(diff string)
	SetCommits(commits []CommitVM)
}

// ConfigView is the interface for config-specific functionality
type ConfigView interface {
	View
	SetEditMode(editing bool)
	GetEditedConfig() map[string]interface{}
}

// ============================================
// Application interface
// ============================================

// Application is the main entry point for the UI
type Application interface {
	// Start starts the application with the specified view type
	Start(ctx context.Context, viewType string) error

	// Stop stops the application
	Stop() error

	// SetView switches to a different view type
	SetView(viewType string) error

	// GetPresenter returns the presenter
	GetPresenter() Presenter

	// GetCurrentView returns the current view
	GetCurrentView() View
}
