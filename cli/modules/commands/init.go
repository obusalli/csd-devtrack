package commands

import (
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/config"
)

// AppContext holds application-wide context
type AppContext struct {
	Config         *config.Config
	ConfigPath     string
	ProjectService *projects.Service
	ProjectRepo    *projects.Repository
}

var globalContext *AppContext

// InitContext initializes the application context
func InitContext() error {
	cfg := config.GetGlobal()
	configPath := config.GetGlobalPath()

	// Create project repository
	projectRepo := projects.NewRepository(configPath)
	if err := projectRepo.Load(); err != nil {
		return err
	}

	// Create project service
	projectService := projects.NewService(projectRepo)

	globalContext = &AppContext{
		Config:         cfg,
		ConfigPath:     configPath,
		ProjectService: projectService,
		ProjectRepo:    projectRepo,
	}

	return nil
}

// GetContext returns the global application context
func GetContext() *AppContext {
	return globalContext
}

// registerCoreCommands registers core project management commands
func registerCoreCommands() {
	// Add command
	RegisterCommand(&Command{
		Name:        "add",
		Category:    "Project Management",
		Description: "Add a project from a path",
		Usage:       "csd-devtrack add <path> [--name <name>]",
		Examples: []string{
			"csd-devtrack add ../csd-core",
			"csd-devtrack add ../csd-stocks --name 'CSD Stocks'",
			"csd-devtrack add . --name 'Current Project'",
		},
		Handler: addCommand,
		Order:   10,
	})

	// List command
	RegisterCommand(&Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Category:    "Project Management",
		Description: "List all configured projects",
		Usage:       "csd-devtrack list [--json]",
		Examples: []string{
			"csd-devtrack list",
			"csd-devtrack ls --json",
		},
		Handler: listCommand,
		Order:   11,
	})

	// Remove command
	RegisterCommand(&Command{
		Name:        "remove",
		Aliases:     []string{"rm"},
		Category:    "Project Management",
		Description: "Remove a project from configuration",
		Usage:       "csd-devtrack remove <project-id>",
		Examples: []string{
			"csd-devtrack remove csd-core",
			"csd-devtrack rm csd-stocks",
		},
		Handler: removeCommand,
		Order:   12,
	})

	// Status command
	RegisterCommand(&Command{
		Name:        "status",
		Aliases:     []string{"st"},
		Category:    "Project Management",
		Description: "Show status of all projects",
		Usage:       "csd-devtrack status [project-id]",
		Examples: []string{
			"csd-devtrack status",
			"csd-devtrack st csd-core",
		},
		Handler: statusCommand,
		Order:   13,
	})

	// Refresh command
	RegisterCommand(&Command{
		Name:        "refresh",
		Category:    "Project Management",
		Description: "Re-detect project structure",
		Usage:       "csd-devtrack refresh <project-id>",
		Examples: []string{
			"csd-devtrack refresh csd-core",
		},
		Handler: refreshCommand,
		Order:   14,
	})
}

// registerBuildCommands registers build-related commands
func registerBuildCommands() {
	RegisterCommand(&Command{
		Name:        "build",
		Aliases:     []string{"b"},
		Category:    "Build",
		Description: "Build a project or component",
		Usage:       "csd-devtrack build [project] [component]",
		Examples: []string{
			"csd-devtrack build",
			"csd-devtrack build csd-core",
			"csd-devtrack build csd-core backend",
			"csd-devtrack b all",
		},
		SubCommands: []SubCommand{
			{Name: "all", Description: "Build all projects"},
		},
		Handler: buildCommand,
		Order:   20,
	})
}

