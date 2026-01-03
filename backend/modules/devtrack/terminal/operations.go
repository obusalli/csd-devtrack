package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"csd-devtrack/backend/modules/platform/graphql"
	"csd-devtrack/cli/modules/platform/terminal"
)

func init() {
	// Register queries (internal use only, not exposed to frontend)
	graphql.RegisterQuery("terminalSessionInfo", "Get terminal session info for a Claude session", "", GetTerminalSessionInfo)

	// Register mutations
	graphql.RegisterMutation("createTerminalToken", "Create a token for terminal session access", "", CreateTerminalToken)
	graphql.RegisterMutation("revokeTerminalToken", "Revoke a terminal token", "", RevokeTerminalToken)
	graphql.RegisterMutation("killTerminalSession", "Kill a terminal session", "", KillTerminalSession)
}

// RegisterHTTPHandlers registers HTTP handlers for terminal token validation
// This should be called during server initialization
func RegisterHTTPHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/devtrack/api/terminal/validate", HandleValidateToken)
}

// HandleValidateToken handles token validation requests from csd-core
// csd-core calls this endpoint to validate a terminal token and get the session info
func HandleValidateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate token
	claims := ValidateToken(req.Token)

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claims)
}

// CreateTerminalToken creates a token for terminal session access
// The token is opaque - the frontend never sees the session name or prefix
func CreateTerminalToken(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	sessionID, ok := graphql.ParseString(variables, "sessionId")
	if !ok || sessionID == "" {
		graphql.SendError(w, nil, "sessionId is required")
		return
	}

	sessionType, _ := graphql.ParseString(variables, "sessionType")
	if sessionType == "" {
		sessionType = "claude"
	}

	// Get user ID from context (set by auth middleware)
	userID := ""
	if uid, ok := ctx.Value("userID").(string); ok {
		userID = uid
	}

	// Generate token
	token, err := GenerateToken(sessionID, sessionType, userID)
	if err != nil {
		graphql.SendError(w, err, "Failed to create token")
		return
	}

	// Return only the token - no session info exposed
	graphql.SendData(w, "createTerminalToken", map[string]interface{}{
		"token":     token,
		"expiresIn": 300, // 5 minutes in seconds
	})
}

// RevokeTerminalToken revokes a terminal token
func RevokeTerminalToken(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	token, ok := graphql.ParseString(variables, "token")
	if !ok || token == "" {
		graphql.SendError(w, nil, "token is required")
		return
	}

	success := RevokeToken(token)
	graphql.SendData(w, "revokeTerminalToken", map[string]interface{}{
		"success": success,
	})
}

// TerminalSessionInfo contains info about a terminal session
type TerminalSessionInfo struct {
	SessionName string `json:"sessionName"` // Full tmux session name (e.g., "cdt-cc-12345678")
	Prefix      string `json:"prefix"`      // Prefix used (e.g., "cdt-cc-")
	ShortID     string `json:"shortId"`     // Short ID (first 8 chars of session ID)
	Exists      bool   `json:"exists"`      // Whether the tmux session exists
	SessionType string `json:"sessionType"` // Type: "claude", "codex", "database", "shell"
}

// GetTerminalSessionInfo returns info about a terminal session for a Claude session
func GetTerminalSessionInfo(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	sessionID, ok := graphql.ParseString(variables, "sessionId")
	if !ok || sessionID == "" {
		graphql.SendError(w, nil, "sessionId is required")
		return
	}

	sessionType, _ := graphql.ParseString(variables, "sessionType")
	if sessionType == "" {
		sessionType = "claude"
	}

	prefix := prefixForType(sessionType)
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	sessionName := prefix + shortID

	// Check if tmux session exists
	exists := tmuxSessionExists(sessionName)

	info := TerminalSessionInfo{
		SessionName: sessionName,
		Prefix:      prefix,
		ShortID:     shortID,
		Exists:      exists,
		SessionType: sessionType,
	}

	graphql.SendData(w, "terminalSessionInfo", info)
}

