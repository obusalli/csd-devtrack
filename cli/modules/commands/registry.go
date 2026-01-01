package commands

import (
	"fmt"
	"sort"
	"strings"
)

// CommandHandler is the function signature for command handlers
type CommandHandler func(args []string) error

// SubCommand represents a sub-command
type SubCommand struct {
	Name        string
	Description string
	Handler     CommandHandler
}

// Command represents a CLI command
type Command struct {
	Name        string
	Aliases     []string
	Category    string
	Description string
	Usage       string
	Examples    []string
	Handler     CommandHandler
	SubCommands []SubCommand
	Order       int
}

// Registry holds all registered commands
type Registry struct {
	commands map[string]*Command
	aliases  map[string]string
}

// Global registry instance
var globalRegistry *Registry

// InitRegistry initializes the global command registry
func InitRegistry() {
	globalRegistry = &Registry{
		commands: make(map[string]*Command),
		aliases:  make(map[string]string),
	}

	// Register all commands
	registerCoreCommands()
	registerBuildCommands()
	registerRunCommands()
	registerGitCommands()
	registerConfigCommands()
	registerUICommands()
}

// RegisterCommand registers a command
func RegisterCommand(cmd *Command) {
	if globalRegistry == nil {
		InitRegistry()
	}

	globalRegistry.commands[cmd.Name] = cmd

	// Register aliases
	for _, alias := range cmd.Aliases {
		globalRegistry.aliases[alias] = cmd.Name
	}
}

// GetCommand returns a command by name or alias
func GetCommand(name string) *Command {
	if globalRegistry == nil {
		return nil
	}

	// Check direct name
	if cmd, ok := globalRegistry.commands[name]; ok {
		return cmd
	}

	// Check aliases
	if cmdName, ok := globalRegistry.aliases[name]; ok {
		return globalRegistry.commands[cmdName]
	}

	return nil
}

// GetAllCommands returns all registered commands
func GetAllCommands() []*Command {
	if globalRegistry == nil {
		return nil
	}

	commands := make([]*Command, 0, len(globalRegistry.commands))
	for _, cmd := range globalRegistry.commands {
		commands = append(commands, cmd)
	}

	// Sort by order then name
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Order != commands[j].Order {
			return commands[i].Order < commands[j].Order
		}
		return commands[i].Name < commands[j].Name
	})

	return commands
}

// GetCommandsByCategory returns commands grouped by category
func GetCommandsByCategory() map[string][]*Command {
	categories := make(map[string][]*Command)

	for _, cmd := range GetAllCommands() {
		categories[cmd.Category] = append(categories[cmd.Category], cmd)
	}

	return categories
}

// PrintCommands prints all commands
func PrintCommands() {
	categories := GetCommandsByCategory()

	// Define category order
	categoryOrder := []string{
		"Project Management",
		"Build",
		"Process Management",
		"Git",
		"Configuration",
		"Interface",
	}

	for _, category := range categoryOrder {
		cmds, ok := categories[category]
		if !ok || len(cmds) == 0 {
			continue
		}

		fmt.Printf("  %s:\n", category)
		for _, cmd := range cmds {
			aliases := ""
			if len(cmd.Aliases) > 0 {
				aliases = fmt.Sprintf(" (%s)", strings.Join(cmd.Aliases, ", "))
			}
			fmt.Printf("    %-20s %s%s\n", cmd.Name, cmd.Description, aliases)
		}
		fmt.Println()
	}
}

// PrintCommandHelp prints help for a specific command
func PrintCommandHelp(name string) {
	cmd := GetCommand(name)
	if cmd == nil {
		fmt.Printf("Unknown command: %s\n", name)
		return
	}

	fmt.Printf("Command: %s\n", cmd.Name)
	if len(cmd.Aliases) > 0 {
		fmt.Printf("Aliases: %s\n", strings.Join(cmd.Aliases, ", "))
	}
	fmt.Printf("Category: %s\n", cmd.Category)
	fmt.Println()
	fmt.Printf("Description:\n  %s\n", cmd.Description)
	fmt.Println()

	if cmd.Usage != "" {
		fmt.Printf("Usage:\n  %s\n", cmd.Usage)
		fmt.Println()
	}

	if len(cmd.SubCommands) > 0 {
		fmt.Println("Sub-commands:")
		for _, sub := range cmd.SubCommands {
			fmt.Printf("  %-15s %s\n", sub.Name, sub.Description)
		}
		fmt.Println()
	}

	if len(cmd.Examples) > 0 {
		fmt.Println("Examples:")
		for _, example := range cmd.Examples {
			fmt.Printf("  %s\n", example)
		}
	}
}

// GetCommandNames returns all command names (for completion)
func GetCommandNames() []string {
	if globalRegistry == nil {
		return nil
	}

	names := make([]string, 0, len(globalRegistry.commands)+len(globalRegistry.aliases))
	for name := range globalRegistry.commands {
		names = append(names, name)
	}
	for alias := range globalRegistry.aliases {
		names = append(names, alias)
	}

	sort.Strings(names)
	return names
}

// Helper function to find sub-command
func findSubCommand(cmd *Command, name string) *SubCommand {
	for i := range cmd.SubCommands {
		if cmd.SubCommands[i].Name == name {
			return &cmd.SubCommands[i]
		}
	}
	return nil
}
