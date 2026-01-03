package capabilities

import "time"

// Capability represents an external tool capability
type Capability string

// Available capabilities
const (
	CapTmux   Capability = "tmux"   // Terminal multiplexer (for Claude/Database views)
	CapClaude Capability = "claude" // Claude CLI
	CapCodex  Capability = "codex"  // OpenAI Codex CLI
	CapShell  Capability = "shell"  // Bash/sh shell
	CapSudo   Capability = "sudo"   // Sudo for root access
	CapPsql   Capability = "psql"   // PostgreSQL client
	CapMysql  Capability = "mysql"  // MySQL client
	CapSqlite Capability = "sqlite" // SQLite client
	CapGit    Capability = "git"    // Git version control
	CapGo     Capability = "go"     // Go compiler
	CapNode   Capability = "node"   // Node.js runtime
	CapNpm    Capability = "npm"    // Node package manager
)

// AllCapabilities lists all capabilities to detect
var AllCapabilities = []Capability{
	CapTmux,
	CapClaude,
	CapCodex,
	CapShell,
	CapSudo,
	CapPsql,
	CapMysql,
	CapSqlite,
	CapGit,
	CapGo,
	CapNode,
	CapNpm,
}

// CapabilityInfo holds information about a detected capability
type CapabilityInfo struct {
	Name      Capability // Capability name
	Available bool       // Whether the tool is available
	Path      string     // Full path to the executable
	Version   string     // Detected version (if available)
	CheckedAt time.Time  // When this capability was last checked
}

// capabilityConfig defines how to detect each capability
type capabilityConfig struct {
	name       Capability
	binaries   []string // Possible binary names to search
	versionArg string   // Argument to get version (e.g., "--version")
	verify     bool     // Whether to verify by running the binary
}

// capabilityConfigs defines detection configuration for each capability
var capabilityConfigs = map[Capability]capabilityConfig{
	CapTmux: {
		name:       CapTmux,
		binaries:   []string{"tmux"},
		versionArg: "-V",
		verify:     true,
	},
	CapClaude: {
		name:       CapClaude,
		binaries:   []string{"claude"},
		versionArg: "--version",
		verify:     true,
	},
	CapCodex: {
		name:       CapCodex,
		binaries:   []string{"codex"},
		versionArg: "--version",
		verify:     true,
	},
	CapShell: {
		name: CapShell,
		// Order of preference: Unix shells first, then Windows shells
		// Detection will find whichever is available on the current system
		binaries:   []string{"bash", "zsh", "sh", "pwsh", "powershell", "cmd"},
		versionArg: "",
		verify:     false, // Don't verify - shell is fundamental
	},
	CapSudo: {
		name:       CapSudo,
		binaries:   []string{"sudo"},
		versionArg: "",
		verify:     false, // Don't verify - just check existence
	},
	CapPsql: {
		name:       CapPsql,
		binaries:   []string{"psql"},
		versionArg: "--version",
		verify:     true,
	},
	CapMysql: {
		name:       CapMysql,
		binaries:   []string{"mysql"},
		versionArg: "--version",
		verify:     true,
	},
	CapSqlite: {
		name:       CapSqlite,
		binaries:   []string{"sqlite3"},
		versionArg: "--version",
		verify:     true,
	},
	CapGit: {
		name:       CapGit,
		binaries:   []string{"git"},
		versionArg: "--version",
		verify:     true,
	},
	CapGo: {
		name:       CapGo,
		binaries:   []string{"go"},
		versionArg: "version",
		verify:     true,
	},
	CapNode: {
		name:       CapNode,
		binaries:   []string{"node"},
		versionArg: "--version",
		verify:     true,
	},
	CapNpm: {
		name:       CapNpm,
		binaries:   []string{"npm"},
		versionArg: "--version",
		verify:     true,
	},
}
