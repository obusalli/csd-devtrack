package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/builder"
	"csd-devtrack/cli/modules/platform/claude"
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
	claudeService  *claude.Service
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

	// Initialize Claude service
	claudeDataDir := ""
	if p.config != nil && p.config.Settings != nil && p.config.Settings.Claude != nil {
		claudeDataDir = p.config.Settings.Claude.SessionsDir
	}
	if claudeDataDir == "" {
		// Default to ~/.csd-devtrack/claude-sessions/
		if home, err := os.UserHomeDir(); err == nil {
			claudeDataDir = filepath.Join(home, ".csd-devtrack", "claude-sessions")
		}
	}
	p.claudeService = claude.NewService(claudeDataDir)

	// Initialize Claude state
	p.refreshClaude()

	// Set up event handlers
	p.setupEventHandlers()

	// FAST: Load projects without git info first
	p.refreshProjectsWithoutGit()
	p.refreshProcesses()
	p.refreshDashboard()

	// Mark initialization as complete (projects are ready, UI can display)
	p.mu.Lock()
	p.state.Initializing = false
	p.state.GitLoading = true // Git is loading in background
	p.mu.Unlock()

	// Broadcast state update to inform clients that initialization is complete
	p.broadcastFullState()

	// SLOW: Start git operations in background
	go p.loadGitInBackground()

	return nil
}

// loadGitInBackground loads git info for all projects in background
func (p *AppPresenter) loadGitInBackground() {
	// Enrich all projects with git info (slow operation)
	p.gitService.EnrichAllProjects()

	// Refresh git status
	p.refreshGitStatus()

	// Update projects with git info
	p.refreshProjectsFromGit()

	// Update dashboard with new git info
	p.refreshDashboard()

	// Mark git loading as complete
	p.mu.Lock()
	p.state.GitLoading = false
	p.state.LastRefresh = time.Now()
	p.mu.Unlock()

	// Broadcast full state update
	p.broadcastFullState()
}

