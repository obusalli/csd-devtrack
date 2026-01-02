package core

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/builder"
	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/platform/git"
	"csd-devtrack/cli/modules/platform/supervisor"
)

// AppPresenter is the main presenter implementation
type AppPresenter struct {
	mu sync.RWMutex

	// Services
	projectService *projects.Service
	buildOrch      *builder.Orchestrator
	processService *processes.Service
	processMgr     *supervisor.Manager
	gitService     *git.Service
	config         *config.Config

	// State
	state *AppState

	// Callbacks
	stateCallbacks        []func(StateUpdate)
	notificationCallbacks []func(*Notification)

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Build cancellation
	buildCtx    context.Context
	buildCancel context.CancelFunc

	// Self process tracking
	startTime time.Time // When csd-devtrack started
}

// NewAppPresenter creates a new application presenter
func NewAppPresenter(
	projectService *projects.Service,
	cfg *config.Config,
) *AppPresenter {
	return &AppPresenter{
		projectService:        projectService,
		config:                cfg,
		state:                 NewAppState(),
		stateCallbacks:        make([]func(StateUpdate), 0),
		notificationCallbacks: make([]func(*Notification), 0),
		startTime:             time.Now(), // Track when we started
	}
}

// NewPresenter is a convenience constructor that returns the Presenter interface
func NewPresenter(
	projectService *projects.Service,
	cfg *config.Config,
) Presenter {
	return NewAppPresenter(projectService, cfg)
}

// Initialize sets up the presenter
func (p *AppPresenter) Initialize(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Initialize build orchestrator
	parallelBuilds := 4
	if p.config != nil && p.config.Settings != nil {
		parallelBuilds = p.config.Settings.ParallelBuilds
	}
	p.buildOrch = builder.NewOrchestrator(p.projectService, parallelBuilds)

	// Initialize process service and manager
	p.processService = processes.NewService(p.projectService)
	p.processMgr = supervisor.NewManager(p.processService)

	// Initialize git service
	p.gitService = git.NewService(p.projectService)

	// Set up event handlers
	p.setupEventHandlers()

	// Initial data load (this includes slow git operations)
	err := p.Refresh()

	// Mark initialization as complete
	p.mu.Lock()
	p.state.Initializing = false
	p.mu.Unlock()

	// Broadcast state update to inform clients that initialization is complete
	p.broadcastFullState()

	return err
}

// broadcastFullState sends the current full state to all subscribers
func (p *AppPresenter) broadcastFullState() {
	p.mu.RLock()
	callbacks := p.stateCallbacks
	p.mu.RUnlock()

	// Notify all view updates to refresh the entire UI
	for _, viewType := range []ViewModelType{VMDashboard, VMProjects, VMBuild, VMProcesses, VMLogs, VMGit, VMConfig} {
		vm, _ := p.GetViewModel(viewType)
		if vm != nil {
			update := StateUpdate{
				ViewType:  viewType,
				ViewModel: vm,
			}
			for _, cb := range callbacks {
				cb(update)
			}
		}
	}
}

// setupEventHandlers sets up internal event handlers
func (p *AppPresenter) setupEventHandlers() {
	// Build events
	p.buildOrch.SetEventHandler(func(event builds.BuildEvent) {
		p.handleBuildEvent(event)
	})

	// Process events
	p.processService.SetEventHandler(func(event processes.ProcessEvent) {
		p.handleProcessEvent(event)
	})
}

