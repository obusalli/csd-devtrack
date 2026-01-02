package tui

import (
	"context"
	"fmt"
	"sync"

	"csd-devtrack/cli/modules/platform/daemon"
	"csd-devtrack/cli/modules/ui/core"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIView implements the core.View interface for Bubble Tea TUI
type TUIView struct {
	mu              sync.RWMutex
	presenter       core.Presenter
	program         *tea.Program
	model           *Model
	ctx             context.Context
	cancel          context.CancelFunc
	detachable      bool              // If true, Ctrl+D detaches instead of quit
	detached        bool              // Set to true if user detached
	pendingTUIState *daemon.TUIState  // Buffered TUI state if received before program starts
	pendingUpdates  []core.StateUpdate // Buffered state updates if received before program starts
}

// NewTUIView creates a new TUI view
func NewTUIView() *TUIView {
	return &TUIView{}
}

// SetDetachable enables or disables Ctrl+D detach (daemon mode)
func (v *TUIView) SetDetachable(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.detachable = enabled
}

// WasDetached returns true if the user detached (Ctrl+D) instead of quit
func (v *TUIView) WasDetached() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.detached
}

// ExportTUIState exports the current TUI state for daemon persistence
func (v *TUIView) ExportTUIState() *daemon.TUIState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.model != nil {
		return v.model.ExportTUIState()
	}
	return nil
}

// ImportTUIState restores TUI state from daemon persistence
// This sends a message to the running tea.Program to restore state
func (v *TUIView) ImportTUIState(state *daemon.TUIState) {
	if state == nil {
		return
	}

	v.mu.Lock()
	program := v.program
	if program == nil {
		// Program not started yet, buffer the state
		v.pendingTUIState = state
		v.mu.Unlock()
		return
	}
	v.mu.Unlock()

	// Send via the tea.Program message queue
	program.Send(tuiStateRestoreMsg{state: state})
}

// SetStateRestoreCallback sets a callback to be called after state is restored
func (v *TUIView) SetStateRestoreCallback(callback func()) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.model != nil {
		v.model.SetStateRestoreCallback(callback)
	}
}

// Initialize sets up the view with a presenter
func (v *TUIView) Initialize(presenter core.Presenter) error {
	v.mu.Lock()
	v.presenter = presenter
	v.model = NewModel(presenter)
	v.model.detachable = v.detachable
	v.mu.Unlock()

	// Subscribe to state updates (must be outside lock - callback may call UpdateState)
	presenter.Subscribe(func(update core.StateUpdate) {
		v.UpdateState(update)
	})

	// Subscribe to notifications
	presenter.SubscribeNotifications(func(n *core.Notification) {
		v.ShowNotification(n)
	})

	return nil
}

// Run starts the TUI main loop (blocking)
func (v *TUIView) Run(ctx context.Context) error {
	v.mu.Lock()
	v.ctx, v.cancel = context.WithCancel(ctx)
	v.detached = false // Reset detached state for this run
	pendingState := v.pendingTUIState
	v.pendingTUIState = nil
	pendingUpdates := v.pendingUpdates
	v.pendingUpdates = nil
	v.mu.Unlock()

	// Create the program
	v.mu.Lock()
	v.model.detached = false // Ensure model starts with detached=false
	v.program = tea.NewProgram(
		v.model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	program := v.program
	v.mu.Unlock()

	// Channel to receive final model and error from program.Run()
	type runResult struct {
		model tea.Model
		err   error
	}
	resultCh := make(chan runResult, 1)
	go func() {
		finalModel, err := program.Run()
		resultCh <- runResult{model: finalModel, err: err}
	}()

	// Apply pending state updates after program starts
	for _, update := range pendingUpdates {
		program.Send(stateUpdateMsg{update: update})
	}

	// Apply pending TUI state if we had one buffered (for reattach)
	if pendingState != nil {
		program.Send(tuiStateRestoreMsg{state: pendingState})
	}

	// Wait for either context cancellation or program exit
	select {
	case <-v.ctx.Done():
		v.program.Quit()
		return v.ctx.Err()
	case result := <-resultCh:
		// Get the final model state from Bubble Tea (it works with copies)
		v.mu.Lock()
		if finalModel, ok := result.model.(Model); ok {
			v.model = &finalModel
			v.detached = finalModel.detached
		} else if finalModelPtr, ok := result.model.(*Model); ok {
			v.model = finalModelPtr
			v.detached = finalModelPtr.detached
		} else {
			// Type assertion failed - default to not detached (quit)
			v.detached = false
		}
		v.mu.Unlock()
		return result.err
	}
}

// Stop gracefully stops the TUI
func (v *TUIView) Stop() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.cancel != nil {
		v.cancel()
	}
	if v.program != nil {
		v.program.Quit()
	}
	return nil
}

// UpdateState updates the view with new state from the presenter
func (v *TUIView) UpdateState(update core.StateUpdate) {
	v.mu.Lock()
	program := v.program
	if program == nil {
		// Buffer if program not started yet
		v.pendingUpdates = append(v.pendingUpdates, update)
		v.mu.Unlock()
		return
	}
	v.mu.Unlock()

	program.Send(stateUpdateMsg{update: update})
}

// ShowNotification displays a notification
func (v *TUIView) ShowNotification(notification *core.Notification) {
	v.mu.RLock()
	program := v.program
	v.mu.RUnlock()

	if program != nil {
		program.Send(notificationMsg{notification: notification})
	}
}

// GetCurrentView returns the current active view type
func (v *TUIView) GetCurrentView() core.ViewModelType {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.model != nil {
		return v.model.currentView
	}
	return core.VMDashboard
}

// ===========================================
// ViewFactory implementation
// ===========================================

// TUIFactory creates TUI views
type TUIFactory struct{}

// NewTUIFactory creates a new TUI factory
func NewTUIFactory() *TUIFactory {
	return &TUIFactory{}
}

// CreateView creates a TUI view
func (f *TUIFactory) CreateView(_ string, presenter core.Presenter) (core.View, error) {
	view := NewTUIView()
	if err := view.Initialize(presenter); err != nil {
		return nil, fmt.Errorf("failed to initialize TUI view: %w", err)
	}
	return view, nil
}

// AvailableTypes returns the available view types
func (f *TUIFactory) AvailableTypes() []string {
	return []string{"tui"}
}
