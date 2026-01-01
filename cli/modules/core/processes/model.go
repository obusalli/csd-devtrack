package processes

import (
	"os/exec"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/projects"
)

// ProcessState represents the state of a process
type ProcessState string

const (
	ProcessStateStopped  ProcessState = "stopped"
	ProcessStateStarting ProcessState = "starting"
	ProcessStateRunning  ProcessState = "running"
	ProcessStateStopping ProcessState = "stopping"
	ProcessStateCrashed  ProcessState = "crashed"
)

// Process represents a running or managed process
type Process struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id"`
	Component   projects.ComponentType `json:"component"`
	PID         int                    `json:"pid"`
	State       ProcessState           `json:"state"`
	StartedAt   time.Time              `json:"started_at"`
	StoppedAt   *time.Time             `json:"stopped_at,omitempty"`
	Restarts    int                    `json:"restarts"`
	LastError   string                 `json:"last_error,omitempty"`
	ExitCode    *int                   `json:"exit_code,omitempty"`
	WorkDir     string                 `json:"work_dir"`
	Command     string                 `json:"command"`
	Args        []string               `json:"args"`
	Port        int                    `json:"port,omitempty"`

	// Runtime fields (not serialized)
	cmd       *exec.Cmd  `json:"-"`
	logBuffer *RingBuffer `json:"-"`
	mu        sync.RWMutex `json:"-"`
}

// NewProcess creates a new process
func NewProcess(projectID string, component projects.ComponentType, workDir, command string, args []string) *Process {
	return &Process{
		ID:        generateProcessID(projectID, component),
		ProjectID: projectID,
		Component: component,
		State:     ProcessStateStopped,
		WorkDir:   workDir,
		Command:   command,
		Args:      args,
		logBuffer: NewRingBuffer(10000),
	}
}

// generateProcessID generates a unique process ID
func generateProcessID(projectID string, component projects.ComponentType) string {
	return projectID + "/" + string(component)
}

// SetCmd sets the exec.Cmd for the process
func (p *Process) SetCmd(cmd *exec.Cmd) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cmd = cmd
}

// GetCmd returns the exec.Cmd for the process
func (p *Process) GetCmd() *exec.Cmd {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cmd
}

// SetState sets the process state
func (p *Process) SetState(state ProcessState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = state
}

// GetState returns the process state
func (p *Process) GetState() ProcessState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// SetPID sets the process PID
func (p *Process) SetPID(pid int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.PID = pid
}

// AppendLog appends a log line to the buffer
func (p *Process) AppendLog(line string) {
	p.logBuffer.Write(line)
}

// GetLogs returns the last n log lines
func (p *Process) GetLogs(n int) []string {
	return p.logBuffer.Read(n)
}

// GetAllLogs returns all log lines
func (p *Process) GetAllLogs() []string {
	return p.logBuffer.ReadAll()
}

// IsRunning returns true if the process is running
func (p *Process) IsRunning() bool {
	state := p.GetState()
	return state == ProcessStateRunning || state == ProcessStateStarting
}

// ProcessAction represents an action to perform on a process
type ProcessAction string

const (
	ActionStart     ProcessAction = "start"
	ActionStop      ProcessAction = "stop"      // SIGTERM graceful
	ActionRestart   ProcessAction = "restart"
	ActionKill      ProcessAction = "kill"      // SIGKILL force
	ActionForceKill ProcessAction = "force_kill" // SIGKILL -9 immediate
)

// ProcessEvent represents an event from a process
type ProcessEvent struct {
	Type      ProcessEventType `json:"type"`
	ProcessID string           `json:"process_id"`
	ProjectID string           `json:"project_id"`
	Component string           `json:"component"`
	Message   string           `json:"message"`
	Timestamp time.Time        `json:"timestamp"`
}

// ProcessEventType represents the type of process event
type ProcessEventType string

const (
	ProcessEventStarting   ProcessEventType = "starting"
	ProcessEventStarted    ProcessEventType = "started"
	ProcessEventStopping   ProcessEventType = "stopping"
	ProcessEventStopped    ProcessEventType = "stopped"
	ProcessEventCrashed    ProcessEventType = "crashed"
	ProcessEventRestarting ProcessEventType = "restarting"
	ProcessEventOutput     ProcessEventType = "output"
	ProcessEventError      ProcessEventType = "error"
)

// RingBuffer is a circular buffer for log lines
type RingBuffer struct {
	lines []string
	size  int
	head  int
	count int
	mu    sync.RWMutex
}

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

// Write writes a line to the buffer
func (rb *RingBuffer) Write(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.lines[rb.head] = line
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Read reads the last n lines from the buffer
func (rb *RingBuffer) Read(n int) []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.count {
		n = rb.count
	}

	result := make([]string, n)
	start := (rb.head - n + rb.size) % rb.size

	for i := 0; i < n; i++ {
		result[i] = rb.lines[(start+i)%rb.size]
	}

	return result
}

// ReadAll reads all lines from the buffer
func (rb *RingBuffer) ReadAll() []string {
	return rb.Read(rb.count)
}

// ProcessSummary is a lightweight process summary
type ProcessSummary struct {
	ID        string       `json:"id"`
	ProjectID string       `json:"project_id"`
	Component string       `json:"component"`
	State     ProcessState `json:"state"`
	PID       int          `json:"pid"`
	Uptime    string       `json:"uptime,omitempty"`
	Restarts  int          `json:"restarts"`
}

// ToSummary converts a process to a summary
func (p *Process) ToSummary() *ProcessSummary {
	summary := &ProcessSummary{
		ID:        p.ID,
		ProjectID: p.ProjectID,
		Component: string(p.Component),
		State:     p.GetState(),
		PID:       p.PID,
		Restarts:  p.Restarts,
	}

	if p.State == ProcessStateRunning && !p.StartedAt.IsZero() {
		summary.Uptime = time.Since(p.StartedAt).Round(time.Second).String()
	}

	return summary
}
