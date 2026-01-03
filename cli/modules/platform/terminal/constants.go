// Package terminal provides shared terminal-related constants and utilities
// for both CLI and backend modules.
package terminal

// Tmux session prefixes (cdt = csd-devtrack)
// These prefixes are used to identify different types of terminal sessions.
const (
	PrefixClaude   = "cdt-cc-" // Claude Code sessions
	PrefixCodex    = "cdt-cx-" // Codex sessions
	PrefixDatabase = "cdt-db-" // Database clients (psql, mysql, sqlite3)
	PrefixShell    = "cdt-sh-" // Terminal/Shell sessions
)

// AllPrefixes returns all valid session prefixes
func AllPrefixes() []string {
	return []string{
		PrefixClaude,
		PrefixCodex,
		PrefixDatabase,
		PrefixShell,
	}
}

// IsValidPrefix checks if a prefix is a valid csd-devtrack session prefix
func IsValidPrefix(prefix string) bool {
	for _, p := range AllPrefixes() {
		if p == prefix {
			return true
		}
	}
	return false
}