// HandleEvent processes a user event
func (p *AppPresenter) HandleEvent(event *Event) error {
	switch event.Type {
	// Navigation
	case EventNavigate:
		return p.handleNavigate(event)
	case EventRefresh:
		return p.Refresh()
	case EventQuit:
		return p.Shutdown()

	// Project events
	case EventSelectProject:
		return p.handleSelectProject(event)
	case EventAddProject:
		return p.handleAddProject(event)
	case EventRemoveProject:
		return p.handleRemoveProject(event)

	// Build events
	case EventStartBuild:
		return p.handleStartBuild(event)
	case EventBuildAll:
		return p.handleBuildAll(event)
	case EventCancelBuild:
		return p.handleCancelBuild(event)

	// Process events
	case EventStartProcess:
		return p.handleStartProcess(event)
	case EventStopProcess:
		return p.handleStopProcess(event)
	case EventRestartProcess:
		return p.handleRestartProcess(event)
	case EventKillProcess:
		return p.handleKillProcess(event)
	case EventPauseProcess:
		return p.handlePauseProcess(event)

	// Git events
	case EventGitStatus:
		return p.handleGitStatus(event)
	case EventGitDiff:
		return p.handleGitDiff(event)
	case EventGitLog:
		return p.handleGitLog(event)

	// Filter/sort
	case EventFilter:
		return p.handleFilter(event)

	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// GetViewModel returns the view model for a view type
func (p *AppPresenter) GetViewModel(viewType ViewModelType) (ViewModel, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch viewType {
	case VMDashboard:
		return p.state.Dashboard, nil
	case VMProjects:
		return p.state.Projects, nil
	case VMBuild:
		return p.state.Builds, nil
	case VMProcesses:
		return p.state.Processes, nil
	case VMLogs:
		return p.state.Logs, nil
	case VMGit:
		return p.state.Git, nil
	case VMConfig:
		return p.state.Config, nil
	default:
		return nil, fmt.Errorf("unknown view type: %s", viewType)
	}
}

// Subscribe registers a callback for state updates
func (p *AppPresenter) Subscribe(callback func(StateUpdate)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stateCallbacks = append(p.stateCallbacks, callback)
}

// SubscribeNotifications registers a callback for notifications
func (p *AppPresenter) SubscribeNotifications(callback func(*Notification)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notificationCallbacks = append(p.notificationCallbacks, callback)
}

// Refresh forces a refresh of all data
func (p *AppPresenter) Refresh() error {
	// Refresh projects
	if err := p.refreshProjects(); err != nil {
		return err
	}

	// Refresh git status
	p.refreshGitStatus()

	// Refresh processes
	p.refreshProcesses()

	// Update dashboard
	p.refreshDashboard()

	p.state.LastRefresh = time.Now()
	return nil
}

// Shutdown cleans up resources
func (p *AppPresenter) Shutdown() error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// GetState returns the full application state (for daemon sync)
func (p *AppPresenter) GetState() *AppState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// ============================================
// Private handlers
// ============================================

func (p *AppPresenter) handleNavigate(event *Event) error {
	viewType := ViewModelType(event.Target)
	p.state.SetCurrentView(viewType)

	// Refresh the target view
	switch viewType {
	case VMDashboard:
		p.refreshDashboard()
	case VMProjects:
		p.refreshProjects()
	case VMProcesses:
		p.refreshProcesses()
	case VMGit:
		p.refreshGitStatus()
	}

	p.notifyStateUpdate(viewType, p.state.GetCurrentViewModel())
	return nil
}

func (p *AppPresenter) handleSelectProject(event *Event) error {
	p.mu.Lock()
	p.state.Projects.SelectedIndex = -1
	for i, proj := range p.state.Projects.Projects {
		if proj.ID == event.ProjectID {
			p.state.Projects.SelectedIndex = i
			break
		}
	}
	p.mu.Unlock()

	p.notifyStateUpdate(VMProjects, p.state.Projects)
	return nil
}

func (p *AppPresenter) handleAddProject(event *Event) error {
	path, ok := event.Value.(string)
	if !ok {
		return fmt.Errorf("invalid path")
	}

	project, err := p.projectService.AddProject(path)
	if err != nil {
		p.notify(NotifyError, "Add Project Failed", err.Error())
		return err
	}

	p.notify(NotifySuccess, "Project Added", fmt.Sprintf("Added %s", project.Name))
	return p.refreshProjects()
}

func (p *AppPresenter) handleRemoveProject(event *Event) error {
	if err := p.projectService.RemoveProject(event.ProjectID); err != nil {
		p.notify(NotifyError, "Remove Failed", err.Error())
		return err
	}

	p.notify(NotifySuccess, "Project Removed", fmt.Sprintf("Removed %s", event.ProjectID))
	return p.refreshProjects()
}

func (p *AppPresenter) handleStartBuild(event *Event) error {
	// Cancel any previous build
	if p.buildCancel != nil {
		p.buildCancel()
	}

	// Create a new cancellable context for this build
	p.buildCtx, p.buildCancel = context.WithCancel(p.ctx)

	go func() {
		var err error
		buildCtx := p.buildCtx // Capture context

		if event.Component != "" {
			result := p.buildOrch.BuildComponent(buildCtx, event.ProjectID, event.Component)
			if result.Error != nil {
				err = result.Error
			}
		} else {
			_, err = p.buildOrch.BuildProject(buildCtx, event.ProjectID)
		}

		// Check if cancelled
		if buildCtx.Err() == context.Canceled {
			p.notify(NotifyWarning, "Build Cancelled", fmt.Sprintf("%s build was cancelled", event.ProjectID))
			return
		}

		if err != nil {
			p.notify(NotifyError, "Build Failed", err.Error())
		} else {
			p.notify(NotifySuccess, "Build Complete", fmt.Sprintf("%s built successfully", event.ProjectID))
		}
	}()

	return nil
}

func (p *AppPresenter) handleBuildAll(event *Event) error {
	// Cancel any previous build
	if p.buildCancel != nil {
		p.buildCancel()
	}

	// Create a new cancellable context for this build
	p.buildCtx, p.buildCancel = context.WithCancel(p.ctx)

	go func() {
		buildCtx := p.buildCtx // Capture context
		results, err := p.buildOrch.BuildAll(buildCtx)

		// Check if cancelled
		if buildCtx.Err() == context.Canceled {
			p.notify(NotifyWarning, "Build Cancelled", "Build all was cancelled")
			return
		}

		if err != nil {
			p.notify(NotifyError, "Build Failed", err.Error())
			return
		}

		summary := p.buildOrch.Summarize(results)
		if summary.FailedProjects > 0 {
			p.notify(NotifyWarning, "Build Complete",
				fmt.Sprintf("%d/%d projects built with failures", summary.SuccessProjects, summary.TotalProjects))
		} else {
			p.notify(NotifySuccess, "Build Complete",
				fmt.Sprintf("All %d projects built successfully", summary.TotalProjects))
		}
	}()

	return nil
}

func (p *AppPresenter) handleCancelBuild(event *Event) error {
	if p.buildCancel != nil {
		p.buildCancel()
		p.buildCancel = nil

		// Update state to show build is cancelled
		p.mu.Lock()
		p.state.Builds.IsBuilding = false
		if p.state.Builds.CurrentBuild != nil {
			p.state.Builds.CurrentBuild.Status = builds.BuildStatusCanceled
		}
		p.mu.Unlock()

		p.notifyStateUpdate(VMBuild, p.state.Builds)
		p.notify(NotifyInfo, "Build Cancelled", "Build was cancelled by user")
	}
	return nil
}

func (p *AppPresenter) handleStartProcess(event *Event) error {
	go func() {
		err := p.processService.StartComponent(p.ctx, event.ProjectID, event.Component, p.processMgr)
		if err != nil {
			p.notify(NotifyError, "Start Failed", err.Error())
		} else {
			p.notify(NotifySuccess, "Started", fmt.Sprintf("%s/%s started", event.ProjectID, event.Component))
		}
		p.refreshProcesses()
	}()
	return nil
}

func (p *AppPresenter) handleStopProcess(event *Event) error {
	processID := fmt.Sprintf("%s/%s", event.ProjectID, event.Component)
	go func() {
		err := p.processService.StopProcess(p.ctx, processID, p.processMgr, false)
		if err != nil {
			p.notify(NotifyError, "Stop Failed", err.Error())
		}
		p.refreshProcesses()
	}()
	return nil
}

func (p *AppPresenter) handleRestartProcess(event *Event) error {
	processID := fmt.Sprintf("%s/%s", event.ProjectID, event.Component)
	go func() {
		err := p.processService.RestartProcess(p.ctx, processID, p.processMgr)
		if err != nil {
			p.notify(NotifyError, "Restart Failed", err.Error())
		}
		p.refreshProcesses()
	}()
	return nil
}

func (p *AppPresenter) handleKillProcess(event *Event) error {
	processID := fmt.Sprintf("%s/%s", event.ProjectID, event.Component)
	go func() {
		err := p.processService.KillProcess(processID, p.processMgr)
		if err != nil {
			p.notify(NotifyError, "Kill Failed", err.Error())
		}
		p.refreshProcesses()
	}()
	return nil
}

func (p *AppPresenter) handlePauseProcess(event *Event) error {
	processID := fmt.Sprintf("%s/%s", event.ProjectID, event.Component)
	go func() {
		err := p.processService.PauseProcess(processID, p.processMgr)
		if err != nil {
			p.notify(NotifyError, "Pause/Resume Failed", err.Error())
		} else {
			proc := p.processService.GetProcess(processID)
			if proc != nil && proc.IsPaused() {
				p.notify(NotifyInfo, "Paused", fmt.Sprintf("%s is paused", processID))
			} else {
				p.notify(NotifyInfo, "Resumed", fmt.Sprintf("%s is running", processID))
			}
		}
		p.refreshProcesses()
	}()
	return nil
}

func (p *AppPresenter) handleGitStatus(event *Event) error {
	p.refreshGitStatus()
	return nil
}

func (p *AppPresenter) handleGitDiff(event *Event) error {
	if event.ProjectID == "" {
		return nil
	}

	diff, err := p.gitService.GetDiff(event.ProjectID, git.DefaultDiffOptions())
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.state.Git.DiffFiles = diff.Files
	p.state.Git.ShowDiff = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMGit, p.state.Git)
	return nil
}

func (p *AppPresenter) handleGitLog(event *Event) error {
	if event.ProjectID == "" {
		return nil
	}

	commits, err := p.gitService.GetLog(event.ProjectID, git.DefaultLogOptions())
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.state.Git.Commits = make([]CommitVM, len(commits))
	for i, c := range commits {
		p.state.Git.Commits[i] = CommitVM{
			Hash:      c.Hash,
			ShortHash: c.ShortHash,
			Author:    c.Author,
			Date:      c.Date,
			DateStr:   c.Date.Format("2006-01-02 15:04"),
			Subject:   c.Subject,
		}
	}
	p.mu.Unlock()

	p.notifyStateUpdate(VMGit, p.state.Git)
	return nil
}

func (p *AppPresenter) handleFilter(event *Event) error {
	filterText, _ := event.Value.(string)

	p.mu.Lock()
	switch p.state.CurrentView {
	case VMProjects:
		p.state.Projects.FilterText = filterText
	case VMProcesses:
		p.state.Processes.FilterProject = filterText
	case VMLogs:
		p.state.Logs.FilterProject = filterText
	}
	p.mu.Unlock()

	p.notifyStateUpdate(p.state.CurrentView, p.state.GetCurrentViewModel())
	return nil
}

// ============================================
// Refresh helpers
// ============================================

func (p *AppPresenter) refreshProjects() error {
	// Enrich projects with git info
	p.gitService.EnrichAllProjects()

	allProjects := p.projectService.ListProjects()

	p.mu.Lock()
	p.state.Projects.Projects = make([]ProjectVM, len(allProjects))
	for i, proj := range allProjects {
		p.state.Projects.Projects[i] = p.projectToVM(proj)
	}

	// Sort by project name for consistent ordering
	sort.Slice(p.state.Projects.Projects, func(i, j int) bool {
		return p.state.Projects.Projects[i].Name < p.state.Projects.Projects[j].Name
	})

	p.state.Projects.UpdatedAt = time.Now()
	p.mu.Unlock()

	p.notifyStateUpdate(VMProjects, p.state.Projects)
	return nil
}

func (p *AppPresenter) refreshProcesses() {
	allProcesses := p.processService.GetAllProcesses()

	p.mu.Lock()
	// Start with supervised processes
	p.state.Processes.Processes = make([]ProcessVM, 0, len(allProcesses)+1)
	for _, proc := range allProcesses {
		p.state.Processes.Processes = append(p.state.Processes.Processes, p.processToVM(proc))
	}

	// Add self process (csd-devtrack itself)
	selfProcess := ProcessVM{
		ID:          "self",
		ProjectID:   "csd-devtrack",
		ProjectName: "csd-devtrack",
		Component:   projects.ComponentCLI,
		State:       processes.ProcessStateRunning,
		PID:         os.Getpid(),
		Uptime:      time.Since(p.startTime).Round(time.Second).String(),
		Restarts:    0,
		IsSelf:      true,
	}
	p.state.Processes.Processes = append(p.state.Processes.Processes, selfProcess)

	// Sort by project name for consistent ordering (self will be at the end due to 'c' in csd-devtrack)
	sort.Slice(p.state.Processes.Processes, func(i, j int) bool {
		// Put self first
		if p.state.Processes.Processes[i].IsSelf {
			return true
		}
		if p.state.Processes.Processes[j].IsSelf {
			return false
		}
		return p.state.Processes.Processes[i].ProjectName < p.state.Processes.Processes[j].ProjectName
	})

	p.state.Processes.UpdatedAt = time.Now()
	p.mu.Unlock()

	p.notifyStateUpdate(VMProcesses, p.state.Processes)
}

func (p *AppPresenter) refreshGitStatus() {
	allStatus := p.gitService.GetAllStatus()

	p.mu.Lock()
	p.state.Git.Projects = make([]GitStatusVM, 0, len(allStatus))
	for projectID, status := range allStatus {
		proj, _ := p.projectService.GetProject(projectID)
		name := projectID
		if proj != nil {
			name = proj.Name
		}

		p.state.Git.Projects = append(p.state.Git.Projects, GitStatusVM{
			ProjectID:   projectID,
			ProjectName: name,
			Branch:      status.Branch,
			IsClean:     status.IsClean,
			Ahead:       status.Ahead,
			Behind:      status.Behind,
			Staged:      status.Staged,
			Modified:    status.Modified,
			Untracked:   status.Untracked,
			Deleted:     status.Deleted,
		})
	}

	// Sort by project name for consistent ordering
	sort.Slice(p.state.Git.Projects, func(i, j int) bool {
		return p.state.Git.Projects[i].ProjectName < p.state.Git.Projects[j].ProjectName
	})

	p.state.Git.UpdatedAt = time.Now()
	p.mu.Unlock()

	p.notifyStateUpdate(VMGit, p.state.Git)
}

func (p *AppPresenter) refreshDashboard() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.Dashboard.ProjectCount = len(p.state.Projects.Projects)
	p.state.Dashboard.Projects = p.state.Projects.Projects

	// Count running processes
	running := 0
	for _, proc := range p.state.Processes.Processes {
		if proc.State == "running" {
			running++
		}
	}
	p.state.Dashboard.RunningCount = running
	p.state.Dashboard.RunningProcesses = p.state.Processes.Processes

	// Git summary
	p.state.Dashboard.GitSummary = p.state.Git.Projects

	p.state.Dashboard.UpdatedAt = time.Now()
}

