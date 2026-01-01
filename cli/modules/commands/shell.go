package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

const (
	historyFile     = ".devtrack_history"
	maxHistoryLines = 1000
)

// Shell represents the interactive shell
type Shell struct {
	rl       *readline.Instance
	isTTY    bool
	running  bool
}

// StartShell starts the interactive shell
func StartShell() error {
	shell := &Shell{
		isTTY: term.IsTerminal(int(os.Stdin.Fd())),
	}

	return shell.Run()
}

// Run starts the shell main loop
func (s *Shell) Run() error {
	s.running = true

	if s.isTTY {
		return s.runInteractive()
	}
	return s.runNonInteractive()
}

// runInteractive runs the shell with readline support
func (s *Shell) runInteractive() error {
	// Build completer
	completer := s.buildCompleter()

	// Get history file path
	historyPath := s.getHistoryPath()

	// Create readline instance
	config := &readline.Config{
		Prompt:          s.getPrompt(),
		HistoryFile:     historyPath,
		HistoryLimit:    maxHistoryLines,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	s.rl = rl

	// Print welcome message
	s.printWelcome()

	// Main loop
	for s.running {
		// Update prompt
		rl.SetPrompt(s.getPrompt())

		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(line) == 0 {
					fmt.Println("Use 'exit' or 'quit' to leave the shell.")
					continue
				}
				continue
			}
			if err == io.EOF {
				fmt.Println()
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle special commands
		if s.handleSpecialCommand(line) {
			continue
		}

		// Execute command
		s.executeCommand(line)
	}

	return nil
}

// runNonInteractive runs the shell without readline (for pipes/non-TTY)
func (s *Shell) runNonInteractive() error {
	scanner := bufio.NewScanner(os.Stdin)

	for s.running && scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if s.handleSpecialCommand(line) {
			continue
		}

		s.executeCommand(line)
	}

	return scanner.Err()
}

// getPrompt returns the current prompt
func (s *Shell) getPrompt() string {
	timestamp := time.Now().Format("15:04:05")

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "devtrack"
	}

	// Get current user
	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}

	if s.isTTY {
		// Colored prompt for TTY
		return fmt.Sprintf("\033[90m[%s]\033[0m \033[36m%s\033[0m@\033[33m%s\033[0m \033[32mdevtrack>\033[0m ",
			timestamp, user, hostname)
	}

	// Plain prompt for non-TTY
	return fmt.Sprintf("[%s] %s@%s devtrack> ", timestamp, user, hostname)
}

// printWelcome prints the welcome message
func (s *Shell) printWelcome() {
	fmt.Println()
	fmt.Println("\033[36m  ╔═══════════════════════════════════════════╗\033[0m")
	fmt.Println("\033[36m  ║\033[0m       \033[1m\033[33mCSD DevTrack\033[0m - Dev Tool Shell       \033[36m║\033[0m")
	fmt.Println("\033[36m  ╚═══════════════════════════════════════════╝\033[0m")
	fmt.Println()
	fmt.Println("  Type 'help' for available commands, 'exit' to quit.")
	fmt.Println()
}

// handleSpecialCommand handles shell-specific commands
func (s *Shell) handleSpecialCommand(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return true
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "exit", "quit", "q":
		fmt.Println("Goodbye!")
		s.running = false
		return true

	case "clear", "cls":
		s.clearScreen()
		return true

	case "history":
		s.showHistory(parts[1:])
		return true
	}

	// Handle shell escape (!)
	if strings.HasPrefix(line, "!") {
		s.executeShellCommand(strings.TrimPrefix(line, "!"))
		return true
	}

	return false
}

// executeCommand executes a devtrack command
func (s *Shell) executeCommand(line string) {
	parts := parseCommandLine(line)
	if len(parts) == 0 {
		return
	}

	cmdName := parts[0]
	args := parts[1:]

	// Handle help
	if cmdName == "help" {
		if len(args) > 0 {
			PrintCommandHelp(args[0])
		} else {
			PrintCommands()
		}
		return
	}

	// Look up command
	cmd := GetCommand(cmdName)
	if cmd == nil {
		fmt.Printf("Unknown command: %s\n", cmdName)
		fmt.Println("Type 'help' for available commands.")
		return
	}

	// Execute command
	if err := cmd.Handler(args); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// executeShellCommand executes an OS shell command
func (s *Shell) executeShellCommand(cmdLine string) {
	cmdLine = strings.TrimSpace(cmdLine)
	if cmdLine == "" {
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdLine)
	} else {
		cmd = exec.Command("sh", "-c", cmdLine)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Shell error: %v\n", err)
	}
}

// clearScreen clears the terminal screen
func (s *Shell) clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		fmt.Print("\033[2J\033[H")
	}
}

// showHistory shows command history
func (s *Shell) showHistory(args []string) {
	if s.rl == nil {
		fmt.Println("History not available in non-interactive mode.")
		return
	}

	// Get history
	historyPath := s.getHistoryPath()
	data, err := os.ReadFile(historyPath)
	if err != nil {
		fmt.Println("No history available.")
		return
	}

	lines := strings.Split(string(data), "\n")

	// Apply search filter if provided
	searchTerm := ""
	if len(args) > 0 {
		searchTerm = strings.ToLower(args[0])
	}

	count := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if searchTerm != "" && !strings.Contains(strings.ToLower(line), searchTerm) {
			continue
		}

		fmt.Printf("%4d  %s\n", i+1, line)
		count++

		if count >= 50 {
			fmt.Println("... (showing last 50 entries)")
			break
		}
	}
}

// getHistoryPath returns the path to the history file
func (s *Shell) getHistoryPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return historyFile
	}
	return homeDir + "/" + historyFile
}

// buildCompleter builds the readline completer
func (s *Shell) buildCompleter() *readline.PrefixCompleter {
	items := []readline.PrefixCompleterInterface{
		readline.PcItem("help",
			readline.PcItemDynamic(func(line string) []string {
				return GetCommandNames()
			}),
		),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
		readline.PcItem("clear"),
		readline.PcItem("history"),
	}

	// Add all commands
	for _, cmd := range GetAllCommands() {
		item := readline.PcItem(cmd.Name)

		// Add subcommands if any
		if len(cmd.SubCommands) > 0 {
			subItems := make([]readline.PrefixCompleterInterface, 0, len(cmd.SubCommands))
			for _, sub := range cmd.SubCommands {
				subItems = append(subItems, readline.PcItem(sub.Name))
			}
			item = readline.PcItem(cmd.Name, subItems...)
		}

		items = append(items, item)
	}

	return readline.NewPrefixCompleter(items...)
}

// parseCommandLine parses a command line into parts
func parseCommandLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range line {
		switch {
		case ch == '"' || ch == '\'':
			if inQuote {
				if ch == quoteChar {
					inQuote = false
				} else {
					current.WriteRune(ch)
				}
			} else {
				inQuote = true
				quoteChar = ch
			}
		case ch == ' ' || ch == '\t':
			if inQuote {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// filterInput filters special input characters
func filterInput(r rune) (rune, bool) {
	switch r {
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}
