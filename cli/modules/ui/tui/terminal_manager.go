package tui

import (
	"sync"
)

// TerminalInterface defines the interface for terminal implementations
type TerminalInterface interface {
	SetSize(width, height int)
	Start(sessionID string) error
	Write(data []byte) error
	WriteString(s string) error
	HandleKey(key string) (consumed bool, exitTerminal bool)
	View() string
	State() TerminalState
	IsRunning() bool
	Stop()
	SetCallbacks(onOutput, onExit func())
	LineCount() int
	Width() int
	Height() int
	GetLines() []string
}

// TerminalManager manages multiple terminal sessions
type TerminalManager struct {
	mu         sync.RWMutex
	terminals  map[string]TerminalInterface // sessionID -> Terminal
	claudePath string
}

// NewTerminalManager creates a new terminal manager
func NewTerminalManager(claudePath string) *TerminalManager {
	return &TerminalManager{
		terminals:  make(map[string]TerminalInterface),
		claudePath: claudePath,
	}
}

// GetOrCreate gets an existing terminal or creates a new one (for Claude)
func (tm *TerminalManager) GetOrCreate(sessionID, workDir string) TerminalInterface {
	return tm.GetOrCreateWithPrefix(sessionID, workDir, TmuxPrefixClaude)
}

// GetOrCreateWithPrefix gets an existing terminal or creates a new one with a specific prefix
func (tm *TerminalManager) GetOrCreateWithPrefix(sessionID, workDir, prefix string) TerminalInterface {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, exists := tm.terminals[sessionID]; exists {
		return t
	}

	// Use tmux-based terminal (persistent, captures ANSI colors with capture-pane -e)
	t := NewTerminalTmuxWithPrefix(sessionID, workDir, tm.claudePath, prefix)
	tm.terminals[sessionID] = t
	return t
}

// GetOrCreateWithCommand gets an existing terminal or creates a new one with a custom command (for database)
func (tm *TerminalManager) GetOrCreateWithCommand(sessionID, command string, args []string) TerminalInterface {
	return tm.GetOrCreateCommandWithPrefix(sessionID, command, args, TmuxPrefixDatabase)
}

// GetOrCreateCommandWithPrefix gets an existing terminal or creates a new one with a custom command and prefix
func (tm *TerminalManager) GetOrCreateCommandWithPrefix(sessionID, command string, args []string, prefix string) TerminalInterface {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, exists := tm.terminals[sessionID]; exists {
		return t
	}

	// Use generic tmux-based terminal with custom command
	t := NewTerminalTmuxCommandWithPrefix(sessionID, command, args, prefix)
	tm.terminals[sessionID] = t
	return t
}

// Get returns an existing terminal or nil
func (tm *TerminalManager) Get(sessionID string) TerminalInterface {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.terminals[sessionID]
}

// Remove removes a terminal
func (tm *TerminalManager) Remove(sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, exists := tm.terminals[sessionID]; exists {
		t.Stop()
		delete(tm.terminals, sessionID)
	}
}

// GetRunning returns all running terminal session IDs
func (tm *TerminalManager) GetRunning() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var running []string
	for id, t := range tm.terminals {
		if t.IsRunning() {
			running = append(running, id)
		}
	}
	return running
}

// StopAll stops all terminals
func (tm *TerminalManager) StopAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range tm.terminals {
		t.Stop()
	}
	tm.terminals = make(map[string]TerminalInterface)
}

// SetClaudePath updates the claude path for new terminals
func (tm *TerminalManager) SetClaudePath(path string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.claudePath = path
}

// Count returns the number of terminals
func (tm *TerminalManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.terminals)
}

// RunningCount returns the number of running terminals
func (tm *TerminalManager) RunningCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	count := 0
	for _, t := range tm.terminals {
		if t.IsRunning() {
			count++
		}
	}
	return count
}
