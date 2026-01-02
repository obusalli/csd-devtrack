package processes

import (
	"context"
	"fmt"
	"sync"

	"csd-devtrack/cli/modules/core/projects"
)

// ProcessHandler is a function that handles process events
type ProcessHandler func(event ProcessEvent)

// Service manages processes
type Service struct {
	projectService *projects.Service
	processes      map[string]*Process
	mu             sync.RWMutex
	eventHandler   ProcessHandler
}

// NewService creates a new process service
func NewService(projectService *projects.Service) *Service {
	return &Service{
		projectService: projectService,
		processes:      make(map[string]*Process),
	}
}

// SetEventHandler sets the process event handler
func (s *Service) SetEventHandler(handler ProcessHandler) {
	s.eventHandler = handler
}

// GetProcess returns a process by ID
func (s *Service) GetProcess(id string) *Process {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.processes[id]
}

// GetProcessForComponent returns the process for a project component
func (s *Service) GetProcessForComponent(projectID string, component projects.ComponentType) *Process {
	id := generateProcessID(projectID, component)
	return s.GetProcess(id)
}

// GetAllProcesses returns all processes
func (s *Service) GetAllProcesses() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	procs := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		procs = append(procs, p)
	}
	return procs
}

// GetRunningProcesses returns all running processes
func (s *Service) GetRunningProcesses() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var procs []*Process
	for _, p := range s.processes {
		if p.IsRunning() {
			procs = append(procs, p)
		}
	}
	return procs
}

// GetProcessesForProject returns all processes for a project
func (s *Service) GetProcessesForProject(projectID string) []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var procs []*Process
	for _, p := range s.processes {
		if p.ProjectID == projectID {
			procs = append(procs, p)
		}
	}
	return procs
}

// RegisterProcess registers a new process
func (s *Service) RegisterProcess(proc *Process) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processes[proc.ID] = proc
}

// UnregisterProcess removes a process
func (s *Service) UnregisterProcess(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.processes, id)
}

// StartProject starts all components of a project
func (s *Service) StartProject(ctx context.Context, projectID string, supervisor Supervisor) error {
	project, err := s.projectService.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %s", projectID)
	}

	for _, comp := range project.GetEnabledComponents() {
		if err := s.StartComponent(ctx, projectID, comp.Type, supervisor); err != nil {
			return fmt.Errorf("failed to start %s: %w", comp.Type, err)
		}
	}

	return nil
}

// StartComponent starts a specific component
func (s *Service) StartComponent(ctx context.Context, projectID string, component projects.ComponentType, supervisor Supervisor) error {
	project, err := s.projectService.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %s", projectID)
	}

	comp := project.GetComponent(component)
	if comp == nil {
		return fmt.Errorf("component not found: %s", component)
	}

	// Check if already running
	existing := s.GetProcessForComponent(projectID, component)
	if existing != nil && existing.IsRunning() {
		return fmt.Errorf("component is already running: %s/%s", projectID, component)
	}

	// Start the process
	proc, err := supervisor.Start(ctx, project, comp)
	if err != nil {
		return err
	}

	s.RegisterProcess(proc)
	return nil
}

// StopProject stops all components of a project
func (s *Service) StopProject(ctx context.Context, projectID string, supervisor Supervisor, force bool) error {
	procs := s.GetProcessesForProject(projectID)

	for _, proc := range procs {
		if proc.IsRunning() {
			if err := s.StopProcess(ctx, proc.ID, supervisor, force); err != nil {
				return fmt.Errorf("failed to stop %s: %w", proc.ID, err)
			}
		}
	}

	return nil
}

// StopProcess stops a specific process
func (s *Service) StopProcess(ctx context.Context, processID string, supervisor Supervisor, force bool) error {
	proc := s.GetProcess(processID)
	if proc == nil {
		return fmt.Errorf("process not found: %s", processID)
	}

	if !proc.IsRunning() {
		return nil // Already stopped
	}

	return supervisor.Stop(ctx, proc, force)
}

// RestartProcess restarts a process
func (s *Service) RestartProcess(ctx context.Context, processID string, supervisor Supervisor) error {
	proc := s.GetProcess(processID)
	if proc == nil {
		return fmt.Errorf("process not found: %s", processID)
	}

	// Stop if running
	if proc.IsRunning() {
		if err := supervisor.Stop(ctx, proc, false); err != nil {
			return fmt.Errorf("failed to stop process: %w", err)
		}
	}

	// Get project and component info to restart
	project, err := s.projectService.GetProject(proc.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %s", proc.ProjectID)
	}

	comp := project.GetComponent(proc.Component)
	if comp == nil {
		return fmt.Errorf("component not found: %s", proc.Component)
	}

	// Start again
	newProc, err := supervisor.Start(ctx, project, comp)
	if err != nil {
		return err
	}

	newProc.Restarts = proc.Restarts + 1
	s.RegisterProcess(newProc)

	return nil
}

// KillProcess forcefully kills a process
func (s *Service) KillProcess(processID string, supervisor Supervisor) error {
	proc := s.GetProcess(processID)
	if proc == nil {
		return fmt.Errorf("process not found: %s", processID)
	}

	return supervisor.Kill(proc)
}

// PauseProcess toggles pause state of a process
func (s *Service) PauseProcess(processID string, supervisor Supervisor) error {
	proc := s.GetProcess(processID)
	if proc == nil {
		return fmt.Errorf("process not found: %s", processID)
	}

	if !proc.IsRunning() {
		return fmt.Errorf("process is not running: %s", processID)
	}

	return supervisor.TogglePause(proc)
}

// GetProcessSummaries returns summaries of all processes
func (s *Service) GetProcessSummaries() []*ProcessSummary {
	procs := s.GetAllProcesses()
	summaries := make([]*ProcessSummary, len(procs))
	for i, p := range procs {
		summaries[i] = p.ToSummary()
	}
	return summaries
}

// EmitEvent emits a process event
func (s *Service) EmitEvent(event ProcessEvent) {
	if s.eventHandler != nil {
		s.eventHandler(event)
	}
}

// Supervisor interface for process supervision
type Supervisor interface {
	Start(ctx context.Context, project *projects.Project, component *projects.Component) (*Process, error)
	Stop(ctx context.Context, proc *Process, force bool) error
	Kill(proc *Process) error
	Signal(proc *Process, sig int) error
	TogglePause(proc *Process) error
}