// registerRunCommands registers run/process management commands
func registerRunCommands() {
	RegisterCommand(&Command{
		Name:        "run",
		Aliases:     []string{"start"},
		Category:    "Process Management",
		Description: "Start a project or component",
		Usage:       "csd-devtrack run <project> [component]",
		Examples: []string{
			"csd-devtrack run csd-core",
			"csd-devtrack run csd-core backend",
			"csd-devtrack start csd-stocks frontend",
		},
		Handler: runCommand,
		Order:   30,
	})

	RegisterCommand(&Command{
		Name:        "stop",
		Category:    "Process Management",
		Description: "Stop a running project or component",
		Usage:       "csd-devtrack stop <project> [component]",
		Examples: []string{
			"csd-devtrack stop csd-core",
			"csd-devtrack stop csd-core backend",
		},
		Handler: stopCommand,
		Order:   31,
	})

	RegisterCommand(&Command{
		Name:        "restart",
		Category:    "Process Management",
		Description: "Restart a project or component",
		Usage:       "csd-devtrack restart <project> [component]",
		Examples: []string{
			"csd-devtrack restart csd-core",
			"csd-devtrack restart csd-core backend",
		},
		Handler: restartCommand,
		Order:   32,
	})

	RegisterCommand(&Command{
		Name:        "kill",
		Category:    "Process Management",
		Description: "Force kill a project or component",
		Usage:       "csd-devtrack kill <project> [component] [--force]",
		Examples: []string{
			"csd-devtrack kill csd-core",
			"csd-devtrack kill csd-core --force",
		},
		Handler: killCommand,
		Order:   33,
	})

	RegisterCommand(&Command{
		Name:        "logs",
		Aliases:     []string{"log"},
		Category:    "Process Management",
		Description: "View logs for a project or component",
		Usage:       "csd-devtrack logs <project> [component] [--follow]",
		Examples: []string{
			"csd-devtrack logs csd-core",
			"csd-devtrack logs csd-core backend -f",
		},
		Handler: logsCommand,
		Order:   34,
	})
}

// registerGitCommands registers git-related commands
func registerGitCommands() {
	RegisterCommand(&Command{
		Name:        "git",
		Aliases:     []string{"g"},
		Category:    "Git",
		Description: "Git operations on projects",
		Usage:       "csd-devtrack git <subcommand> [project]",
		SubCommands: []SubCommand{
			{Name: "status", Description: "Show git status"},
			{Name: "diff", Description: "Show git diff"},
			{Name: "log", Description: "Show git log"},
		},
		Examples: []string{
			"csd-devtrack git status",
			"csd-devtrack git diff csd-core",
			"csd-devtrack g log csd-stocks",
		},
		Handler: gitCommand,
		Order:   40,
	})
}

// registerConfigCommands registers configuration commands
func registerConfigCommands() {
	RegisterCommand(&Command{
		Name:        "config",
		Aliases:     []string{"cfg"},
		Category:    "Configuration",
		Description: "Configuration management",
		Usage:       "csd-devtrack config <subcommand>",
		SubCommands: []SubCommand{
			{Name: "show", Description: "Show current configuration"},
			{Name: "edit", Description: "Edit configuration"},
			{Name: "path", Description: "Show config file path"},
			{Name: "init", Description: "Initialize configuration"},
		},
		Examples: []string{
			"csd-devtrack config show",
			"csd-devtrack config edit",
			"csd-devtrack cfg path",
		},
		Handler: configCommand,
		Order:   50,
	})
}

// registerUICommands registers UI-related commands
func registerUICommands() {
	RegisterCommand(&Command{
		Name:        "ui",
		Category:    "Interface",
		Description: "Launch the TUI interface",
		Usage:       "csd-devtrack ui",
		Examples: []string{
			"csd-devtrack ui",
		},
		Handler: uiCommand,
		Order:   60,
	})

	RegisterCommand(&Command{
		Name:        "shell",
		Aliases:     []string{"sh"},
		Category:    "Interface",
		Description: "Start interactive shell",
		Usage:       "csd-devtrack shell",
		Examples: []string{
			"csd-devtrack shell",
			"csd-devtrack sh",
		},
		Handler: shellCommand,
		Order:   61,
	})

	RegisterCommand(&Command{
		Name:        "server",
		Aliases:     []string{"srv"},
		Category:    "Interface",
		Description: "Start the web server only",
		Usage:       "csd-devtrack server [--port <port>]",
		Examples: []string{
			"csd-devtrack server",
			"csd-devtrack server --port 9099",
		},
		Handler: serverCommand,
		Order:   62,
	})
}