// ============================================
// Converters
// ============================================

func (p *AppPresenter) projectToVM(proj *projects.Project) ProjectVM {
	vm := ProjectVM{
		ID:         proj.ID,
		Name:       proj.Name,
		Path:       proj.Path,
		Type:       proj.Type,
		IsSelf:     proj.Self,
		GitBranch:  proj.GitBranch,
		GitDirty:   proj.GitDirty,
		GitAhead:   proj.GitAhead,
		GitBehind:  proj.GitBehind,
		Components: make([]ComponentVM, 0),
	}

	for _, ct := range projects.AllComponentTypes() {
		if comp := proj.GetComponent(ct); comp != nil && comp.Enabled {
			cvm := ComponentVM{
				Type:    comp.Type,
				Path:    comp.Path,
				Binary:  comp.Binary,
				Port:    comp.Port,
				Enabled: comp.Enabled,
			}

			// Check if running
			proc := p.processService.GetProcessForComponent(proj.ID, ct)
			if proc != nil && proc.IsRunning() {
				cvm.IsRunning = true
				cvm.PID = proc.PID
				vm.RunningCount++
			} else if proj.Self && ct == projects.ComponentCLI {
				// Self project CLI is always running (it's us!)
				cvm.IsRunning = true
				cvm.PID = os.Getpid()
				vm.RunningCount++
			}

			vm.Components = append(vm.Components, cvm)
		}
	}

	return vm
}

