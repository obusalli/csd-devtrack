package core

import (
	"sync"
	"time"
)

// AppState represents the global application state
type AppState struct {
	mu sync.RWMutex

	// Current view
	CurrentView ViewModelType

	// View models (cached)
	Dashboard *DashboardVM
	Projects  *ProjectsVM
	Builds    *BuildsVM
	Processes *ProcessesVM
	Logs      *LogsVM
	Git       *GitVM
	Config    *ConfigVM
	Claude    *ClaudeVM
	Cockpit   *CockpitVM
	Database  *DatabaseVM

	// Global state
	IsConnected   bool
	Initializing  bool      // True while presenter is initializing (project loading)
	GitLoading    bool      // True while git info is loading in background
	LastRefresh   time.Time
	Notifications []*Notification

	// Header event (transient status message shown in header center)
	HeaderEvent *HeaderEvent
}

// NewAppState creates a new application state
func NewAppState() *AppState {
	return &AppState{
		CurrentView:  VMDashboard,
		Initializing: true, // Start as initializing until presenter completes
		Dashboard:    &DashboardVM{BaseViewModel: BaseViewModel{VMType: VMDashboard}},
		Projects:     &ProjectsVM{BaseViewModel: BaseViewModel{VMType: VMProjects}},
		Builds:       &BuildsVM{BaseViewModel: BaseViewModel{VMType: VMBuild}},
		Processes:    &ProcessesVM{BaseViewModel: BaseViewModel{VMType: VMProcesses}},
		Logs:         &LogsVM{BaseViewModel: BaseViewModel{VMType: VMLogs}, AutoScroll: true, MaxLines: 1000},
		Git:          &GitVM{BaseViewModel: BaseViewModel{VMType: VMGit}},
		Config:       &ConfigVM{BaseViewModel: BaseViewModel{VMType: VMConfig}},
		Claude:       &ClaudeVM{BaseViewModel: BaseViewModel{VMType: VMClaude}},
		Cockpit:      &CockpitVM{BaseViewModel: BaseViewModel{VMType: VMCockpit}},
		Database:     &DatabaseVM{BaseViewModel: BaseViewModel{VMType: VMDatabase}},
		Notifications: make([]*Notification, 0),
	}
}

// GetCurrentViewModel returns the view model for the current view
func (s *AppState) GetCurrentViewModel() ViewModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch s.CurrentView {
	case VMDashboard:
		return s.Dashboard
	case VMProjects:
		return s.Projects
	case VMBuild:
		return s.Builds
	case VMProcesses:
		return s.Processes
	case VMLogs:
		return s.Logs
	case VMGit:
		return s.Git
	case VMConfig:
		return s.Config
	case VMClaude:
		return s.Claude
	case VMCockpit:
		return s.Cockpit
	case VMDatabase:
		return s.Database
	default:
		return s.Dashboard
	}
}

// SetCurrentView changes the current view
func (s *AppState) SetCurrentView(view ViewModelType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentView = view
}

// UpdateViewModel updates a specific view model
func (s *AppState) UpdateViewModel(vm ViewModel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch v := vm.(type) {
	case *DashboardVM:
		s.Dashboard = v
	case *ProjectsVM:
		s.Projects = v
	case *BuildsVM:
		s.Builds = v
	case *ProcessesVM:
		s.Processes = v
	case *LogsVM:
		s.Logs = v
	case *GitVM:
		s.Git = v
	case *ConfigVM:
		s.Config = v
	case *ClaudeVM:
		s.Claude = v
	case *CockpitVM:
		s.Cockpit = v
	case *DatabaseVM:
		s.Database = v
	}
}

// AddNotification adds a notification
func (s *AppState) AddNotification(n *Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Notifications = append(s.Notifications, n)
}

// RemoveNotification removes a notification
func (s *AppState) RemoveNotification(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= 0 && index < len(s.Notifications) {
		s.Notifications = append(s.Notifications[:index], s.Notifications[index+1:]...)
	}
}

// ClearNotifications clears all notifications
func (s *AppState) ClearNotifications() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Notifications = make([]*Notification, 0)
}

// SetHeaderEvent sets the current header event
func (s *AppState) SetHeaderEvent(event *HeaderEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HeaderEvent = event
}

// ClearHeaderEvent clears the current header event
func (s *AppState) ClearHeaderEvent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HeaderEvent = nil
}

// GetHeaderEvent returns the current header event
func (s *AppState) GetHeaderEvent() *HeaderEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HeaderEvent
}

// ============================================
// State selectors (for views to query state)
// ============================================

// Selector is a function that extracts data from state
type Selector[T any] func(state *AppState) T

// SelectProjects returns all projects
func SelectProjects(state *AppState) []ProjectVM {
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.Projects == nil {
		return nil
	}
	return state.Projects.Projects
}

// SelectRunningProcesses returns running processes
func SelectRunningProcesses(state *AppState) []ProcessVM {
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.Processes == nil {
		return nil
	}

	var running []ProcessVM
	for _, p := range state.Processes.Processes {
		if p.State == "running" {
			running = append(running, p)
		}
	}
	return running
}

// SelectProjectByID returns a project by ID
func SelectProjectByID(state *AppState, projectID string) *ProjectVM {
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.Projects == nil {
		return nil
	}

	for _, p := range state.Projects.Projects {
		if p.ID == projectID {
			return &p
		}
	}
	return nil
}

// SelectNotifications returns all notifications
func SelectNotifications(state *AppState) []*Notification {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.Notifications
}