// broadcastFullState sends the current full state to all subscribers
func (p *AppPresenter) broadcastFullState() {
	p.mu.RLock()
	callbacks := p.stateCallbacks
	p.mu.RUnlock()

	// Notify all view updates to refresh the entire UI
	for _, viewType := range []ViewModelType{VMDashboard, VMProjects, VMBuild, VMProcesses, VMLogs, VMGit, VMConfig, VMClaude} {
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

	// Claude events
	case EventClaudeCreateSession:
		return p.handleClaudeCreateSession(event)
	case EventClaudeSelectSession:
		return p.handleClaudeSelectSession(event)
	case EventClaudeDeleteSession:
		return p.handleClaudeDeleteSession(event)
	case EventClaudeRenameSession:
		return p.handleClaudeRenameSession(event)
	case EventClaudeSendMessage:
		return p.handleClaudeSendMessage(event)
	case EventClaudeStopSession:
		return p.handleClaudeStopSession(event)
	case EventClaudeClearHistory:
		return p.handleClaudeClearHistory(event)
	case EventClaudeApprovePermission:
		return p.handleClaudeApprovePermission(event)
	case EventClaudeDenyPermission:
		return p.handleClaudeDenyPermission(event)
	case EventClaudeAnswerQuestion:
		return p.handleClaudeAnswerQuestion(event)
	case EventClaudeApprovePlan:
		return p.handleClaudeApprovePlan(event)
	case EventClaudeRejectPlan:
		return p.handleClaudeRejectPlan(event)

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
	case VMClaude:
		return p.state.Claude, nil
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
	// Refresh projects (will skip git enrichment if git is loading in background)
	if err := p.refreshProjects(); err != nil {
		return err
	}

	// Refresh git status (skip if git is loading in background)
	if !p.state.GitLoading {
		p.refreshGitStatus()
	}

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

	// Shutdown Claude service
	if p.claudeService != nil {
		p.claudeService.Shutdown()
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
	case VMClaude:
		p.refreshClaude()
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
	// Enrich projects with git info (if not loading in background)
	if !p.state.GitLoading {
		p.gitService.EnrichAllProjects()
	}

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

// refreshProjectsWithoutGit loads projects without git info (fast)
func (p *AppPresenter) refreshProjectsWithoutGit() error {
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

// refreshProjectsFromGit updates projects with git info after background load
func (p *AppPresenter) refreshProjectsFromGit() {
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

// ============================================
// Claude handlers
// ============================================

func (p *AppPresenter) handleClaudeCreateSession(event *Event) error {
	projectID := event.ProjectID
	if projectID == "" {
		return fmt.Errorf("project ID required")
	}

	// Get project info
	proj, err := p.projectService.GetProject(projectID)
	if err != nil {
		return err
	}

	// Create session
	session, err := p.claudeService.CreateSession(projectID, proj.Name, proj.Path, "")
	if err != nil {
		p.notify(NotifyError, "Create Session Failed", err.Error())
		return err
	}

	p.notify(NotifySuccess, "Session Created", fmt.Sprintf("Created %s", session.Name))
	p.refreshClaude()
	return nil
}

func (p *AppPresenter) handleClaudeSelectSession(event *Event) error {
	sessionID, ok := event.Value.(string)
	if !ok || sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	session, err := p.claudeService.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Start persistent process for faster responses
	// Uses stream-json bidirectional mode to keep Claude running
	go func() {
		if err := p.claudeService.StartPersistentProcess(sessionID); err != nil {
			p.notify(NotifyWarning, "Persistent Mode", "Could not start persistent process: "+err.Error())
		}
	}()

	// Update active session in state
	p.mu.Lock()
	p.state.Claude.ActiveSessionID = sessionID
	p.state.Claude.ActiveSession = p.sessionToVM(session)
	p.state.Claude.Messages = p.messagesToVM(session.Messages)
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)
	return nil
}

func (p *AppPresenter) handleClaudeDeleteSession(event *Event) error {
	sessionID, ok := event.Value.(string)
	if !ok || sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.DeleteSession(sessionID); err != nil {
		p.notify(NotifyError, "Delete Failed", err.Error())
		return err
	}

	p.notify(NotifySuccess, "Session Deleted", "Session was deleted")
	p.refreshClaude()
	return nil
}

func (p *AppPresenter) handleClaudeRenameSession(event *Event) error {
	sessionID := event.Data["session_id"]
	newName := event.Data["new_name"]
	if sessionID == "" || newName == "" {
		return fmt.Errorf("session ID and new name required")
	}

	if err := p.claudeService.RenameSession(sessionID, newName); err != nil {
		p.notify(NotifyError, "Rename Failed", err.Error())
		return err
	}

	p.notify(NotifySuccess, "Session Renamed", fmt.Sprintf("Renamed to %s", newName))
	p.refreshClaude()
	return nil
}

func (p *AppPresenter) handleClaudeSendMessage(event *Event) error {
	sessionID := event.Data["session_id"]
	message := event.Data["message"]
	if sessionID == "" || message == "" {
		return fmt.Errorf("session ID and message required")
	}

	// The TUI may have already added the user message for immediate feedback.
	// We check if the last user message matches to avoid duplicates.
	now := time.Now()
	p.mu.Lock()

	// Check if user message already exists (added by TUI for instant feedback)
	needUserMsg := true
	if len(p.state.Claude.Messages) > 0 {
		lastMsg := p.state.Claude.Messages[len(p.state.Claude.Messages)-1]
		// Check if it's a recent user message with same content (within last 2 seconds)
		if lastMsg.Role == "user" && lastMsg.Content == message &&
			now.Sub(lastMsg.Timestamp) < 2*time.Second {
			needUserMsg = false
		}
		// Also check if there's an assistant placeholder after a matching user msg
		if len(p.state.Claude.Messages) > 1 && lastMsg.Role == "assistant" && lastMsg.IsPartial {
			prevMsg := p.state.Claude.Messages[len(p.state.Claude.Messages)-2]
			if prevMsg.Role == "user" && prevMsg.Content == message &&
				now.Sub(prevMsg.Timestamp) < 2*time.Second {
				needUserMsg = false
			}
		}
	}

	if needUserMsg {
		userMsg := ClaudeMessageVM{
			ID:        fmt.Sprintf("user-%d", now.UnixNano()),
			Role:      "user",
			Content:   message,
			Timestamp: now,
			TimeStr:   now.Format("060102 - 15:04:05"),
		}
		p.state.Claude.Messages = append(p.state.Claude.Messages, userMsg)

		// Add placeholder for assistant response
		assistantMsg := ClaudeMessageVM{
			ID:        fmt.Sprintf("assistant-%d", now.UnixNano()),
			Role:      "assistant",
			Content:   "",
			Timestamp: now,
			TimeStr:   now.Format("060102 - 15:04:05"),
			IsPartial: true,
		}
		p.state.Claude.Messages = append(p.state.Claude.Messages, assistantMsg)
	}

	assistantIdx := len(p.state.Claude.Messages) - 1
	p.state.Claude.IsProcessing = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)

	// Create output channel for streaming
	outputChan := make(chan claude.ClaudeOutput, 100)

	// Start processing in background
	go func() {
		var err error

		// Try persistent process first (faster), fallback to one-shot
		if p.claudeService.IsPersistentProcessRunning(sessionID) {
			err = p.claudeService.SendMessagePersistent(sessionID, message, outputChan)
		} else {
			err = p.claudeService.SendMessage(p.ctx, sessionID, message, outputChan)
		}

		if err != nil {
			p.notify(NotifyError, "Send Failed", err.Error())
			p.mu.Lock()
			p.state.Claude.IsProcessing = false
			p.mu.Unlock()
			p.notifyStateUpdate(VMClaude, p.state.Claude)
			return
		}

		// Process streaming output
		var contentBuilder strings.Builder
		for output := range outputChan {
			if output.IsEnd {
				break
			}

			p.mu.Lock()
			switch output.Type {
			case "text":
				contentBuilder.WriteString(output.Content)
				if assistantIdx < len(p.state.Claude.Messages) {
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String()
				}

			case "tool_use":
				// Show tool usage with formatted diff output (Claude CLI style)
				if assistantIdx < len(p.state.Claude.Messages) {
					toolInfo := "\n" + formatToolOutput(output.Tool, output.Content)
					contentBuilder.WriteString(toolInfo)
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String()
				}

				// Check if waiting for permission
				if output.WaitingForInput && output.InputType == "permission" {
					p.state.Claude.WaitingForInput = true
					p.state.Claude.IsProcessing = false
					p.state.Claude.Interactive = &ClaudeInteractiveVM{
						Type:     "permission",
						ToolName: output.Tool,
						ToolID:   output.ToolID,
						FilePath: output.FilePath,
					}
				}

			case "thinking":
				// Show Claude's thinking process
				if assistantIdx < len(p.state.Claude.Messages) {
					thinkInfo := fmt.Sprintf("\n💭 *thinking...*\n%s\n", output.Content)
					contentBuilder.WriteString(thinkInfo)
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String()
				}

			case "permission_request", "permission_denied":
				// Show permission issues
				if assistantIdx < len(p.state.Claude.Messages) {
					permInfo := fmt.Sprintf("\n⚠️ %s\n", output.Content)
					contentBuilder.WriteString(permInfo)
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String()
				}

			case "question":
				// Claude is asking a question
				if assistantIdx < len(p.state.Claude.Messages) {
					questionInfo := fmt.Sprintf("\n❓ %s\n", output.Content)
					contentBuilder.WriteString(questionInfo)
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String()
				}
				if output.WaitingForInput {
					p.state.Claude.WaitingForInput = true
					p.state.Claude.IsProcessing = false
					p.state.Claude.Interactive = &ClaudeInteractiveVM{
						Type:     "question",
						Question: output.Content,
						Options:  output.Options,
					}
				}

			case "plan":
				// Claude has a plan for approval
				if assistantIdx < len(p.state.Claude.Messages) {
					planInfo := fmt.Sprintf("\n📋 Plan:\n%s\n", output.Content)
					contentBuilder.WriteString(planInfo)
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String()
				}
				if output.WaitingForInput {
					p.state.Claude.WaitingForInput = true
					p.state.Claude.IsProcessing = false
					p.state.Claude.PlanMode = true
					p.state.Claude.PlanPending = true
					p.state.Claude.Interactive = &ClaudeInteractiveVM{
						Type:        "plan",
						PlanContent: output.Content,
					}
				}

			case "error":
				p.state.Claude.IsProcessing = false
				p.state.Claude.WaitingForInput = false
				if assistantIdx < len(p.state.Claude.Messages) {
					p.state.Claude.Messages[assistantIdx].Content = contentBuilder.String() + "\n❌ Error: " + output.Content
					p.state.Claude.Messages[assistantIdx].IsPartial = false
				}
			}
			p.mu.Unlock()

			// Notify UI of update
			p.notifyStateUpdate(VMClaude, p.state.Claude)
		}

		// Mark as complete
		p.mu.Lock()
		p.state.Claude.IsProcessing = false
		if assistantIdx < len(p.state.Claude.Messages) {
			p.state.Claude.Messages[assistantIdx].IsPartial = false
		}
		p.mu.Unlock()

		// Final refresh to sync with service state
		p.refreshClaudeMessages(sessionID)
		p.notifyStateUpdate(VMClaude, p.state.Claude)
	}()

	return nil
}

func (p *AppPresenter) handleClaudeStopSession(event *Event) error {
	sessionID, ok := event.Value.(string)
	if !ok || sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.StopSession(sessionID); err != nil {
		p.notify(NotifyError, "Stop Failed", err.Error())
		return err
	}

	p.notify(NotifyInfo, "Stopped", "Claude session stopped")
	p.refreshClaude()
	return nil
}

func (p *AppPresenter) handleClaudeClearHistory(event *Event) error {
	sessionID, ok := event.Value.(string)
	if !ok || sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.ClearSessionHistory(sessionID); err != nil {
		p.notify(NotifyError, "Clear Failed", err.Error())
		return err
	}

	p.notify(NotifyInfo, "Cleared", "Session history cleared")
	p.refreshClaude()
	return nil
}

func (p *AppPresenter) handleClaudeApprovePermission(event *Event) error {
	sessionID := event.Data["session_id"]
	if sessionID == "" {
		sessionID = p.state.Claude.ActiveSessionID
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.SendInteractiveResponse(sessionID, "approve", ""); err != nil {
		p.notify(NotifyError, "Approval Failed", err.Error())
		return err
	}

	// Clear interactive state in UI
	p.mu.Lock()
	p.state.Claude.WaitingForInput = false
	p.state.Claude.Interactive = nil
	p.state.Claude.IsProcessing = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)
	return nil
}

func (p *AppPresenter) handleClaudeDenyPermission(event *Event) error {
	sessionID := event.Data["session_id"]
	if sessionID == "" {
		sessionID = p.state.Claude.ActiveSessionID
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.SendInteractiveResponse(sessionID, "deny", ""); err != nil {
		p.notify(NotifyError, "Denial Failed", err.Error())
		return err
	}

	// Clear interactive state in UI
	p.mu.Lock()
	p.state.Claude.WaitingForInput = false
	p.state.Claude.Interactive = nil
	p.state.Claude.IsProcessing = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)
	return nil
}

func (p *AppPresenter) handleClaudeAnswerQuestion(event *Event) error {
	sessionID := event.Data["session_id"]
	answer := event.Data["answer"]
	if sessionID == "" {
		sessionID = p.state.Claude.ActiveSessionID
	}
	if sessionID == "" || answer == "" {
		return fmt.Errorf("session ID and answer required")
	}

	if err := p.claudeService.SendInteractiveResponse(sessionID, answer, answer); err != nil {
		p.notify(NotifyError, "Answer Failed", err.Error())
		return err
	}

	// Clear interactive state in UI
	p.mu.Lock()
	p.state.Claude.WaitingForInput = false
	p.state.Claude.Interactive = nil
	p.state.Claude.IsProcessing = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)
	return nil
}

func (p *AppPresenter) handleClaudeApprovePlan(event *Event) error {
	sessionID := event.Data["session_id"]
	if sessionID == "" {
		sessionID = p.state.Claude.ActiveSessionID
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.SendInteractiveResponse(sessionID, "yes", ""); err != nil {
		p.notify(NotifyError, "Plan Approval Failed", err.Error())
		return err
	}

	// Clear plan state in UI
	p.mu.Lock()
	p.state.Claude.PlanPending = false
	p.state.Claude.WaitingForInput = false
	p.state.Claude.Interactive = nil
	p.state.Claude.IsProcessing = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)
	return nil
}

func (p *AppPresenter) handleClaudeRejectPlan(event *Event) error {
	sessionID := event.Data["session_id"]
	if sessionID == "" {
		sessionID = p.state.Claude.ActiveSessionID
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if err := p.claudeService.SendInteractiveResponse(sessionID, "no", ""); err != nil {
		p.notify(NotifyError, "Plan Rejection Failed", err.Error())
		return err
	}

	// Clear plan state in UI
	p.mu.Lock()
	p.state.Claude.PlanPending = false
	p.state.Claude.WaitingForInput = false
	p.state.Claude.Interactive = nil
	p.state.Claude.IsProcessing = true
	p.mu.Unlock()

	p.notifyStateUpdate(VMClaude, p.state.Claude)
	return nil
}

// ============================================
// Claude refresh and converters
// ============================================

func (p *AppPresenter) refreshClaude() {
	if p.claudeService == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update installation status
	p.state.Claude.IsInstalled = p.claudeService.IsInstalled()
	p.state.Claude.ClaudePath = p.claudeService.GetClaudePath()

	// Get sessions with persistent processes
	persistentSessions := make(map[string]bool)
	for _, id := range p.claudeService.GetPersistentProcessSessions() {
		persistentSessions[id] = true
	}

	// Get all sessions
	sessions := p.claudeService.ListSessions("")
	p.state.Claude.Sessions = make([]ClaudeSessionVM, len(sessions))
	for i, s := range sessions {
		p.state.Claude.Sessions[i] = ClaudeSessionVM{
			ID:           s.ID,
			Name:         s.Name,
			ProjectID:    s.ProjectID,
			ProjectName:  s.ProjectName,
			State:        string(s.State),
			MessageCount: s.MessageCount,
			LastActive:   s.LastActiveAt.Format("2006-01-02 15:04"),
			IsActive:     s.ID == p.state.Claude.ActiveSessionID,
			IsPersistent: persistentSessions[s.ID],
		}
	}

	// Load settings from config
	if p.config != nil && p.config.Settings != nil && p.config.Settings.Claude != nil {
		cfg := p.config.Settings.Claude
		p.state.Claude.Settings = &ClaudeSettingsVM{
			AllowedTools:    cfg.AllowedTools,
			AutoApprove:     cfg.AutoApprove,
			OutputFormat:    cfg.OutputFormat,
			MaxTurns:        cfg.MaxTurns,
			PlanModeEnabled: cfg.PlanModeEnabled,
		}
	}
}

// refreshClaudeMessages syncs messages from the service for a specific session
func (p *AppPresenter) refreshClaudeMessages(sessionID string) {
	if p.claudeService == nil || sessionID == "" {
		return
	}

	session, err := p.claudeService.GetSession(sessionID)
	if err != nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update messages from the service (authoritative source)
	p.state.Claude.Messages = p.messagesToVM(session.Messages)

	// Update active session info
	p.state.Claude.ActiveSession = p.sessionToVM(session)
}

func (p *AppPresenter) sessionToVM(session *claude.Session) *ClaudeSessionVM {
	if session == nil {
		return nil
	}
	return &ClaudeSessionVM{
		ID:           session.ID,
		Name:         session.DisplayName(), // Use custom name if set
		ProjectID:    session.ProjectID,
		ProjectName:  session.ProjectName,
		State:        string(session.State),
		MessageCount: len(session.Messages),
		LastActive:   session.LastActiveAt.Format("2006-01-02 15:04"),
		IsActive:     true,
	}
}

// formatToolOutput formats tool usage for display in chat
// Mimics Claude CLI format with diffs and line numbers
func formatToolOutput(toolName, content string) string {
	var sb strings.Builder

	// Parse the tool input JSON
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(content), &input); err != nil {
		// Not JSON, just show raw content
		sb.WriteString(fmt.Sprintf("● %s\n", toolName))
		if content != "" {
			sb.WriteString(fmt.Sprintf("  ⎿  %s\n", truncateForDisplay(content, 100)))
		}
		return sb.String()
	}

	switch toolName {
	case "Read":
		// Read tool: show file path and line count estimate
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			filePath, _ = input["path"].(string)
		}
		fileName := filepath.Base(filePath)
		sb.WriteString(fmt.Sprintf("● Read(%s)\n", fileName))
		// Estimate lines from content or show generic message
		if limit, ok := input["limit"].(float64); ok {
			sb.WriteString(fmt.Sprintf("  ⎿  Read %d lines\n", int(limit)))
		} else {
			sb.WriteString("  ⎿  Read file\n")
		}

	case "Edit":
		// Edit tool: show file path and diff
		filePath, _ := input["file_path"].(string)
		fileName := filepath.Base(filePath)
		oldStr, _ := input["old_string"].(string)
		newStr, _ := input["new_string"].(string)

		sb.WriteString(fmt.Sprintf("● Update(%s)\n", fileName))

		// Count lines changed
		oldLines := strings.Count(oldStr, "\n") + 1
		newLines := strings.Count(newStr, "\n") + 1
		addedLines := 0
		removedLines := 0

		if newLines > oldLines {
			addedLines = newLines - oldLines
		} else if oldLines > newLines {
			removedLines = oldLines - newLines
		}

		// Summary line
		summary := ""
		if addedLines > 0 && removedLines > 0 {
			summary = fmt.Sprintf("Added %d lines, removed %d lines", addedLines, removedLines)
		} else if addedLines > 0 {
			summary = fmt.Sprintf("Added %d lines", addedLines)
		} else if removedLines > 0 {
			summary = fmt.Sprintf("Removed %d lines", removedLines)
		} else {
			summary = "Modified"
		}
		sb.WriteString(fmt.Sprintf("  ⎿  %s\n", summary))

		// Show diff with context
		sb.WriteString(formatDiff(oldStr, newStr, 2))

	case "Write":
		// Write tool: show file path and size
		filePath, _ := input["file_path"].(string)
		fileName := filepath.Base(filePath)
		contentStr, _ := input["content"].(string)
		lines := strings.Count(contentStr, "\n") + 1

		sb.WriteString(fmt.Sprintf("● Write(%s)\n", fileName))
		sb.WriteString(fmt.Sprintf("  ⎿  Wrote %d lines\n", lines))

	case "Bash":
		// Bash command: show command
		cmd, _ := input["command"].(string)
		desc, _ := input["description"].(string)
		if desc != "" {
			sb.WriteString(fmt.Sprintf("● Bash(%s)\n", desc))
		} else {
			// Truncate command for display
			cmdDisplay := truncateForDisplay(cmd, 50)
			sb.WriteString(fmt.Sprintf("● Bash(%s)\n", cmdDisplay))
		}
		sb.WriteString("  ⎿  (running...)\n")

	case "Glob", "Grep":
		// Search tools
		pattern, _ := input["pattern"].(string)
		sb.WriteString(fmt.Sprintf("● %s(%s)\n", toolName, truncateForDisplay(pattern, 40)))
		sb.WriteString("  ⎿  Searching...\n")

	default:
		// Generic tool display
		sb.WriteString(fmt.Sprintf("● %s\n", toolName))
		if content != "" && len(content) < 200 {
			sb.WriteString(fmt.Sprintf("  ⎿  %s\n", truncateForDisplay(content, 100)))
		}
	}

	return sb.String()
}

// formatDiff creates a simple diff display with context
// Uses markers {{-...}} for removed and {{+...}} for added lines
// These markers are interpreted by the view layer for coloring
func formatDiff(oldStr, newStr string, contextLines int) string {
	var sb strings.Builder

	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// Find common prefix (lines that are identical at the start)
	commonPrefix := 0
	minLen := len(oldLines)
	if len(newLines) < minLen {
		minLen = len(newLines)
	}
	for i := 0; i < minLen && oldLines[i] == newLines[i]; i++ {
		commonPrefix++
	}

	// Find common suffix (lines that are identical at the end)
	commonSuffix := 0
	for i := 0; i < minLen-commonPrefix && oldLines[len(oldLines)-1-i] == newLines[len(newLines)-1-i]; i++ {
		commonSuffix++
	}

	// Show context before change
	startContext := commonPrefix - contextLines
	if startContext < 0 {
		startContext = 0
	}

	lineNum := startContext + 1 // 1-based line numbers

	// Context before
	for i := startContext; i < commonPrefix && i < len(oldLines); i++ {
		sb.WriteString(fmt.Sprintf("      %3d   %s\n", lineNum, oldLines[i]))
		lineNum++
	}

	// Show removed lines (from old) with {{-...}} marker
	removedEnd := len(oldLines) - commonSuffix
	for i := commonPrefix; i < removedEnd; i++ {
		sb.WriteString(fmt.Sprintf("{{-   %3d - %s}}\n", lineNum, oldLines[i]))
		lineNum++
	}

	// Reset line number for new lines at same position
	lineNum = commonPrefix + 1

	// Show added lines (from new) with {{+...}} marker
	addedEnd := len(newLines) - commonSuffix
	for i := commonPrefix; i < addedEnd; i++ {
		sb.WriteString(fmt.Sprintf("{{+   %3d + %s}}\n", lineNum, newLines[i]))
		lineNum++
	}

	// Context after (if any)
	if commonSuffix > 0 && commonSuffix <= contextLines {
		contextStart := len(newLines) - commonSuffix
		lineNum = contextStart + 1
		for i := contextStart; i < len(newLines); i++ {
			sb.WriteString(fmt.Sprintf("      %3d   %s\n", lineNum, newLines[i]))
			lineNum++
		}
	}

	return sb.String()
}

// truncateForDisplay truncates a string for display
func truncateForDisplay(s string, maxLen int) string {
	// Remove newlines for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func (p *AppPresenter) messagesToVM(messages []claude.Message) []ClaudeMessageVM {
	result := make([]ClaudeMessageVM, len(messages))
	for i, m := range messages {
		result[i] = ClaudeMessageVM{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp,
			TimeStr:   m.Timestamp.Format("060102 - 15:04:05"),
			IsPartial: m.Partial,
		}
	}
	return result
}
