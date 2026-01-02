package supervisor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
)

// Manager supervises processes
type Manager struct {
	processService *processes.Service
	mu             sync.RWMutex
	stopTimeout    time.Duration
}

// NewManager creates a new process manager
func NewManager(processService *processes.Service) *Manager {
	return &Manager{
		processService: processService,
		stopTimeout:    30 * time.Second,
	}
}

// Start starts a component process
func (m *Manager) Start(ctx context.Context, project *projects.Project, component *projects.Component) (*processes.Process, error) {
	workDir := filepath.Join(project.Path, component.Path)
	if component.Path == "" {
		workDir = project.Path
	}

	// Determine command and args
	command, args := m.buildCommand(project, component)

	// Create process
	proc := processes.NewProcess(project.ID, component.Type, workDir, command, args)
	proc.Port = component.Port
	proc.SetState(processes.ProcessStateStarting)

	// Emit starting event
	m.emitEvent(processes.ProcessEvent{
		Type:      processes.ProcessEventStarting,
		ProcessID: proc.ID,
		ProjectID: project.ID,
		Component: string(component.Type),
		Message:   fmt.Sprintf("Starting %s/%s", project.ID, component.Type),
		Timestamp: time.Now(),
	})

	// Create exec.Cmd
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	cmd.Env = m.buildEnvironment(project, component)

	// Set up process group for proper signal handling
	m.setupProcessGroup(cmd)

	// Set up pipes for stdout/stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		proc.SetState(processes.ProcessStateCrashed)
		proc.LastError = err.Error()
		return proc, fmt.Errorf("failed to start process: %w", err)
	}

	proc.SetCmd(cmd)
	proc.SetPID(cmd.Process.Pid)
	proc.StartedAt = time.Now()
	proc.SetState(processes.ProcessStateRunning)

	// Emit started event
	m.emitEvent(processes.ProcessEvent{
		Type:      processes.ProcessEventStarted,
		ProcessID: proc.ID,
		ProjectID: project.ID,
		Component: string(component.Type),
		Message:   fmt.Sprintf("Started %s/%s (PID: %d)", project.ID, component.Type, proc.PID),
		Timestamp: time.Now(),
	})

	// Start log readers
	go m.readOutput(proc, stdout, false)
	go m.readOutput(proc, stderr, true)

	// Monitor process
	go m.monitor(proc)

	return proc, nil
}

// buildCommand builds the command and arguments for a component
func (m *Manager) buildCommand(project *projects.Project, component *projects.Component) (string, []string) {
	// If custom run command is specified, use it
	if component.RunCmd != "" {
		if runtime.GOOS == "windows" {
			return "cmd", []string{"/c", component.RunCmd}
		}
		return "sh", []string{"-c", component.RunCmd}
	}

	// Default commands based on component type
	switch component.Type {
	case projects.ComponentFrontend:
		args := []string{"run", "dev"}
		if len(component.Args) > 0 {
			args = append(args, "--")
			args = append(args, component.Args...)
		}
		return "npm", args

	case projects.ComponentCLI, projects.ComponentBackend, projects.ComponentAgent:
		// Use go run with entry point file
		// Working directory is already set to component path
		entryPoint := component.EntryPoint
		if entryPoint == "" {
			entryPoint = "."
		}
		args := []string{"run", entryPoint}
		// Append component-specific arguments from config
		if len(component.Args) > 0 {
			args = append(args, component.Args...)
		}
		return "go", args
	}

	return "", nil
}

// buildEnvironment builds the environment for a component
func (m *Manager) buildEnvironment(project *projects.Project, component *projects.Component) []string {
	env := os.Environ()

	// Add component-specific environment
	env = append(env, fmt.Sprintf("CSD_PROJECT=%s", project.ID))
	env = append(env, fmt.Sprintf("CSD_COMPONENT=%s", component.Type))

	if component.Port > 0 {
		env = append(env, fmt.Sprintf("PORT=%d", component.Port))
	}

	return env
}

// Stop stops a process gracefully
func (m *Manager) Stop(ctx context.Context, proc *processes.Process, force bool) error {
	if !proc.IsRunning() {
		return nil
	}

	proc.SetState(processes.ProcessStateStopping)

	// Emit stopping event
	m.emitEvent(processes.ProcessEvent{
		Type:      processes.ProcessEventStopping,
		ProcessID: proc.ID,
		ProjectID: proc.ProjectID,
		Component: string(proc.Component),
		Message:   fmt.Sprintf("Stopping %s", proc.ID),
		Timestamp: time.Now(),
	})

	cmd := proc.GetCmd()
	if cmd == nil || cmd.Process == nil {
		proc.SetState(processes.ProcessStateStopped)
		return nil
	}

	if force {
		return m.Kill(proc)
	}

	// Send SIGTERM (or equivalent)
	if err := m.signalProcess(cmd, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(m.stopTimeout):
		// Timeout - force kill
		return m.Kill(proc)
	case err := <-done:
		now := time.Now()
		proc.StoppedAt = &now
		proc.SetState(processes.ProcessStateStopped)

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode := exitErr.ExitCode()
				proc.ExitCode = &exitCode
			}
		}

		// Emit stopped event
		m.emitEvent(processes.ProcessEvent{
			Type:      processes.ProcessEventStopped,
			ProcessID: proc.ID,
			ProjectID: proc.ProjectID,
			Component: string(proc.Component),
			Message:   fmt.Sprintf("Stopped %s", proc.ID),
			Timestamp: time.Now(),
		})

		return nil
	}
}