// GetTerminalPrefixes returns the valid terminal session prefixes
func GetTerminalPrefixes(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	prefixes := map[string]string{
		"claude":   terminal.PrefixClaude,
		"codex":    terminal.PrefixCodex,
		"database": terminal.PrefixDatabase,
		"shell":    terminal.PrefixShell,
	}
	graphql.SendData(w, "terminalPrefixes", prefixes)
}

// CreateTerminalSession creates or attaches to a terminal session
func CreateTerminalSession(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	sessionID, ok := graphql.ParseString(variables, "sessionId")
	if !ok || sessionID == "" {
		graphql.SendError(w, nil, "sessionId is required")
		return
	}

	sessionType, _ := graphql.ParseString(variables, "sessionType")
	if sessionType == "" {
		sessionType = "claude"
	}

	prefix := prefixForType(sessionType)
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	sessionName := prefix + shortID

	// Check if session already exists
	exists := tmuxSessionExists(sessionName)

	info := TerminalSessionInfo{
		SessionName: sessionName,
		Prefix:      prefix,
		ShortID:     shortID,
		Exists:      exists,
		SessionType: sessionType,
	}

	graphql.SendData(w, "createTerminalSession", info)
}

// KillTerminalSession kills a terminal session
func KillTerminalSession(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	sessionID, ok := graphql.ParseString(variables, "sessionId")
	if !ok || sessionID == "" {
		graphql.SendError(w, nil, "sessionId is required")
		return
	}

	sessionType, _ := graphql.ParseString(variables, "sessionType")
	if sessionType == "" {
		sessionType = "claude"
	}

	prefix := prefixForType(sessionType)
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	sessionName := prefix + shortID

	// Kill tmux session
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	err := cmd.Run()

	if err != nil {
		graphql.SendError(w, err, "Failed to kill session")
		return
	}

	graphql.SendData(w, "killTerminalSession", map[string]interface{}{
		"success":     true,
		"sessionName": sessionName,
	})
}

// prefixForType returns the terminal prefix for a session type
func prefixForType(sessionType string) string {
	switch sessionType {
	case "claude":
		return terminal.PrefixClaude
	case "codex":
		return terminal.PrefixCodex
	case "database":
		return terminal.PrefixDatabase
	case "shell":
		return terminal.PrefixShell
	default:
		return terminal.PrefixClaude
	}
}

// tmuxSessionExists checks if a tmux session exists
func tmuxSessionExists(sessionName string) bool {
	// SECURITY: Validate session name to prevent command injection
	if !isValidSessionName(sessionName) {
		return false
	}
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

// isValidSessionName validates that a session name is safe
// SECURITY: Prevents command injection by ensuring the name only contains safe characters
func isValidSessionName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	// Only allow alphanumeric, hyphens, underscores
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	// Must start with a valid prefix
	for _, p := range terminal.AllPrefixes() {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// ListTerminalSessions returns all terminal sessions for a given type
func ListTerminalSessions(sessionType string) ([]string, error) {
	prefix := prefixForType(sessionType)

	cmd := exec.Command("tmux", "ls", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// No tmux server or no sessions is not an error
		return []string{}, nil
	}

	var sessions []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			sessions = append(sessions, line)
		}
	}

	return sessions, nil
}

// CleanupOrphanSessions kills all sessions with cdt-* prefixes
func CleanupOrphanSessions() (int, error) {
	cmd := exec.Command("tmux", "ls", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return 0, nil // No tmux server or no sessions
	}

	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "cdt-") {
			killCmd := exec.Command("tmux", "kill-session", "-t", line)
			if err := killCmd.Run(); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// GetSessionIDFromName extracts the session ID from a terminal session name
func GetSessionIDFromName(sessionName string) (string, string) {
	for _, prefix := range terminal.AllPrefixes() {
		if strings.HasPrefix(sessionName, prefix) {
			return strings.TrimPrefix(sessionName, prefix), prefix
		}
	}
	return "", ""
}

// BuildSessionName builds a full session name from type and ID
func BuildSessionName(sessionType, sessionID string) string {
	prefix := prefixForType(sessionType)
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("%s%s", prefix, shortID)
}
