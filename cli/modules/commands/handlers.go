package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/builder"
	"csd-devtrack/cli/modules/platform/git"
	"csd-devtrack/cli/modules/platform/server"
	"csd-devtrack/cli/modules/platform/supervisor"
	uicore "csd-devtrack/cli/modules/ui/core"
	"csd-devtrack/cli/modules/ui/tui"
)

// addCommand handles the 'add' command
func addCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("path is required\nUsage: csd-devtrack add <path> [--name <name>]")
	}

	path := args[0]
	name := ""

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		if args[i] == "--name" && i+1 < len(args) {
			name = args[i+1]
			i++
		}
	}

	ctx := GetContext()
	var project *projects.Project
	var err error

	if name != "" {
		project, err = ctx.ProjectService.AddProjectWithName(path, name)
	} else {
		project, err = ctx.ProjectService.AddProject(path)
	}

	if err != nil {
		return fmt.Errorf("failed to add project: %w", err)
	}

	fmt.Printf("Added project: %s (%s)\n", project.Name, project.ID)
	fmt.Printf("Type: %s\n", project.Type)
	fmt.Printf("Path: %s\n", project.Path)
	fmt.Printf("Components:\n")
	for ct, comp := range project.Components {
		if comp.Enabled {
			fmt.Printf("  - %s: %s\n", ct, comp.Path)
		}
	}

	return nil
}

// listCommand handles the 'list' command
func listCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		}
	}

	ctx := GetContext()
	projectsList := ctx.ProjectService.ListProjects()

	if len(projectsList) == 0 {
		fmt.Println("No projects configured.")
		fmt.Println("Use 'csd-devtrack add <path>' to add a project.")
		return nil
	}

	if jsonOutput {
		data, err := json.MarshalIndent(projectsList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal projects: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Projects (%d):\n\n", len(projectsList))
	for _, p := range projectsList {
		selfMarker := ""
		if p.Self {
			selfMarker = " [self]"
		}

		fmt.Printf("  %s%s\n", p.Name, selfMarker)
		fmt.Printf("    ID:   %s\n", p.ID)
		fmt.Printf("    Type: %s\n", p.Type)
		fmt.Printf("    Path: %s\n", p.Path)

		components := []string{}
		for ct, comp := range p.Components {
			if comp.Enabled {
				components = append(components, string(ct))
			}
		}
		fmt.Printf("    Components: %s\n", strings.Join(components, ", "))
		fmt.Println()
	}

	return nil
}

// removeCommand handles the 'remove' command
func removeCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project ID is required\nUsage: csd-devtrack remove <project-id>")
	}

	projectID := args[0]
	ctx := GetContext()

	// Check if project exists
	project, err := ctx.ProjectService.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %s", projectID)
	}

	if err := ctx.ProjectService.RemoveProject(projectID); err != nil {
		return fmt.Errorf("failed to remove project: %w", err)
	}

	fmt.Printf("Removed project: %s (%s)\n", project.Name, project.ID)
	return nil
}

// statusCommand handles the 'status' command
func statusCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	ctx := GetContext()

	if len(args) > 0 {
		// Show status for specific project
		projectID := args[0]
		project, err := ctx.ProjectService.GetProject(projectID)
		if err != nil {
			return fmt.Errorf("project not found: %s", projectID)
		}

		printProjectStatus(project)
		return nil
	}

	// Show status for all projects
	projectsList := ctx.ProjectService.ListProjects()
	if len(projectsList) == 0 {
		fmt.Println("No projects configured.")
		return nil
	}

	fmt.Println("Project Status:")
	fmt.Println()

	for _, p := range projectsList {
		printProjectStatus(p)
		fmt.Println()
	}

	return nil
}

func printProjectStatus(p *projects.Project) {
	selfMarker := ""
	if p.Self {
		selfMarker = " [self]"
	}

	fmt.Printf("%s%s\n", p.Name, selfMarker)
	fmt.Printf("  ID:   %s\n", p.ID)
	fmt.Printf("  Type: %s\n", p.Type)

	// Git status (placeholder - will be implemented with git module)
	if p.GitBranch != "" {
		fmt.Printf("  Git:  %s", p.GitBranch)
		if p.GitDirty {
			fmt.Print(" (dirty)")
		}
		fmt.Println()
	}

	// Components status
	fmt.Println("  Components:")
	for ct, comp := range p.Components {
		if comp.Enabled {
			status := "stopped"
			// TODO: Get actual process status
			fmt.Printf("    %s: %s\n", ct, status)
		}
	}
}