func (p *AppPresenter) processToVM(proc *processes.Process) ProcessVM {
	proj, _ := p.projectService.GetProject(proc.ProjectID)
	name := proc.ProjectID
	isSelf := false
	if proj != nil {
		name = proj.Name
		isSelf = proj.Self
	}

	vm := ProcessVM{
		ID:          proc.ID,
		ProjectID:   proc.ProjectID,
		ProjectName: name,
		Component:   proc.Component,
		State:       proc.State,
		PID:         proc.PID,
		Restarts:    proc.Restarts,
		LastError:   proc.LastError,
		IsSelf:      isSelf,
	}

	if proc.State == processes.ProcessStateRunning && !proc.StartedAt.IsZero() {
		vm.Uptime = time.Since(proc.StartedAt).Round(time.Second).String()
	}

	return vm
}

// ============================================
// Notification helpers
// ============================================

func (p *AppPresenter) notify(ntype NotificationType, title, message string) {
	n := NewNotification(ntype, title, message)
	p.state.AddNotification(n)

	p.mu.RLock()
	callbacks := p.notificationCallbacks
	p.mu.RUnlock()

	for _, cb := range callbacks {
		cb(n)
	}
}

func (p *AppPresenter) notifyStateUpdate(viewType ViewModelType, vm ViewModel) {
	update := StateUpdate{
		ViewType:  viewType,
		ViewModel: vm,
	}

	p.mu.RLock()
	callbacks := p.stateCallbacks
	p.mu.RUnlock()

	for _, cb := range callbacks {
		cb(update)
	}
}

