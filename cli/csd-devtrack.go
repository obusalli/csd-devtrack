package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"csd-devtrack/cli/modules"
	"csd-devtrack/cli/modules/commands"
	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/platform/daemon"
	"csd-devtrack/cli/modules/platform/logger"
)

func main() {
	// Check if running as daemon process
	if daemon.IsDaemonMode() {
		runAsDaemon()
		return
	}

	// Parse global flags
	args := os.Args[1:]
	configPath := ""
	verbose := false
	noDaemon := false
	instanceName := ""

	// Extract global flags
	var cmdArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--config" || arg == "-c":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--verbose" || arg == "-v":
			verbose = true
		case arg == "--no-daemon":
			noDaemon = true
		case arg == "--name" || arg == "-n":
			if i+1 < len(args) {
				instanceName = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--name="):
			instanceName = strings.TrimPrefix(arg, "--name=")
		case arg == "--names":
			// List all daemon instances
			instances := daemon.ListInstances()
			if len(instances) == 0 {
				fmt.Println("No daemon instances running.")
			} else {
				fmt.Println("Daemon instances:")
				for _, name := range instances {
					// Check if running
					if name == "(default)" {
						daemon.SetInstanceName("")
					} else {
						daemon.SetInstanceName(name)
					}
					if daemon.IsRunning() {
						fmt.Printf("  ● %s (PID %d)\n", name, daemon.GetServerPID())
					} else {
						fmt.Printf("  ○ %s (stale)\n", name)
					}
				}
			}
			return
		case arg == "--kill":
			// Stop daemon gracefully - delay processing until after --name is parsed
			cmdArgs = append(cmdArgs, "__kill__")
		case arg == "--kill-force":
			// Force kill daemon - delay processing until after --name is parsed
			cmdArgs = append(cmdArgs, "__kill-force__")
		case arg == "--wipe":
			// Force cleanup of daemon files
			if err := daemon.Wipe(); err != nil {
				fmt.Fprintf(os.Stderr, "Wipe failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Daemon files cleaned up.")
			return
		case arg == "--version" || arg == "-V":
			printVersion()
			return
		case arg == "--help" || arg == "-h":
			printHelp()
			return
		default:
			cmdArgs = append(cmdArgs, arg)
		}
	}

	// Set instance name if specified (with validation)
	if instanceName != "" {
		if err := daemon.ValidateInstanceName(instanceName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		daemon.SetInstanceName(instanceName)
	}

	// Handle delayed --kill and --kill-force (after --name is processed)
	daemonName := "(default)"
	if instanceName != "" {
		daemonName = instanceName
	}
	for _, arg := range cmdArgs {
		switch arg {
		case "__kill__":
			if !daemon.IsRunning() {
				fmt.Printf("Daemon '%s' is not running\n", daemonName)
				return
			}
			if err := daemon.StopDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to stop daemon '%s': %v\n", daemonName, err)
				os.Exit(1)
			}
			fmt.Printf("Daemon '%s' stopped.\n", daemonName)
			return
		case "__kill-force__":
			if !daemon.IsRunning() {
				fmt.Printf("Daemon '%s' is not running\n", daemonName)
				return
			}
			if err := daemon.ForceKillDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to kill daemon '%s': %v\n", daemonName, err)
				os.Exit(1)
			}
			fmt.Printf("Daemon '%s' killed.\n", daemonName)
			return
		}
	}

	// Remove internal command markers from args
	cleanArgs := make([]string, 0, len(cmdArgs))
	for _, arg := range cmdArgs {
		if arg != "__kill__" && arg != "__kill-force__" {
			cleanArgs = append(cleanArgs, arg)
		}
	}
	cmdArgs = cleanArgs

	// Load configuration
	// If --config is explicitly specified, create the file if it doesn't exist
	explicitConfig := configPath != ""
	if configPath == "" {
		configPath = config.FindConfigFile()
	}

	if err := config.LoadGlobalWithCreate(configPath, explicitConfig); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}
	}

	// Initialize command registry
	commands.InitRegistry()

	// If no command specified, default to UI (with daemon)
	if len(cmdArgs) == 0 {
		cmdArgs = []string{"ui"}
	}

	// Find and execute command
	cmdName := cmdArgs[0]
	cmdRemainingArgs := cmdArgs[1:]

	// Handle special commands
	switch cmdName {
	case "version":
		printVersion()
		return
	case "help":
		if len(cmdRemainingArgs) > 0 {
			commands.PrintCommandHelp(cmdRemainingArgs[0])
		} else {
			printHelp()
		}
		return
	case "daemon":
		// Daemon subcommands
		handleDaemonCommand(cmdRemainingArgs)
		return
	}

	// For UI command, handle daemon mode
	if cmdName == "ui" && !noDaemon {
		commands.SetDaemonMode(true)
	}

	// Look up command in registry
	cmd := commands.GetCommand(cmdName)
	if cmd == nil {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmdName)
		fmt.Fprintf(os.Stderr, "Run 'csd-devtrack help' for usage.\n")
		os.Exit(1)
	}

	// Execute command
	if err := cmd.Handler(cmdRemainingArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runAsDaemon runs as the background daemon process
func runAsDaemon() {
	// Parse daemon-specific args
	configPath := ""
	instanceName := ""
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--config" || arg == "-c":
			if i+1 < len(os.Args) {
				configPath = os.Args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--name" || arg == "-n":
			if i+1 < len(os.Args) {
				instanceName = os.Args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--name="):
			instanceName = strings.TrimPrefix(arg, "--name=")
		}
	}

	// Set instance name if specified
	if instanceName != "" {
		daemon.SetInstanceName(instanceName)
	}

	// Load config
	if configPath == "" {
		configPath = config.FindConfigFile()
	}
	if err := config.LoadGlobalWithCreate(configPath, configPath != ""); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize context
	if err := commands.InitContext(); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon: Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	appCtx := commands.GetContext()

	// Create presenter
	presenter := commands.CreatePresenter(appCtx)

	// Initialize presenter to populate state (projects, processes, git, etc.)
	ctx := context.Background()
	if err := presenter.Initialize(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon: Failed to initialize presenter: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger from config
	cfg := config.GetGlobal()
	loggerCfg := cfg.Settings.GetLoggerConfig()

	logDir := filepath.Join(os.Getenv("HOME"), ".csd-devtrack")
	var logOutputs []io.Writer

	// Determine log file path
	logFilePath := loggerCfg.FilePath
	if logFilePath == "" {
		logFilePath = filepath.Join(logDir, "daemon.log")
	}

	// Create log file
	if logFile, err := logger.CreateLogFile(logFilePath, loggerCfg.MaxSizeMB); err == nil {
		logOutputs = append(logOutputs, logFile)
		defer logFile.Close()
	}

	// Parse log level from config
	logLevel := logger.ParseLevel(loggerCfg.Level)
	log := logger.NewLogger(logLevel, logOutputs, "csd-devtrack")
	logger.SetGlobalLogger(log)

	// Create daemon server
	server := daemon.NewServer(presenter)

	// Connect logger to server for broadcasting
	log.SetBroadcaster(server)

	// Capture stdout/stderr and redirect through logger based on config
	// This must be done AFTER setting the broadcaster
	if loggerCfg.CaptureStderr {
		if _, err := logger.CaptureStderr(log); err != nil {
			log.Warn("Failed to capture stderr: %v", err)
		}
	}
	if loggerCfg.CaptureStdout {
		if _, err := logger.CaptureStdout(log); err != nil {
			log.Warn("Failed to capture stdout: %v", err)
		}
	}

	log.Info("Daemon starting...")

	if err := server.Start(); err != nil {
		log.Error("Failed to start: %v", err)
		os.Exit(1)
	}

	log.Info("Daemon started successfully")

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	<-sigCh

	// Graceful shutdown
	server.Stop()
	presenter.Shutdown()
}

// handleDaemonCommand handles daemon subcommands
func handleDaemonCommand(args []string) {
	if len(args) == 0 {
		fmt.Println(daemon.Status())
		return
	}

	switch args[0] {
	case "status":
		fmt.Println(daemon.Status())

	case "start":
		if daemon.IsRunning() {
			fmt.Printf("Daemon already running (PID %d)\n", daemon.GetServerPID())
			return
		}
		pid, err := daemon.StartDaemon()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Daemon started (PID %d)\n", pid)

	case "stop":
		if !daemon.IsRunning() {
			fmt.Println("Daemon is not running")
			return
		}
		if err := daemon.StopDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Daemon stopped")

	case "restart":
		if daemon.IsRunning() {
			if err := daemon.StopDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to stop daemon: %v\n", err)
				os.Exit(1)
			}
		}
		pid, err := daemon.StartDaemon()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Daemon restarted (PID %d)\n", pid)

	case "wipe":
		if err := daemon.Wipe(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to wipe: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Daemon files cleaned up")

	default:
		fmt.Fprintf(os.Stderr, "Unknown daemon subcommand: %s\n", args[0])
		fmt.Println("Usage: csd-devtrack daemon <status|start|stop|restart|wipe>")
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("%s version %s\n", modules.AppName, modules.AppVersion)
	fmt.Printf("Client build: %s\n", modules.BuildHash())

	// Check if daemon is running and show its version
	if daemon.IsRunning() {
		daemonVersion, daemonHash := daemon.GetDaemonVersion()
		if daemonHash != "" {
			fmt.Printf("Daemon build: %s", daemonHash)
			if daemonHash != modules.BuildHash() {
				fmt.Printf(" (version mismatch!)")
			}
			fmt.Println()
			if daemonVersion != "" && daemonVersion != modules.AppVersion {
				fmt.Printf("Daemon version: %s\n", daemonVersion)
			}
		} else {
			fmt.Printf("Daemon: running (PID %d), version unknown\n", daemon.GetServerPID())
		}
	} else {
		fmt.Println("Daemon: not running")
	}
}

func printHelp() {
	fmt.Printf("%s - %s\n", modules.AppName, modules.AppDescription)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  csd-devtrack [flags] [command] [arguments]")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  -c, --config <path>    Path to config file")
	fmt.Println("  -n, --name <name>      Daemon instance name (for multi-instance)")
	fmt.Println("  -v, --verbose          Verbose output")
	fmt.Println("  -V, --version          Print version")
	fmt.Println("  -h, --help             Print help")
	fmt.Println("      --no-daemon        Run without daemon mode")
	fmt.Println()
	fmt.Println("Daemon Management:")
	fmt.Println("      --names            List all daemon instances")
	fmt.Println("      --kill             Stop the daemon gracefully")
	fmt.Println("      --kill-force       Force kill the daemon")
	fmt.Println("      --wipe             Clean up stale daemon files")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println()

	// Print commands by category
	commands.PrintCommands()

	fmt.Println()
	fmt.Println("Use 'csd-devtrack help <command>' for more information about a command.")
}