// refreshCommand handles the 'refresh' command
func refreshCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project ID is required\nUsage: csd-devtrack refresh <project-id>")
	}

	projectID := args[0]
	ctx := GetContext()

	project, err := ctx.ProjectService.RefreshProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to refresh project: %w", err)
	}

	fmt.Printf("Refreshed project: %s\n", project.Name)
	fmt.Printf("Type: %s\n", project.Type)
	fmt.Printf("Components:\n")
	for ct, comp := range project.Components {
		if comp.Enabled {
			fmt.Printf("  - %s: %s\n", ct, comp.Path)
		}
	}

	return nil
}

// buildCommand handles the 'build' command
func buildCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	ctx := GetContext()
	orchestrator := builder.NewOrchestrator(ctx.ProjectService, ctx.Config.Settings.ParallelBuilds)

	// Set up event handler for real-time output
	orchestrator.SetEventHandler(func(event builds.BuildEvent) {
		timestamp := event.Timestamp.Format("15:04:05")
		switch event.Type {
		case builds.BuildEventStarted:
			fmt.Printf("[%s] ▶ %s\n", timestamp, event.Message)
		case builds.BuildEventOutput:
			fmt.Printf("[%s]   %s\n", timestamp, event.Message)
		case builds.BuildEventError:
			fmt.Printf("[%s] ✗ %s\n", timestamp, event.Message)
		case builds.BuildEventWarning:
			fmt.Printf("[%s] ⚠ %s\n", timestamp, event.Message)
		case builds.BuildEventFinished:
			fmt.Printf("[%s] ✓ %s\n", timestamp, event.Message)
		}
	})

	buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if len(args) == 0 || args[0] == "all" {
		// Build all projects
		fmt.Println("Building all projects...")
		fmt.Println()

		results, err := orchestrator.BuildAll(buildCtx)
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		summary := orchestrator.Summarize(results)
		orchestrator.PrintSummary(summary)

		if summary.FailedComponents > 0 {
			return fmt.Errorf("build completed with %d failures", summary.FailedComponents)
		}
		return nil
	}

	projectID := args[0]
	componentStr := ""
	if len(args) > 1 {
		componentStr = args[1]
	}

	if componentStr != "" {
		// Build specific component
		componentType := projects.ComponentType(componentStr)
		fmt.Printf("Building %s/%s...\n", projectID, componentType)
		fmt.Println()

		result := orchestrator.BuildComponent(buildCtx, projectID, componentType)
		if result.Error != nil {
			return fmt.Errorf("build failed: %w", result.Error)
		}

		if result.Build != nil {
			printBuildResult(result.Build)
		}
		return nil
	}

	// Build all components of a project
	fmt.Printf("Building %s...\n", projectID)
	fmt.Println()

	results, err := orchestrator.BuildProject(buildCtx, projectID)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	for _, result := range results {
		if result.Build != nil {
			printBuildResult(result.Build)
		}
	}

	return nil
}

func printBuildResult(build *builds.Build) {
	statusIcon := "✓"
	if !build.IsSuccess() {
		statusIcon = "✗"
	}

	fmt.Printf("%s %s/%s - %s (%s)\n",
		statusIcon,
		build.ProjectID,
		build.Component,
		build.Status,
		build.Duration.Round(time.Millisecond),
	)

	if len(build.Errors) > 0 {
		fmt.Println("  Errors:")
		for _, err := range build.Errors {
			fmt.Printf("    %s\n", err)
		}
	}

	if build.Artifact != "" {
		fmt.Printf("  Artifact: %s\n", build.Artifact)
	}
}

// Global process service and manager (lazy initialized)
var (
	globalProcessService *processes.Service
	globalSupervisor     *supervisor.Manager
)

func getProcessManager() (*processes.Service, *supervisor.Manager) {
	if globalProcessService == nil {
		ctx := GetContext()
		globalProcessService = processes.NewService(ctx.ProjectService)
		globalSupervisor = supervisor.NewManager(globalProcessService)

		// Set up event handler
		globalProcessService.SetEventHandler(func(event processes.ProcessEvent) {
			timestamp := event.Timestamp.Format("15:04:05")
			switch event.Type {
			case processes.ProcessEventStarting:
				fmt.Printf("[%s] ▶ %s\n", timestamp, event.Message)
			case processes.ProcessEventStarted:
				fmt.Printf("[%s] ✓ %s\n", timestamp, event.Message)
			case processes.ProcessEventStopping:
				fmt.Printf("[%s] ◼ %s\n", timestamp, event.Message)
			case processes.ProcessEventStopped:
				fmt.Printf("[%s] ✓ %s\n", timestamp, event.Message)
			case processes.ProcessEventCrashed:
				fmt.Printf("[%s] ✗ %s\n", timestamp, event.Message)
			case processes.ProcessEventOutput:
				fmt.Printf("[%s] %s | %s\n", timestamp, event.ProcessID, event.Message)
			case processes.ProcessEventError:
				fmt.Printf("[%s] %s | ERROR: %s\n", timestamp, event.ProcessID, event.Message)
			}
		})
	}
	return globalProcessService, globalSupervisor
}