// Kill forcefully kills a process
func (m *Manager) Kill(proc *processes.Process) error {
	cmd := proc.GetCmd()
	if cmd == nil || cmd.Process == nil {
		proc.SetState(processes.ProcessStateStopped)
		return nil
	}

	if err := m.killProcess(cmd); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	now := time.Now()
	proc.StoppedAt = &now
	proc.SetState(processes.ProcessStateStopped)

	// Emit stopped event
	m.emitEvent(processes.ProcessEvent{
		Type:      processes.ProcessEventStopped,
		ProcessID: proc.ID,
		ProjectID: proc.ProjectID,
		Component: string(proc.Component),
		Message:   fmt.Sprintf("Killed %s", proc.ID),
		Timestamp: time.Now(),
	})

	return nil
}

// Signal sends a signal to a process
func (m *Manager) Signal(proc *processes.Process, sig int) error {
	cmd := proc.GetCmd()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	return m.signalProcess(cmd, syscall.Signal(sig))
}

// Pause pauses a running process using SIGSTOP
func (m *Manager) Pause(proc *processes.Process) error {
	cmd := proc.GetCmd()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if proc.IsPaused() {
		return nil // Already paused
	}

	if err := m.signalProcess(cmd, syscall.SIGSTOP); err != nil {
		return fmt.Errorf("failed to pause process: %w", err)
	}

	proc.SetState(processes.ProcessStatePaused)

	// Emit paused event
	m.emitEvent(processes.ProcessEvent{
		Type:      processes.ProcessEventPaused,
		ProcessID: proc.ID,
		ProjectID: proc.ProjectID,
		Component: string(proc.Component),
		Message:   fmt.Sprintf("Paused %s", proc.ID),
		Timestamp: time.Now(),
	})

	return nil
}

// Resume resumes a paused process using SIGCONT
func (m *Manager) Resume(proc *processes.Process) error {
	cmd := proc.GetCmd()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if !proc.IsPaused() {
		return nil // Not paused
	}

	if err := m.signalProcess(cmd, syscall.SIGCONT); err != nil {
		return fmt.Errorf("failed to resume process: %w", err)
	}

	proc.SetState(processes.ProcessStateRunning)

	// Emit resumed event
	m.emitEvent(processes.ProcessEvent{
		Type:      processes.ProcessEventResumed,
		ProcessID: proc.ID,
		ProjectID: proc.ProjectID,
		Component: string(proc.Component),
		Message:   fmt.Sprintf("Resumed %s", proc.ID),
		Timestamp: time.Now(),
	})

	return nil
}

// TogglePause toggles pause state of a process
func (m *Manager) TogglePause(proc *processes.Process) error {
	if proc.IsPaused() {
		return m.Resume(proc)
	}
	return m.Pause(proc)
}

// monitor monitors a process and handles crashes
func (m *Manager) monitor(proc *processes.Process) {
	cmd := proc.GetCmd()
	if cmd == nil {
		return
	}

	// Wait for process to exit
	err := cmd.Wait()

	// Process has exited
	now := time.Now()
	proc.StoppedAt = &now

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			proc.ExitCode = &exitCode

			// If we weren't stopping it intentionally, it crashed
			if proc.GetState() == processes.ProcessStateRunning {
				proc.SetState(processes.ProcessStateCrashed)
				proc.LastError = fmt.Sprintf("Process exited with code %d", exitCode)

				// Emit crashed event
				m.emitEvent(processes.ProcessEvent{
					Type:      processes.ProcessEventCrashed,
					ProcessID: proc.ID,
					ProjectID: proc.ProjectID,
					Component: string(proc.Component),
					Message:   fmt.Sprintf("Crashed %s (exit code: %d)", proc.ID, exitCode),
					Timestamp: time.Now(),
				})
			}
		}
	} else {
		exitCode := 0
		proc.ExitCode = &exitCode
		proc.SetState(processes.ProcessStateStopped)
	}
}

// readOutput reads output from a pipe and stores it in the process log buffer
func (m *Manager) readOutput(proc *processes.Process, pipe interface{ Read([]byte) (int, error) }, isStderr bool) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		proc.AppendLog(line)

		// Emit output event
		eventType := processes.ProcessEventOutput
		if isStderr {
			eventType = processes.ProcessEventError
		}

		m.emitEvent(processes.ProcessEvent{
			Type:      eventType,
			ProcessID: proc.ID,
			ProjectID: proc.ProjectID,
			Component: string(proc.Component),
			Message:   line,
			Timestamp: time.Now(),
		})
	}
}

// emitEvent emits a process event
func (m *Manager) emitEvent(event processes.ProcessEvent) {
	if m.processService != nil {
		m.processService.EmitEvent(event)
	}
}

// SetStopTimeout sets the stop timeout
func (m *Manager) SetStopTimeout(timeout time.Duration) {
	m.stopTimeout = timeout
}

// GetProcessService returns the process service
func (m *Manager) GetProcessService() *processes.Service {
	return m.processService
}
