package main

import (
	"fmt"
	"os"
	"strings"

	"csd-devtrack/cli/modules/commands"
	"csd-devtrack/cli/modules/platform/config"
)

const (
	Version   = "0.1.0"
	BuildDate = "development"
)

func main() {
	// Parse global flags
	args := os.Args[1:]
	configPath := ""
	verbose := false

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

	// Load configuration
	if configPath == "" {
		configPath = config.FindConfigFile()
	}

	if err := config.LoadGlobal(configPath); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}
	}

	// Initialize command registry
	commands.InitRegistry()

	// If no command specified, show help or start shell
	if len(cmdArgs) == 0 {
		// Default to interactive shell
		cmdArgs = []string{"shell"}
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

func printVersion() {
	fmt.Printf("csd-devtrack version %s\n", Version)
	fmt.Printf("Build: %s\n", BuildDate)
}

func printHelp() {
	fmt.Println("csd-devtrack - Multi-project development tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  csd-devtrack [flags] [command] [arguments]")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  -c, --config <path>    Path to config file")
	fmt.Println("  -v, --verbose          Verbose output")
	fmt.Println("  -V, --version          Print version")
	fmt.Println("  -h, --help             Print help")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println()

	// Print commands by category
	commands.PrintCommands()

	fmt.Println()
	fmt.Println("Use 'csd-devtrack help <command>' for more information about a command.")
}