// runCommand handles the 'run' command
func runCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project is required\nUsage: csd-devtrack run <project> [component]")
	}

	projectID := args[0]
	componentStr := ""
	if len(args) > 1 {
		componentStr = args[1]
	}

	ctx := GetContext()
	processService, mgr := getProcessManager()

	runCtx := context.Background()

	if componentStr != "" {
		// Start specific component
		componentType := projects.ComponentType(componentStr)
		fmt.Printf("Starting %s/%s...\n", projectID, componentType)

		if err := processService.StartComponent(runCtx, projectID, componentType, mgr); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}
	} else {
		// Start all components
		fmt.Printf("Starting all components of %s...\n", projectID)

		project, err := ctx.ProjectService.GetProject(projectID)
		if err != nil {
			return fmt.Errorf("project not found: %s", projectID)
		}

		for _, comp := range project.GetEnabledComponents() {
			if err := processService.StartComponent(runCtx, projectID, comp.Type, mgr); err != nil {
				fmt.Printf("Warning: failed to start %s: %v\n", comp.Type, err)
			}
		}
	}

	fmt.Println("\nProcesses started. Use 'logs' to view output, 'stop' to stop.")
	return nil
}

// stopCommand handles the 'stop' command
func stopCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project is required\nUsage: csd-devtrack stop <project> [component]")
	}

	projectID := args[0]
	componentStr := ""
	if len(args) > 1 {
		componentStr = args[1]
	}

	processService, mgr := getProcessManager()
	stopCtx := context.Background()

	if componentStr != "" {
		// Stop specific component
		componentType := projects.ComponentType(componentStr)
		processID := projectID + "/" + string(componentType)
		fmt.Printf("Stopping %s...\n", processID)

		if err := processService.StopProcess(stopCtx, processID, mgr, false); err != nil {
			return fmt.Errorf("failed to stop: %w", err)
		}
	} else {
		// Stop all components
		fmt.Printf("Stopping all components of %s...\n", projectID)

		if err := processService.StopProject(stopCtx, projectID, mgr, false); err != nil {
			return fmt.Errorf("failed to stop project: %w", err)
		}
	}

	fmt.Println("Stopped successfully.")
	return nil
}

// restartCommand handles the 'restart' command
func restartCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project is required\nUsage: csd-devtrack restart <project> [component]")
	}

	projectID := args[0]
	componentStr := ""
	if len(args) > 1 {
		componentStr = args[1]
	}

	processService, mgr := getProcessManager()
	restartCtx := context.Background()

	if componentStr != "" {
		// Restart specific component
		componentType := projects.ComponentType(componentStr)
		processID := projectID + "/" + string(componentType)
		fmt.Printf("Restarting %s...\n", processID)

		if err := processService.RestartProcess(restartCtx, processID, mgr); err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}
	} else {
		// Restart all components
		fmt.Printf("Restarting all components of %s...\n", projectID)

		procs := processService.GetProcessesForProject(projectID)
		for _, proc := range procs {
			if err := processService.RestartProcess(restartCtx, proc.ID, mgr); err != nil {
				fmt.Printf("Warning: failed to restart %s: %v\n", proc.ID, err)
			}
		}
	}

	fmt.Println("Restarted successfully.")
	return nil
}