// ============================================
// Event handlers from services
// ============================================

func (p *AppPresenter) handleBuildEvent(event builds.BuildEvent) {
	// Update build view model
	p.mu.Lock()
	if p.state.Builds.CurrentBuild == nil {
		p.state.Builds.CurrentBuild = &BuildVM{}
	}
	p.state.Builds.CurrentBuild.ID = event.BuildID
	p.state.Builds.CurrentBuild.ProjectID = event.ProjectID
	p.state.Builds.CurrentBuild.Component = projects.ComponentType(event.Component)

	switch event.Type {
	case builds.BuildEventStarted:
		p.state.Builds.IsBuilding = true
		p.state.Builds.CurrentBuild.Status = builds.BuildStatusRunning
		p.state.Builds.CurrentBuild.Output = []string{}
	case builds.BuildEventOutput:
		p.state.Builds.CurrentBuild.Output = append(p.state.Builds.CurrentBuild.Output, event.Message)
	case builds.BuildEventError:
		p.state.Builds.CurrentBuild.Errors = append(p.state.Builds.CurrentBuild.Errors, event.Message)
	case builds.BuildEventFinished:
		p.state.Builds.IsBuilding = false
	}

	// Also add to Logs view for persistence
	logLine := LogLineVM{
		Timestamp: event.Timestamp,
		TimeStr:   event.Timestamp.Format("15:04:05"),
		Source:    fmt.Sprintf("build:%s/%s", event.ProjectID, event.Component),
		Message:   event.Message,
	}
	switch event.Type {
	case builds.BuildEventError:
		logLine.Level = "error"
	case builds.BuildEventWarning:
		logLine.Level = "warn"
	default:
		logLine.Level = "info"
	}
	p.state.Logs.Lines = append(p.state.Logs.Lines, logLine)
	if len(p.state.Logs.Lines) > p.state.Logs.MaxLines {
		p.state.Logs.Lines = p.state.Logs.Lines[1:]
	}

	p.mu.Unlock()

	p.notifyStateUpdate(VMBuild, p.state.Builds)
	p.notifyStateUpdate(VMLogs, p.state.Logs)
}

func (p *AppPresenter) handleProcessEvent(event processes.ProcessEvent) {
	p.refreshProcesses()

	// Add to logs
	p.mu.Lock()
	logLine := LogLineVM{
		Timestamp: event.Timestamp,
		TimeStr:   event.Timestamp.Format("15:04:05"),
		Source:    event.ProcessID,
		Message:   event.Message,
	}

	switch event.Type {
	case processes.ProcessEventError:
		logLine.Level = "error"
	case processes.ProcessEventCrashed:
		logLine.Level = "error"
	default:
		logLine.Level = "info"
	}

	p.state.Logs.Lines = append(p.state.Logs.Lines, logLine)
	if len(p.state.Logs.Lines) > p.state.Logs.MaxLines {
		p.state.Logs.Lines = p.state.Logs.Lines[1:]
	}
	p.mu.Unlock()

	p.notifyStateUpdate(VMLogs, p.state.Logs)
}
