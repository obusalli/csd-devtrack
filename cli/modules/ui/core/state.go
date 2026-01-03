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
	Dashboard    *DashboardVM
	Projects     *ProjectsVM
	Builds       *BuildsVM
	Processes    *ProcessesVM
	Logs         *LogsVM
	Git          *GitVM
	Config       *ConfigVM
	Claude       *ClaudeVM
	Cockpit      *CockpitVM
	Database     *DatabaseVM
	Capabilities *CapabilitiesVM

	// Global state
	IsConnected   bool
	Initializing  bool      // True while presenter is initializing (project loading)
	GitLoading    bool      // True while git info is loading in background
	LastRefresh   time.Time
	Notifications []*Notification

	// Header events queue (ticker-style scrolling in header center)
	HeaderEvents []*HeaderEvent
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
		Cockpit:       &CockpitVM{BaseViewModel: BaseViewModel{VMType: VMCockpit}},
		Database:      &DatabaseVM{BaseViewModel: BaseViewModel{VMType: VMDatabase}},
		Capabilities:  &CapabilitiesVM{},
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

// SetHeaderEvent adds a header event to the queue (for ticker display)
func (s *AppState) SetHeaderEvent(event *HeaderEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Always remove persistent events when any new event arrives
	// (persistent events show "in progress" states that are now complete)
	if len(s.HeaderEvents) > 0 {
		filtered := make([]*HeaderEvent, 0, len(s.HeaderEvents))
		for _, e := range s.HeaderEvents {
			if !e.Persistent {
				filtered = append(filtered, e)
			}
		}
		s.HeaderEvents = filtered
	}

	// Don't add duplicate of the most recent event (same message)
	if len(s.HeaderEvents) > 0 {
		lastEvent := s.HeaderEvents[len(s.HeaderEvents)-1]
		if lastEvent.Message == event.Message {
			// Same message - update timestamp instead of adding duplicate
			lastEvent.CreatedAt = event.CreatedAt
			return
		}
	}

	// Add to queue (max 10 events)
	s.HeaderEvents = append(s.HeaderEvents, event)
	if len(s.HeaderEvents) > 10 {
		s.HeaderEvents = s.HeaderEvents[len(s.HeaderEvents)-10:]
	}
}

// ClearHeaderEvent clears all header events
func (s *AppState) ClearHeaderEvent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HeaderEvents = nil
}

// ClearExpiredHeaderEvents removes expired events from the queue
func (s *AppState) ClearExpiredHeaderEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.HeaderEvents) == 0 {
		return
	}

	filtered := make([]*HeaderEvent, 0, len(s.HeaderEvents))
	for _, e := range s.HeaderEvents {
		if !e.IsExpired() {
			filtered = append(filtered, e)
		}
	}
	s.HeaderEvents = filtered
}

// GetHeaderEvent returns the first non-expired header event (for compatibility)
func (s *AppState) GetHeaderEvent() *HeaderEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.HeaderEvents {
		if !e.IsExpired() {
			return e
		}
	}
	return nil
}

// GetHeaderEvents returns all non-expired header events
func (s *AppState) GetHeaderEvents() []*HeaderEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*HeaderEvent, 0, len(s.HeaderEvents))
	for _, e := range s.HeaderEvents {
		if !e.IsExpired() {
			result = append(result, e)
		}
	}
	return result
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