// killCommand handles the 'kill' command
func killCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project is required\nUsage: csd-devtrack kill <project> [component]")
	}

	projectID := args[0]
	componentStr := ""
	force := false

	for i := 1; i < len(args); i++ {
		if args[i] == "--force" || args[i] == "-f" {
			force = true
		} else if componentStr == "" {
			componentStr = args[i]
		}
	}

	processService, mgr := getProcessManager()

	if componentStr != "" {
		// Kill specific component
		componentType := projects.ComponentType(componentStr)
		processID := projectID + "/" + string(componentType)
		fmt.Printf("Killing %s...\n", processID)

		if force {
			if err := processService.KillProcess(processID, mgr); err != nil {
				return fmt.Errorf("failed to kill: %w", err)
			}
		} else {
			killCtx := context.Background()
			if err := processService.StopProcess(killCtx, processID, mgr, true); err != nil {
				return fmt.Errorf("failed to kill: %w", err)
			}
		}
	} else {
		// Kill all components
		fmt.Printf("Killing all components of %s...\n", projectID)

		procs := processService.GetProcessesForProject(projectID)
		for _, proc := range procs {
			if err := processService.KillProcess(proc.ID, mgr); err != nil {
				fmt.Printf("Warning: failed to kill %s: %v\n", proc.ID, err)
			}
		}
	}

	fmt.Println("Killed.")
	return nil
}

// logsCommand handles the 'logs' command
func logsCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("project is required\nUsage: csd-devtrack logs <project> [component] [--follow]")
	}

	projectID := args[0]
	componentStr := ""
	follow := false
	lines := 50

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--follow", "-f":
			follow = true
		case "--lines", "-n":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &lines)
				i++
			}
		default:
			if componentStr == "" {
				componentStr = args[i]
			}
		}
	}

	processService, _ := getProcessManager()

	var procs []*processes.Process
	if componentStr != "" {
		componentType := projects.ComponentType(componentStr)
		proc := processService.GetProcessForComponent(projectID, componentType)
		if proc != nil {
			procs = append(procs, proc)
		}
	} else {
		procs = processService.GetProcessesForProject(projectID)
	}

	if len(procs) == 0 {
		fmt.Println("No processes found for the specified project/component.")
		fmt.Println("Use 'run' to start processes first.")
		return nil
	}

	// Print existing logs
	for _, proc := range procs {
		logs := proc.GetLogs(lines)
		if len(logs) > 0 {
			fmt.Printf("=== %s ===\n", proc.ID)
			for _, line := range logs {
				fmt.Println(line)
			}
			fmt.Println()
		}
	}

	if follow {
		fmt.Println("Following logs... (Ctrl+C to exit)")
		// For now, just wait - in a real implementation, we'd use channels
		select {}
	}

	return nil
}

// gitCommand handles the 'git' command
func gitCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("subcommand is required\nUsage: csd-devtrack git <status|diff|log> [project]")
	}

	subCmd := args[0]
	projectID := ""
	if len(args) > 1 {
		projectID = args[1]
	}

	ctx := GetContext()
	gitService := git.NewService(ctx.ProjectService)

	switch subCmd {
	case "status":
		return gitStatusCommand(gitService, projectID)
	case "diff":
		return gitDiffCommand(gitService, projectID, args[2:])
	case "log":
		return gitLogCommand(gitService, projectID, args[2:])
	default:
		return fmt.Errorf("unknown git subcommand: %s", subCmd)
	}
}

func gitStatusCommand(gitService *git.Service, projectID string) error {
	if projectID == "" {
		// Show status for all projects
		allStatus := gitService.GetAllStatus()
		if len(allStatus) == 0 {
			fmt.Println("No projects with git repositories found.")
			return nil
		}

		fmt.Println("Git Status:")
		fmt.Println()

		for pid, status := range allStatus {
			printGitStatus(pid, status)
			fmt.Println()
		}
		return nil
	}

	// Show status for specific project
	status, err := gitService.GetStatus(projectID)
	if err != nil {
		return fmt.Errorf("failed to get git status: %w", err)
	}

	printGitStatus(projectID, status)
	return nil
}

func printGitStatus(projectID string, status *git.Status) {
	branchIcon := "⎇"
	if status.IsClean && !status.HasUntracked {
		branchIcon = "✓"
	} else {
		branchIcon = "●"
	}

	fmt.Printf("%s %s [%s]\n", branchIcon, projectID, status.Branch)

	if status.Ahead > 0 || status.Behind > 0 {
		fmt.Printf("  ↑%d ↓%d\n", status.Ahead, status.Behind)
	}

	if len(status.Staged) > 0 {
		fmt.Printf("  Staged (%d):\n", len(status.Staged))
		for _, f := range status.Staged {
			fmt.Printf("    + %s\n", f)
		}
	}

	if len(status.Modified) > 0 {
		fmt.Printf("  Modified (%d):\n", len(status.Modified))
		for _, f := range status.Modified {
			fmt.Printf("    M %s\n", f)
		}
	}

	if len(status.Deleted) > 0 {
		fmt.Printf("  Deleted (%d):\n", len(status.Deleted))
		for _, f := range status.Deleted {
			fmt.Printf("    D %s\n", f)
		}
	}

	if len(status.Untracked) > 0 {
		fmt.Printf("  Untracked (%d):\n", len(status.Untracked))
		for _, f := range status.Untracked {
			fmt.Printf("    ? %s\n", f)
		}
	}

	if status.IsClean && !status.HasUntracked {
		fmt.Println("  Working tree clean")
	}
}

