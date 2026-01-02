package daemon

import (
	"context"
	"fmt"
	"sync"

	"csd-devtrack/cli/modules/ui/core"
)

// ClientPresenter implements the Presenter interface for daemon clients
// It forwards events to the daemon and receives state updates
type ClientPresenter struct {
	mu     sync.RWMutex
	client *Client
	state  *core.AppState

	// Callbacks
	stateCallbacks        []func(core.StateUpdate)
	notificationCallbacks []func(*core.Notification)
	tuiStateCallback      func(*TUIState) // Called when TUI state should be restored
	pendingTUIState       *TUIState       // Buffered if received before callback is set

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClientPresenter creates a presenter that communicates with the daemon
func NewClientPresenter(client *Client) *ClientPresenter {
	return &ClientPresenter{
		client:                client,
		state:                 core.NewAppState(),
		stateCallbacks:        make([]func(core.StateUpdate), 0),
		notificationCallbacks: make([]func(*core.Notification), 0),
	}
}

// Initialize sets up the client presenter
func (p *ClientPresenter) Initialize(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Set up handlers to receive updates from daemon
	p.client.SetStateHandler(func(state *core.AppState) {
		p.handleStateUpdate(state)
	})

	p.client.SetNotifyHandler(func(n *core.Notification) {
		p.handleNotification(n)
	})

	p.client.SetLogHandler(func(line core.LogLineVM) {
		p.handleLogLine(line)
	})

	// Set up TUI state handler for restoration on reattach
	p.client.SetTUIStateHandler(func(state *TUIState) {
		p.mu.Lock()
		callback := p.tuiStateCallback
		if callback == nil {
			// Buffer the state if callback not yet set
			p.pendingTUIState = state
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
		callback(state)
	})

	// Request initial state from daemon
	return p.client.RequestState()
}

// SetTUIStateCallback sets the callback for TUI state restoration
// If TUI state was already received, calls the callback immediately
func (p *ClientPresenter) SetTUIStateCallback(callback func(*TUIState)) {
	p.mu.Lock()
	p.tuiStateCallback = callback
	pendingState := p.pendingTUIState
	p.pendingTUIState = nil
	p.mu.Unlock()

	// If we had a pending state, apply it now
	if pendingState != nil && callback != nil {
		callback(pendingState)
	}
}

// SaveTUIState saves the TUI state to the daemon before detaching
func (p *ClientPresenter) SaveTUIState(state *TUIState) error {
	return p.client.SaveTUIState(state)
}

// HandleEvent forwards an event to the daemon
func (p *ClientPresenter) HandleEvent(event *core.Event) error {
	return p.client.SendEvent(event)
}

// GetViewModel returns the current view model for a view type
func (p *ClientPresenter) GetViewModel(viewType core.ViewModelType) (core.ViewModel, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.state == nil {
		return nil, fmt.Errorf("state not initialized")
	}

	switch viewType {
	case core.VMDashboard:
		return p.state.Dashboard, nil
	case core.VMProjects:
		return p.state.Projects, nil
	case core.VMBuild:
		return p.state.Builds, nil
	case core.VMProcesses:
		return p.state.Processes, nil
	case core.VMLogs:
		return p.state.Logs, nil
	case core.VMGit:
		return p.state.Git, nil
	case core.VMConfig:
		return p.state.Config, nil
	default:
		return nil, fmt.Errorf("unknown view type: %s", viewType)
	}
}

// Subscribe registers a callback for state updates
// If state is already available, sends it immediately
func (p *ClientPresenter) Subscribe(callback func(core.StateUpdate)) {
	p.mu.Lock()
	p.stateCallbacks = append(p.stateCallbacks, callback)
	state := p.state
	p.mu.Unlock()

	// Send ALL view models immediately if state available
	if state != nil {
		viewModels := []struct {
			viewType core.ViewModelType
			vm       core.ViewModel
		}{
			{core.VMDashboard, state.Dashboard},
			{core.VMProjects, state.Projects},
			{core.VMBuild, state.Builds},
			{core.VMProcesses, state.Processes},
			{core.VMGit, state.Git},
			{core.VMLogs, state.Logs},
			{core.VMConfig, state.Config},
		}
		for _, v := range viewModels {
			if v.vm != nil {
				callback(core.StateUpdate{
					ViewType:  v.viewType,
					ViewModel: v.vm,
				})
			}
		}
	}
}

// SubscribeNotifications registers a callback for notifications
func (p *ClientPresenter) SubscribeNotifications(callback func(*core.Notification)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notificationCallbacks = append(p.notificationCallbacks, callback)
}

// Refresh requests a state refresh from the daemon
func (p *ClientPresenter) Refresh() error {
	return p.client.RequestState()
}

// Shutdown disconnects from the daemon
func (p *ClientPresenter) Shutdown() error {
	if p.cancel != nil {
		p.cancel()
	}
	// Note: Don't disconnect client here - it stays connected for reattach
	return nil
}

// GetState returns the full application state
func (p *ClientPresenter) GetState() *core.AppState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// Disconnect disconnects from the daemon (for detach)
func (p *ClientPresenter) Disconnect() {
	p.client.Disconnect()
}

// handleStateUpdate processes a full state update from the daemon
func (p *ClientPresenter) handleStateUpdate(state *core.AppState) {
	p.mu.Lock()
	// Preserve existing logs - they are managed separately via MsgLog
	if p.state != nil && p.state.Logs != nil && len(p.state.Logs.Lines) > 0 {
		state.Logs = p.state.Logs
	}
	p.state = state
	p.mu.Unlock()

	p.mu.RLock()
	callbacks := p.stateCallbacks
	p.mu.RUnlock()

	// Notify with ALL view models since this is a full state sync
	viewModels := []struct {
		viewType core.ViewModelType
		vm       core.ViewModel
	}{
		{core.VMDashboard, state.Dashboard},
		{core.VMProjects, state.Projects},
		{core.VMBuild, state.Builds},
		{core.VMProcesses, state.Processes},
		{core.VMGit, state.Git},
		{core.VMLogs, state.Logs},
		{core.VMConfig, state.Config},
	}

	for _, v := range viewModels {
		if v.vm != nil {
			update := core.StateUpdate{
				ViewType:  v.viewType,
				ViewModel: v.vm,
			}
			for _, cb := range callbacks {
				cb(update)
			}
		}
	}
}

// handleNotification processes a notification from the daemon
func (p *ClientPresenter) handleNotification(n *core.Notification) {
	p.mu.Lock()
	p.state.AddNotification(n)
	p.mu.Unlock()

	p.mu.RLock()
	callbacks := p.notificationCallbacks
	p.mu.RUnlock()

	for _, cb := range callbacks {
		cb(n)
	}
}

// handleLogLine processes a log line from the daemon
func (p *ClientPresenter) handleLogLine(line core.LogLineVM) {
	p.mu.Lock()
	p.state.Logs.Lines = append(p.state.Logs.Lines, line)
	if len(p.state.Logs.Lines) > p.state.Logs.MaxLines {
		p.state.Logs.Lines = p.state.Logs.Lines[1:]
	}
	p.mu.Unlock()

	// Notify logs view
	update := core.StateUpdate{
		ViewType:  core.VMLogs,
		ViewModel: p.state.Logs,
	}

	p.mu.RLock()
	callbacks := p.stateCallbacks
	p.mu.RUnlock()

	for _, cb := range callbacks {
		cb(update)
	}
}