func gitDiffCommand(gitService *git.Service, projectID string, args []string) error {
	if projectID == "" {
		return fmt.Errorf("project ID is required for diff")
	}

	opts := git.DefaultDiffOptions()

	// Parse options
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--staged", "-s":
			opts.Staged = true
		default:
			if opts.Path == "" {
				opts.Path = args[i]
			}
		}
	}

	diff, err := gitService.GetDiff(projectID, opts)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	if diff.FileCount == 0 {
		fmt.Println("No changes to show.")
		return nil
	}

	fmt.Printf("Files changed: %d\n\n", diff.FileCount)

	for _, file := range diff.Files {
		statusLabel := ""
		switch file.Status {
		case "A":
			statusLabel = "new file"
		case "M":
			statusLabel = "modified"
		case "D":
			statusLabel = "deleted"
		case "R":
			statusLabel = "renamed"
		case "?":
			statusLabel = "untracked"
		}
		fmt.Printf("  %s: %s\n", statusLabel, file.Path)
	}

	return nil
}

func gitLogCommand(gitService *git.Service, projectID string, args []string) error {
	if projectID == "" {
		return fmt.Errorf("project ID is required for log")
	}

	opts := git.DefaultLogOptions()

	// Parse options
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--max":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &opts.MaxCount)
				i++
			}
		}
	}

	commits, err := gitService.GetLog(projectID, opts)
	if err != nil {
		return fmt.Errorf("failed to get log: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No commits found.")
		return nil
	}

	fmt.Printf("Recent commits (%d):\n\n", len(commits))

	for _, commit := range commits {
		date := commit.Date.Format("2006-01-02 15:04")
		fmt.Printf("\033[33m%s\033[0m %s\n", commit.ShortHash, commit.Subject)
		fmt.Printf("        %s <%s> - %s\n", commit.Author, commit.AuthorEmail, date)
	}

	return nil
}

// configCommand handles the 'config' command
func configCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand is required\nUsage: csd-devtrack config <show|edit|path|init>")
	}

	subCmd := args[0]

	switch subCmd {
	case "show":
		return configShowCommand(args[1:])
	case "edit":
		return configEditCommand(args[1:])
	case "path":
		return configPathCommand(args[1:])
	case "init":
		return configInitCommand(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", subCmd)
	}
}

func configShowCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	ctx := GetContext()
	cfg := ctx.Config

	data, err := json.MarshalIndent(cfg.Settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("Configuration:")
	fmt.Println(string(data))
	return nil
}

func configEditCommand(args []string) error {
	// TODO: Open config file in editor
	fmt.Println("Config edit not yet implemented.")
	return nil
}

func configPathCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	ctx := GetContext()
	fmt.Println(ctx.ConfigPath)
	return nil
}

func configInitCommand(args []string) error {
	// TODO: Initialize config file
	fmt.Println("Config init not yet implemented.")
	return nil
}

// uiCommand handles the 'ui' command
func uiCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	appCtx := GetContext()

	// Create the presenter with services
	presenter := uicore.NewPresenter(appCtx.ProjectService, appCtx.Config)

	// Initialize presenter
	ctx := context.Background()
	if err := presenter.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize presenter: %w", err)
	}
	defer presenter.Shutdown()

	// Create and run TUI view
	tuiView := tui.NewTUIView()
	if err := tuiView.Initialize(presenter); err != nil {
		return fmt.Errorf("failed to initialize TUI: %w", err)
	}

	// Run the TUI (blocking)
	if err := tuiView.Run(ctx); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// shellCommand handles the 'shell' command
func shellCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Start interactive shell
	return StartShell()
}

// serverCommand handles the 'server' command
func serverCommand(args []string) error {
	if err := InitContext(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	appCtx := GetContext()

	srv := server.NewServer(appCtx.ProjectService, appCtx.Config)

	if err := srv.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Printf("Web server running on http://localhost:%d\n", srv.GetPort())
	fmt.Println("Press Ctrl+C to stop")

	// Wait forever (Ctrl+C will terminate the process)
	select {}
}
