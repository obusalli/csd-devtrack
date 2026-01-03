package claude

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// generateID generates a unique ID for messages
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// claudeProjectsDir returns the path to Claude CLI projects directory
func claudeProjectsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// workDirToClaudeProject converts a work directory to Claude project folder name
// e.g., /data/devel/infra/csd-devtrack -> -data-devel-infra-csd-devtrack
func workDirToClaudeProject(workDir string) string {
	return strings.ReplaceAll(workDir, "/", "-")
}

// isValidUUID checks if a string is a valid UUID
func isValidUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(s)
}

// formatToolUseForDisplay formats a tool use block for display (Claude CLI style)
// Uses dynamic extraction of common parameters to work with any tool
func formatToolUseForDisplay(toolName string, input map[string]interface{}) string {
	var sb strings.Builder

	// Map tool names to display names (like Claude CLI does)
	displayName := toolName
	switch toolName {
	case "Edit":
		displayName = "Update"
	case "Grep":
		displayName = "Search"
	}

	// Extract common parameters dynamically
	identifier := extractToolIdentifier(toolName, input)

	// Format header: ● ToolName(identifier)
	if identifier != "" {
		sb.WriteString(fmt.Sprintf("● %s(%s)\n", displayName, identifier))
	} else {
		sb.WriteString(fmt.Sprintf("● %s\n", displayName))
	}

	// Add details based on tool type
	sb.WriteString(formatToolDetails(toolName, input))

	return sb.String()
}

// extractToolIdentifier extracts a meaningful identifier from tool input
func extractToolIdentifier(toolName string, input map[string]interface{}) string {
	// Priority order for identifier extraction
	identifierKeys := []string{
		"file_path", "path", "filePath", "notebook_path", // File operations
		"pattern",                          // Search operations
		"command", "description",           // Bash
		"query",                            // Web search
		"url",                              // Web fetch
		"operation",                        // LSP
		"skill",                            // Skill
		"task_id", "shell_id", "session_id", // IDs
	}

	for _, key := range identifierKeys {
		if val, ok := input[key].(string); ok && val != "" {
			// For file paths, show relative path (remove common prefixes)
			if strings.Contains(key, "path") || key == "path" {
				val = shortenPath(val)
			}
			// For description in Bash, use it directly
			if key == "description" {
				return val
			}
			// Truncate long values
			if len(val) > 60 {
				val = val[:60] + "..."
			}
			return val
		}
	}

	return ""
}

// shortenPath removes common prefixes to show a relative-like path
func shortenPath(path string) string {
	// Remove absolute path prefix to show relative path
	// Common patterns: /home/user/..., /data/..., etc.

	// Try to find a reasonable starting point
	parts := strings.Split(path, "/")

	// If path has many components, try to find a good starting point
	// Look for common project markers
	markers := []string{"src", "lib", "pkg", "cmd", "modules", "internal", "api", "cli"}
	for i, part := range parts {
		for _, marker := range markers {
			if part == marker && i > 0 {
				// Return from one level before the marker
				return strings.Join(parts[i-1:], "/")
			}
		}
	}

	// If no marker found, just show last 3 components or full path if short
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}

	return path
}

// formatToolDetails generates the details line(s) for a tool
func formatToolDetails(toolName string, input map[string]interface{}) string {
	var sb strings.Builder

	switch toolName {
	case "Read":
		if limit, ok := input["limit"].(float64); ok {
			sb.WriteString(fmt.Sprintf("  ⎿  Read %d lines\n", int(limit)))
		} else {
			sb.WriteString("  ⎿  Read file\n")
		}

	case "Edit":
		filePath, _ := input["file_path"].(string)
		oldStr, _ := input["old_string"].(string)
		newStr, _ := input["new_string"].(string)

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

		// Summary
		if addedLines > 0 && removedLines > 0 {
			sb.WriteString(fmt.Sprintf("  ⎿  Added %d lines, removed %d lines\n", addedLines, removedLines))
		} else if addedLines > 0 {
			sb.WriteString(fmt.Sprintf("  ⎿  Added %d lines\n", addedLines))
		} else if removedLines > 0 {
			sb.WriteString(fmt.Sprintf("  ⎿  Removed %d lines\n", removedLines))
		} else {
			sb.WriteString("  ⎿  Modified\n")
		}

		// Format diff with markers and actual line numbers from file
		sb.WriteString(formatDiffWithMarkersAndPath(oldStr, newStr, 2, filePath))

	case "Write":
		contentStr, _ := input["content"].(string)
		lines := strings.Count(contentStr, "\n") + 1
		sb.WriteString(fmt.Sprintf("  ⎿  Wrote %d lines\n", lines))

	case "Bash":
		sb.WriteString("  ⎿  Executed\n")

	case "Glob":
		sb.WriteString("  ⎿  File search\n")

	case "Grep":
		// Show search details
		path, _ := input["path"].(string)
		outputMode, _ := input["output_mode"].(string)
		if path != "" {
			sb.WriteString(fmt.Sprintf("  ⎿  in %s\n", shortenPath(path)))
		} else if outputMode == "content" {
			sb.WriteString("  ⎿  Content search\n")
		} else {
			sb.WriteString("  ⎿  File search\n")
		}

	case "TodoWrite":
		formatTodoWriteDetails(&sb, input)

	case "Task":
		if subType, ok := input["subagent_type"].(string); ok && subType != "" {
			sb.WriteString(fmt.Sprintf("  ⎿  Subagent: %s\n", subType))
		}

	case "WebSearch":
		sb.WriteString("  ⎿  Searching web\n")

	case "WebFetch":
		sb.WriteString("  ⎿  Fetching URL\n")

	case "LSP":
		if op, ok := input["operation"].(string); ok {
			sb.WriteString(fmt.Sprintf("  ⎿  %s\n", op))
		}

	case "AskUserQuestion":
		if questions, ok := input["questions"].([]interface{}); ok && len(questions) > 0 {
			if q, ok := questions[0].(map[string]interface{}); ok {
				if question, ok := q["question"].(string); ok {
					if len(question) > 60 {
						question = question[:60] + "..."
					}
					sb.WriteString(fmt.Sprintf("  ⎿  %s\n", question))
				}
			}
		}

	case "NotebookEdit":
		editMode, _ := input["edit_mode"].(string)
		if editMode == "" {
			editMode = "replace"
		}
		sb.WriteString(fmt.Sprintf("  ⎿  %s cell\n", editMode))

	default:
		// Dynamic detail extraction for unknown tools
		sb.WriteString(formatDynamicDetails(input))
	}

	return sb.String()
}

// formatTodoWriteDetails formats the TodoWrite tool details
func formatTodoWriteDetails(sb *strings.Builder, input map[string]interface{}) {
	todos, _ := input["todos"].([]interface{})
	if len(todos) == 0 {
		sb.WriteString("  ⎿  Updated todos\n")
		return
	}

	// Count by status
	pending, inProgress, completed := 0, 0, 0
	for _, t := range todos {
		if todo, ok := t.(map[string]interface{}); ok {
			status, _ := todo["status"].(string)
			switch status {
			case "pending":
				pending++
			case "in_progress":
				inProgress++
			case "completed":
				completed++
			}
		}
	}
	sb.WriteString(fmt.Sprintf("  ⎿  %d items (✓%d ●%d ○%d)\n", len(todos), completed, inProgress, pending))

	// Show items
	for _, t := range todos {
		if todo, ok := t.(map[string]interface{}); ok {
			content, _ := todo["content"].(string)
			status, _ := todo["status"].(string)
			icon := "○"
			switch status {
			case "completed":
				icon = "✓"
			case "in_progress":
				icon = "●"
			}
			if len(content) > 60 {
				content = content[:60] + "..."
			}
			sb.WriteString(fmt.Sprintf("     %s %s\n", icon, content))
		}
	}
}

// formatDynamicDetails extracts details from unknown tool inputs dynamically
func formatDynamicDetails(input map[string]interface{}) string {
	if len(input) == 0 {
		return ""
	}

	// Look for meaningful values to display
	var details []string

	// Check for common result indicators
	for _, key := range []string{"content", "text", "message", "result", "output"} {
		if val, ok := input[key].(string); ok && val != "" {
			preview := val
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			details = append(details, preview)
			break
		}
	}

	// Check for boolean flags
	for key, val := range input {
		if b, ok := val.(bool); ok && b {
			details = append(details, key)
		}
	}

	// Check for numeric values
	for key, val := range input {
		if n, ok := val.(float64); ok && n > 0 {
			details = append(details, fmt.Sprintf("%s: %v", key, n))
			if len(details) >= 2 {
				break
			}
		}
	}

	if len(details) > 0 {
		return fmt.Sprintf("  ⎿  %s\n", strings.Join(details, ", "))
	}

	return "  ⎿  Executed\n"
}

// findLineNumber searches for oldStr in a file and returns the starting line number
// Returns 0 if not found or file can't be read
func findLineNumber(filePath, oldStr string) int {
	if filePath == "" || oldStr == "" {
		return 0
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}

	fileContent := string(content)
	idx := strings.Index(fileContent, oldStr)
	if idx == -1 {
		// Try with normalized line endings
		normalizedContent := strings.ReplaceAll(fileContent, "\r\n", "\n")
		normalizedOld := strings.ReplaceAll(oldStr, "\r\n", "\n")
		idx = strings.Index(normalizedContent, normalizedOld)
		if idx == -1 {
			return 0
		}
		fileContent = normalizedContent
	}

	// Count newlines before the match to get line number
	lineNum := 1
	for i := 0; i < idx; i++ {
		if fileContent[i] == '\n' {
			lineNum++
		}
	}

	return lineNum
}

// formatDiffWithMarkers creates a diff with {{-...}} and {{+...}} markers
// filePath is optional - if provided, will try to find actual line numbers
func formatDiffWithMarkers(oldStr, newStr string, contextLines int) string {
	return formatDiffWithMarkersAndPath(oldStr, newStr, contextLines, "")
}

// formatDiffWithMarkersAndPath creates a diff with actual line numbers from the file
func formatDiffWithMarkersAndPath(oldStr, newStr string, contextLines int, filePath string) string {
	var sb strings.Builder

	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// Try to find actual line number in file
	baseLineNum := findLineNumber(filePath, oldStr)
	if baseLineNum == 0 {
		baseLineNum = 1 // Fallback to 1 if not found
	}

	// Find common prefix
	commonPrefix := 0
	minLen := len(oldLines)
	if len(newLines) < minLen {
		minLen = len(newLines)
	}
	for i := 0; i < minLen && oldLines[i] == newLines[i]; i++ {
		commonPrefix++
	}

	// Find common suffix
	commonSuffix := 0
	for i := 0; i < minLen-commonPrefix && oldLines[len(oldLines)-1-i] == newLines[len(newLines)-1-i]; i++ {
		commonSuffix++
	}

	// Context before
	startContext := commonPrefix - contextLines
	if startContext < 0 {
		startContext = 0
	}

	lineNum := baseLineNum + startContext
	for i := startContext; i < commonPrefix && i < len(oldLines); i++ {
		sb.WriteString(fmt.Sprintf("      %3d   %s\n", lineNum, oldLines[i]))
		lineNum++
	}

	// Removed lines
	removedEnd := len(oldLines) - commonSuffix
	for i := commonPrefix; i < removedEnd; i++ {
		sb.WriteString(fmt.Sprintf("{{-   %3d - %s}}\n", lineNum, oldLines[i]))
		lineNum++
	}

	// Added lines (use same line number as removed for replacement)
	addedLineNum := baseLineNum + commonPrefix
	addedEnd := len(newLines) - commonSuffix
	for i := commonPrefix; i < addedEnd; i++ {
		sb.WriteString(fmt.Sprintf("{{+   %3d + %s}}\n", addedLineNum, newLines[i]))
		lineNum++
	}

	// Context after (calculate line number based on new content position)
	if commonSuffix > 0 && commonSuffix <= contextLines {
		// After the changes, line numbers continue from where the new content ends
		contextStartIdx := len(newLines) - commonSuffix
		afterLineNum := baseLineNum + contextStartIdx
		for i := contextStartIdx; i < len(newLines); i++ {
			sb.WriteString(fmt.Sprintf("      %3d   %s\n", afterLineNum, newLines[i]))
			afterLineNum++
		}
	}

	return sb.String()
}

// persistentProcess holds a running Claude process for a session
type persistentProcess struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	stderr   io.ReadCloser
	cancel   context.CancelFunc
	outputCh chan ClaudeOutput
	mu       sync.Mutex
}

// Service manages Claude Code integration
type Service struct {
	mu              sync.RWMutex
	sessions        map[string]*Session
	customNames     map[string]string // sessionID -> custom name
	dataDir         string
	claudePath      string
	isInstalled     bool
	activeProcs     map[string]*exec.Cmd
	persistentProcs map[string]*persistentProcess // Persistent processes per session
	outputChans     map[string]chan ClaudeOutput
}

// NewService creates a new Claude service
func NewService(dataDir string) *Service {
	s := &Service{
		sessions:        make(map[string]*Session),
		customNames:     make(map[string]string),
		dataDir:         dataDir,
		activeProcs:     make(map[string]*exec.Cmd),
		persistentProcs: make(map[string]*persistentProcess),
		outputChans:     make(map[string]chan ClaudeOutput),
	}
	s.detectClaude()
	s.loadCustomNames()
	s.loadSessions()
	return s
}

// IsInstalled returns true if Claude Code CLI is available
func (s *Service) IsInstalled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isInstalled
}

// GetClaudePath returns the path to claude binary
func (s *Service) GetClaudePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claudePath
}

// detectClaude checks if Claude Code CLI is installed
func (s *Service) detectClaude() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try common locations
	paths := []string{
		"claude", // In PATH
	}

	// Add user-specific paths
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".local", "bin", "claude"),
			filepath.Join(home, ".npm-global", "bin", "claude"),
			"/usr/local/bin/claude",
		)
	}

	for _, path := range paths {
		if p, err := exec.LookPath(path); err == nil {
			// Verify it's actually claude code by checking version
			cmd := exec.Command(p, "--version")
			output, err := cmd.Output()
			if err == nil && strings.Contains(strings.ToLower(string(output)), "claude") {
				s.claudePath = p
				s.isInstalled = true
				return
			}
		}
	}

	s.isInstalled = false
}

// RefreshInstallStatus rechecks if Claude is installed
func (s *Service) RefreshInstallStatus() bool {
	s.detectClaude()
	return s.IsInstalled()
}

// customNamesFile returns the path to the custom session names file
func (s *Service) customNamesFile() string {
	return filepath.Join(s.dataDir, "claude-session-names.json")
}

// loadCustomNames loads custom session names from local storage
func (s *Service) loadCustomNames() {
	data, err := os.ReadFile(s.customNamesFile())
	if err != nil {
		return // File doesn't exist yet
	}

	if err := json.Unmarshal(data, &s.customNames); err != nil {
		s.customNames = make(map[string]string)
	}
}

// saveCustomNames saves custom session names to local storage
func (s *Service) saveCustomNames() error {
	data, err := json.MarshalIndent(s.customNames, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.customNamesFile(), data, 0644)
}

// loadSessions loads sessions from Claude CLI's ~/.claude/projects/ directory
func (s *Service) loadSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	projectsDir := claudeProjectsDir()

	// Read all project directories
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return // No Claude projects yet
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		s.loadProjectSessions(projectPath, entry.Name())
	}
}

// loadProjectSessions loads sessions from a specific Claude project directory
func (s *Service) loadProjectSessions(projectPath, projectName string) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		// Skip agent sessions (they have different format)
		if strings.HasPrefix(entry.Name(), "agent-") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")

		// Only load UUID sessions (not agent IDs)
		if !isValidUUID(sessionID) {
			continue
		}

		sessionFile := filepath.Join(projectPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		var session *Session

		// Handle empty sessions (newly created, not yet used)
		if info.Size() == 0 {
			// Create minimal session for empty file
			session = &Session{
				ID:            sessionID,
				Name:          sessionID[:8], // Short ID as default name
				ProjectName:   projectName,
				State:         SessionIdle,
				Messages:      make([]Message, 0),
				CreatedAt:     info.ModTime(),
				LastActiveAt:  info.ModTime(),
				IsRealSession: true,
				SessionFile:   sessionFile,
			}
		} else {
			// Load session metadata from JSONL (workDir will be extracted from cwd field)
			session = s.parseSessionFile(sessionID, sessionFile)
		}

		if session != nil {
			// Apply custom name if one exists
			if customName, ok := s.customNames[sessionID]; ok {
				session.CustomName = customName
			}
			s.sessions[sessionID] = session
		}
	}
}

// parseSessionFile reads a Claude CLI JSONL session file and extracts metadata
func (s *Service) parseSessionFile(sessionID, filePath string) *Session {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	session := &Session{
		ID:            sessionID,
		State:         SessionIdle,
		Messages:      make([]Message, 0),
		IsRealSession: true,
		SessionFile:   filePath,
	}

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var firstTimestamp, lastTimestamp time.Time
	var sessionName string
	var workDir string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Get working directory from cwd field (most reliable source)
		if cwd, ok := entry["cwd"].(string); ok && workDir == "" {
			workDir = cwd
		}

		// Get session name from slug
		if slug, ok := entry["slug"].(string); ok && sessionName == "" {
			sessionName = slug
		}

		// Get timestamp
		if ts, ok := entry["timestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				if firstTimestamp.IsZero() {
					firstTimestamp = t
				}
				lastTimestamp = t
			}
		}

		// Count messages (user and assistant types)
		if entryType, ok := entry["type"].(string); ok {
			if entryType == "user" || entryType == "assistant" {
				// Extract message content for display
				if msg, ok := entry["message"].(map[string]interface{}); ok {
					role := ""
					if r, ok := msg["role"].(string); ok {
						role = r
					}

					var contentBuilder strings.Builder
					// Content can be a string (user messages) or array (assistant messages)
					if contentStr, ok := msg["content"].(string); ok {
						// User messages have content as a simple string
						contentBuilder.WriteString(contentStr)
					} else if c, ok := msg["content"].([]interface{}); ok {
						// Assistant messages have content as array of blocks
						for _, item := range c {
							if block, ok := item.(map[string]interface{}); ok {
								blockType, _ := block["type"].(string)

								switch blockType {
								case "text":
									if text, ok := block["text"].(string); ok {
										contentBuilder.WriteString(text)
									}

								case "tool_use":
									// Format tool usage like Claude CLI
									toolName, _ := block["name"].(string)
									input, _ := block["input"].(map[string]interface{})
									contentBuilder.WriteString("\n")
									contentBuilder.WriteString(formatToolUseForDisplay(toolName, input))
								}
							}
						}
					}

					content := contentBuilder.String()
					if role != "" && content != "" {
						session.Messages = append(session.Messages, Message{
							ID:        fmt.Sprintf("%s-%d", sessionID, len(session.Messages)),
							Role:      role,
							Content:   content,
							Timestamp: lastTimestamp,
						})
					}
				}
			}
		}
	}

	// Set working directory and extract project info
	// If workDir not found in JSONL, derive from file path
	if workDir == "" {
		// filePath is like ~/.claude/projects/-data-devel-infra-csd-devtrack/session.jsonl
		// Extract project dir name and convert back to path
		dir := filepath.Dir(filePath)
		projectDirName := filepath.Base(dir)
		if strings.HasPrefix(projectDirName, "-") {
			// Convert -data-devel-infra-csd-devtrack to /data/devel/infra/csd-devtrack
			workDir = strings.ReplaceAll(projectDirName, "-", "/")
		}
	}
	session.WorkDir = workDir
	if workDir != "" {
		// Extract project name from last path segment
		parts := strings.Split(workDir, "/")
		if len(parts) > 0 {
			session.ProjectName = parts[len(parts)-1]
			session.ProjectID = session.ProjectName
		}
	}

	// Set session name
	if sessionName != "" {
		session.Name = sessionName
	} else if len(sessionID) >= 8 {
		session.Name = sessionID[:8] // Use first 8 chars of UUID
	} else {
		session.Name = sessionID
	}

	// Set timestamps
	if !firstTimestamp.IsZero() {
		session.CreatedAt = firstTimestamp
	} else {
		// Use file mod time as fallback
		if info, err := os.Stat(filePath); err == nil {
			session.CreatedAt = info.ModTime()
		}
	}
	if !lastTimestamp.IsZero() {
		session.LastActiveAt = lastTimestamp
	} else {
		session.LastActiveAt = session.CreatedAt
	}

	return session
}

// RefreshSessions reloads sessions from Claude CLI directory
func (s *Service) RefreshSessions() {
	s.mu.Lock()
	// Clear existing sessions
	s.sessions = make(map[string]*Session)
	s.mu.Unlock()

	s.loadSessions()
}

// saveSessions is now a no-op since Claude CLI manages its own sessions
func (s *Service) saveSessions() error {
	// Sessions are managed by Claude CLI, nothing to save
	return nil
}

// CreateSession creates a new Claude session for a project
// Uses a real UUID that will be used with Claude CLI --session-id
func (s *Service) CreateSession(projectID, projectName, workDir, name string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a real UUID for Claude CLI compatibility
	sessionID := GenerateSessionID()

	// Build the expected session file path
	claudeProjectName := workDirToClaudeProject(workDir)
	sessionFile := filepath.Join(claudeProjectsDir(), claudeProjectName, sessionID+".jsonl")

	// Create the directory if it doesn't exist
	sessionDir := filepath.Dir(sessionFile)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create an empty JSONL file so it persists and is picked up by ScanSessions
	f, err := os.Create(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create session file: %w", err)
	}
	f.Close()

	session := &Session{
		ID:            sessionID,
		Name:          name,
		ProjectID:     projectID,
		ProjectName:   projectName,
		WorkDir:       workDir,
		State:         SessionIdle,
		Messages:      make([]Message, 0),
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
		IsRealSession: true,
		SessionFile:   sessionFile,
	}

	s.sessions[session.ID] = session

	// Save custom name if provided
	if name != "" {
		s.customNames[sessionID] = name
		go s.saveCustomNames()
	}

	return session, nil
}

// GetSession returns a session by ID
func (s *Service) GetSession(sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return sess, nil
}

// StartPersistentProcess starts a background Claude process for faster responses
// Uses stream-json bidirectional mode for persistent connection
func (s *Service) StartPersistentProcess(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already running
	if _, exists := s.persistentProcs[sessionID]; exists {
		return nil // Already running
	}

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !s.isInstalled {
		return fmt.Errorf("claude is not installed")
	}

	// Create context for this process
	ctx, cancel := context.WithCancel(context.Background())

	// Start claude in stream-json bidirectional mode
	// This keeps the process alive and allows sending multiple messages
	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
	}

	// Use real Claude CLI session ID for persistence
	// If session file exists, use --resume to continue; otherwise --session-id to create
	if sess.IsRealSession && sess.SessionFile != "" {
		if _, err := os.Stat(sess.SessionFile); err == nil {
			// Session file exists, resume it
			args = append(args, "--resume", sessionID)
		} else {
			// New session, use --session-id to create with specific UUID
			args = append(args, "--session-id", sessionID)
		}
	} else if isValidUUID(sessionID) {
		// Use session ID for new sessions
		args = append(args, "--session-id", sessionID)
	}

	cmd := exec.CommandContext(ctx, s.claudePath, args...)
	cmd.Dir = sess.WorkDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start claude: %w", err)
	}

	proc := &persistentProcess{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdout),
		stderr:   stderr,
		cancel:   cancel,
		outputCh: make(chan ClaudeOutput, 100),
	}

	s.persistentProcs[sessionID] = proc

	// Start goroutine to read output continuously
	go s.readPersistentOutput(sessionID, proc)

	return nil
}

// StopPersistentProcess stops the background Claude process for a session
func (s *Service) StopPersistentProcess(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if proc, exists := s.persistentProcs[sessionID]; exists {
		proc.cancel()
		proc.stdin.Close()
		proc.cmd.Wait()
		close(proc.outputCh)
		delete(s.persistentProcs, sessionID)
	}
}

// IsPersistentProcessRunning checks if a persistent process is running for a session
func (s *Service) IsPersistentProcessRunning(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proc, exists := s.persistentProcs[sessionID]
	if !exists {
		return false
	}
	// Also check if the process is still alive
	if proc.cmd.ProcessState != nil && proc.cmd.ProcessState.Exited() {
		return false
	}
	return true
}

// GetPersistentProcessSessions returns IDs of sessions with active persistent processes
func (s *Service) GetPersistentProcessSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []string
	for id, proc := range s.persistentProcs {
		// Check if still alive
		if proc.cmd.ProcessState == nil || !proc.cmd.ProcessState.Exited() {
			sessions = append(sessions, id)
		}
	}
	return sessions
}

// readPersistentOutput reads output from persistent process continuously
// Parses JSON stream format and extracts text content
// Uses byte-by-byte reading for maximum responsiveness
func (s *Service) readPersistentOutput(sessionID string, proc *persistentProcess) {
	var lineBuffer strings.Builder

	// Read byte by byte for maximum responsiveness
	buf := make([]byte, 1)
	for {
		n, err := proc.stdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				proc.outputCh <- ClaudeOutput{Type: "error", Content: err.Error()}
			}
			break
		}
		if n == 0 {
			continue
		}

		b := buf[0]
		if b == '\n' {
			// Complete line received, process it
			line := strings.TrimSpace(lineBuffer.String())
			lineBuffer.Reset()

			if line == "" {
				continue
			}

			s.processStreamLine(line, proc.outputCh)
		} else {
			lineBuffer.WriteByte(b)
		}
	}
}

// processStreamLine processes a single JSON line from the stream
func (s *Service) processStreamLine(line string, outputCh chan<- ClaudeOutput) {
	// Parse JSON line
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		// Not JSON, send as raw text immediately
		outputCh <- ClaudeOutput{Type: "text", Content: line}
		return
	}

	msgType, _ := msg["type"].(string)

	switch msgType {
	case "system":
		// System init message - extract session_id if present
		if sid, ok := msg["session_id"].(string); ok {
			outputCh <- ClaudeOutput{Type: "system", Content: "Session started: " + sid}
		}

	case "assistant":
		// Extract content from assistant message
		s.parseAssistantMessage(msg, outputCh)

	case "user":
		// User message (usually tool results or permission denials)
		s.parseUserMessage(msg, outputCh)

	case "result":
		// End of turn - check for permission denials
		if denials, ok := msg["permission_denials"].([]interface{}); ok && len(denials) > 0 {
			for _, d := range denials {
				if denial, ok := d.(map[string]interface{}); ok {
					toolName, _ := denial["tool_name"].(string)
					outputCh <- ClaudeOutput{
						Type:    "permission_denied",
						Tool:    toolName,
						Content: fmt.Sprintf("Permission denied for %s", toolName),
					}
				}
			}
		}
		// Signal completion
		outputCh <- ClaudeOutput{Type: "end", IsEnd: true}

	case "error":
		// Error message
		if errMsg, ok := msg["error"].(string); ok {
			outputCh <- ClaudeOutput{Type: "error", Content: errMsg}
		}

	case "content_block_delta":
		// Streaming text delta - extract and send immediately
		if delta, ok := msg["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok && text != "" {
				outputCh <- ClaudeOutput{Type: "text", Content: text}
			}
		}

	case "content_block_start":
		// Start of content block - could be text or tool_use
		if contentBlock, ok := msg["content_block"].(map[string]interface{}); ok {
			blockType, _ := contentBlock["type"].(string)
			if blockType == "tool_use" {
				toolName, _ := contentBlock["name"].(string)
				toolID, _ := contentBlock["id"].(string)
				outputCh <- ClaudeOutput{Type: "tool_use", Tool: toolName, ToolID: toolID}
			}
		}
	}
}

// parseAssistantMessage extracts content from an assistant message
func (s *Service) parseAssistantMessage(msg map[string]interface{}, ch chan<- ClaudeOutput) {
	message, ok := msg["message"].(map[string]interface{})
	if !ok {
		return
	}

	contents, ok := message["content"].([]interface{})
	if !ok {
		return
	}

	for _, c := range contents {
		block, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		blockType, _ := block["type"].(string)

		switch blockType {
		case "text":
			if text, ok := block["text"].(string); ok {
				ch <- ClaudeOutput{Type: "text", Content: text}
			}

		case "tool_use":
			toolName, _ := block["name"].(string)
			toolID, _ := block["id"].(string)
			var inputStr string
			if input, ok := block["input"].(map[string]interface{}); ok {
				// Format input nicely
				if b, err := json.MarshalIndent(input, "", "  "); err == nil {
					inputStr = string(b)
				}
			}
			ch <- ClaudeOutput{
				Type:    "tool_use",
				Tool:    toolName,
				ToolID:  toolID,
				Content: inputStr,
			}

		case "thinking":
			// Claude's thinking (if enabled)
			if text, ok := block["thinking"].(string); ok {
				ch <- ClaudeOutput{Type: "thinking", Content: text}
			}
		}
	}
}

// parseUserMessage extracts content from user messages (tool results, etc.)
func (s *Service) parseUserMessage(msg map[string]interface{}, ch chan<- ClaudeOutput) {
	// Check for tool_use_result field (permission denials come this way)
	if result, ok := msg["tool_use_result"].(string); ok {
		if strings.Contains(result, "permission") || strings.Contains(result, "Error") {
			ch <- ClaudeOutput{Type: "permission_request", Content: result}
		}
	}
}

// SendMessagePersistent sends a message using the persistent process
// Uses stream-json input format: {"type":"user","message":{"role":"user","content":"..."}}
func (s *Service) SendMessagePersistent(sessionID, message string, outputChan chan<- ClaudeOutput) error {
	s.mu.RLock()
	proc, exists := s.persistentProcs[sessionID]
	sess, sessExists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no persistent process for session: %s", sessionID)
	}
	if !sessExists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Add user message to session
	s.mu.Lock()
	userMsg := Message{
		ID:        generateID(),
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, userMsg)
	sess.State = SessionRunning
	sess.LastActiveAt = time.Now()

	// Create assistant message placeholder
	assistantMsg := Message{
		ID:        generateID(),
		Role:      "assistant",
		Content:   "",
		Timestamp: time.Now(),
		Partial:   true,
	}
	sess.Messages = append(sess.Messages, assistantMsg)
	msgIdx := len(sess.Messages) - 1
	s.mu.Unlock()

	// Build JSON message for stream-json input format
	inputMsg := map[string]interface{}{
		"type": "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": message,
		},
	}
	jsonBytes, err := json.Marshal(inputMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send message to Claude via stdin
	proc.mu.Lock()
	_, err = fmt.Fprintf(proc.stdin, "%s\n", jsonBytes)
	proc.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Read response from output channel in background
	go func() {
		var content strings.Builder
		for output := range proc.outputCh {
			if output.IsEnd {
				break
			}

			if output.Type == "error" {
				if outputChan != nil {
					outputChan <- output
				}
				break
			}

			switch output.Type {
			case "text":
				content.WriteString(output.Content)
				if outputChan != nil {
					outputChan <- output
				}

				// Update message content
				s.mu.Lock()
				if msgIdx < len(sess.Messages) {
					sess.Messages[msgIdx].Content = content.String()
				}
				s.mu.Unlock()

			case "tool_use":
				// Forward tool usage to presenter
				// Check if this tool requires permission
				if requiresPermission(output.Tool) {
					// Set interactive state - waiting for permission
					output.WaitingForInput = true
					output.InputType = "permission"
					s.mu.Lock()
					sess.State = SessionWaiting
					sess.Interactive = &InteractiveState{
						Type:     "permission",
						ToolName: output.Tool,
						ToolID:   output.ToolID,
						FilePath: extractFilePath(output.Content),
					}
					s.mu.Unlock()
				}
				if outputChan != nil {
					outputChan <- output
				}

			case "thinking":
				// Forward thinking to presenter
				if outputChan != nil {
					outputChan <- output
				}

			case "permission_request", "permission_denied":
				// Forward permission events
				if outputChan != nil {
					outputChan <- output
				}

			case "question":
				// Claude is asking a question
				output.WaitingForInput = true
				output.InputType = "question"
				s.mu.Lock()
				sess.State = SessionWaiting
				sess.Interactive = &InteractiveState{
					Type:     "question",
					Question: output.Content,
					Options:  output.Options,
				}
				s.mu.Unlock()
				if outputChan != nil {
					outputChan <- output
				}

			case "plan":
				// Plan mode - waiting for approval
				output.WaitingForInput = true
				output.InputType = "plan"
				s.mu.Lock()
				sess.State = SessionWaiting
				sess.Interactive = &InteractiveState{
					Type:        "plan",
					PlanContent: output.Content,
				}
				s.mu.Unlock()
				if outputChan != nil {
					outputChan <- output
				}

			default:
				// Forward any other output types
				if outputChan != nil {
					outputChan <- output
				}
			}
		}

		// Mark as complete (unless waiting for input)
		s.mu.Lock()
		if sess.State != SessionWaiting {
			sess.State = SessionIdle
		}
		if msgIdx < len(sess.Messages) {
			sess.Messages[msgIdx].Partial = false
		}
		s.mu.Unlock()
		s.saveSessions()

		if outputChan != nil {
			outputChan <- ClaudeOutput{Type: "end", IsEnd: true}
		}
	}()

	return nil
}

// SendInteractiveResponse sends a response for interactive prompts (permissions, questions, etc.)
// responseType: "approve", "deny", "yes", "no", or custom text
func (s *Service) SendInteractiveResponse(sessionID, responseType string, customResponse string) error {
	s.mu.RLock()
	proc, exists := s.persistentProcs[sessionID]
	sess, sessExists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no persistent process for session: %s", sessionID)
	}
	if !sessExists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Build the response message based on type
	var inputMsg map[string]interface{}

	switch responseType {
	case "approve", "yes", "y":
		// Send approval as a control message
		inputMsg = map[string]interface{}{
			"type": "control",
			"control": map[string]interface{}{
				"type": "approve",
			},
		}
	case "deny", "no", "n":
		// Send denial
		inputMsg = map[string]interface{}{
			"type": "control",
			"control": map[string]interface{}{
				"type": "deny",
			},
		}
	default:
		// Send as a user message (for answering questions)
		response := customResponse
		if response == "" {
			response = responseType
		}
		inputMsg = map[string]interface{}{
			"type": "user",
			"message": map[string]interface{}{
				"role":    "user",
				"content": response,
			},
		}
	}

	jsonBytes, err := json.Marshal(inputMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Send response to Claude via stdin
	proc.mu.Lock()
	_, err = fmt.Fprintf(proc.stdin, "%s\n", jsonBytes)
	proc.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}

	// Clear interactive state
	s.mu.Lock()
	sess.Interactive = nil
	sess.State = SessionRunning
	s.mu.Unlock()

	return nil
}

// SetInteractiveState sets the interactive state for a session
func (s *Service) SetInteractiveState(sessionID string, state *InteractiveState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[sessionID]; ok {
		sess.Interactive = state
		if state != nil {
			sess.State = SessionWaiting
		} else {
			sess.State = SessionIdle
		}
	}
}

// GetInteractiveState returns the current interactive state for a session
func (s *Service) GetInteractiveState(sessionID string) *InteractiveState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sess, ok := s.sessions[sessionID]; ok {
		return sess.Interactive
	}
	return nil
}

// ListSessions returns all sessions, optionally filtered by project
func (s *Service) ListSessions(projectID string) []SessionSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summaries []SessionSummary
	for _, sess := range s.sessions {
		if projectID == "" || sess.ProjectID == projectID {
			summaries = append(summaries, sess.ToSummary())
		}
	}
	return summaries
}

// ListSessionsForProject returns sessions for a specific project
func (s *Service) ListSessionsForProject(projectID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*Session
	for _, sess := range s.sessions {
		if sess.ProjectID == projectID {
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

// RenameSession sets a custom name for a session (persisted locally)
func (s *Service) RenameSession(sessionID, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Store the custom name (no prefix required for custom names)
	sess.CustomName = newName
	s.customNames[sessionID] = newName

	// Save custom names to file
	go s.saveCustomNames()
	return nil
}

// ClearSessionCustomName removes the custom name for a session
func (s *Service) ClearSessionCustomName(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sess.CustomName = ""
	delete(s.customNames, sessionID)

	go s.saveCustomNames()
	return nil
}

// DeleteSession removes a session and its JSONL file
func (s *Service) DeleteSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Stop if running
	if sess.State == SessionRunning {
		if cmd, exists := s.activeProcs[sessionID]; exists {
			cmd.Process.Kill()
			delete(s.activeProcs, sessionID)
		}
	}

	// Delete the actual session file (Claude CLI JSONL)
	if sess.SessionFile != "" {
		if err := os.Remove(sess.SessionFile); err != nil && !os.IsNotExist(err) {
			// Log but don't fail - the file might already be gone
			// or we might not have permissions
		}
	}

	// Remove custom name if any
	delete(s.customNames, sessionID)

	// Remove from in-memory map
	delete(s.sessions, sessionID)

	// Save custom names (to remove the deleted session's custom name)
	go s.saveCustomNames()

	return nil
}

// SendMessage sends a message to Claude in a session
func (s *Service) SendMessage(ctx context.Context, sessionID, message string, outputChan chan<- ClaudeOutput) error {
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !s.isInstalled {
		s.mu.Unlock()
		return fmt.Errorf("claude code is not installed")
	}

	// Add user message
	userMsg := Message{
		ID:        generateID(),
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, userMsg)
	sess.State = SessionRunning
	sess.LastActiveAt = time.Now()
	claudePath := s.claudePath
	s.mu.Unlock()

	// Build claude command - use simple text format for direct display
	// -p (print mode) is non-interactive by default
	args := []string{
		"-p", message,
		"--output-format", "text",
	}

	// Use real Claude CLI session ID for persistence
	if sess.IsRealSession && sess.SessionFile != "" {
		if _, err := os.Stat(sess.SessionFile); err == nil {
			// Session file exists, resume it
			args = append(args, "--resume", sessionID)
		} else {
			// New session, use --session-id
			args = append(args, "--session-id", sessionID)
		}
	} else if isValidUUID(sessionID) {
		args = append(args, "--session-id", sessionID)
	}

	cmd := exec.CommandContext(ctx, claudePath, args...)
	cmd.Dir = sess.WorkDir

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	s.mu.Lock()
	s.activeProcs[sessionID] = cmd
	s.mu.Unlock()

	// Create assistant message placeholder
	assistantMsg := Message{
		ID:        generateID(),
		Role:      "assistant",
		Content:   "",
		Timestamp: time.Now(),
		Partial:   true,
	}

	s.mu.Lock()
	sess.Messages = append(sess.Messages, assistantMsg)
	msgIdx := len(sess.Messages) - 1
	s.mu.Unlock()

	// Process output in goroutine
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.activeProcs, sessionID)
			sess.State = SessionIdle
			if msgIdx < len(sess.Messages) {
				sess.Messages[msgIdx].Partial = false
			}
			s.mu.Unlock()
			s.saveSessions()

			if outputChan != nil {
				outputChan <- ClaudeOutput{Type: "end", IsEnd: true}
				close(outputChan)
			}
		}()

		// Read stdout - simple text, line by line
		reader := bufio.NewReader(stdout)
		var fullContent strings.Builder

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					if outputChan != nil {
						outputChan <- ClaudeOutput{Type: "error", Content: err.Error()}
					}
				}
				break
			}

			// Display text as-is
			fullContent.WriteString(line)
			if outputChan != nil {
				outputChan <- ClaudeOutput{Type: "text", Content: line}
			}
		}

		// Read any stderr
		stderrBytes, _ := io.ReadAll(stderr)
		if len(stderrBytes) > 0 {
			errStr := string(stderrBytes)
			if outputChan != nil {
				outputChan <- ClaudeOutput{Type: "error", Content: errStr}
			}
			s.mu.Lock()
			sess.Error = errStr
			s.mu.Unlock()
		}

		// Wait for command to finish
		cmd.Wait()

		// Update message content
		s.mu.Lock()
		if msgIdx < len(sess.Messages) {
			sess.Messages[msgIdx].Content = fullContent.String()
		}
		s.mu.Unlock()
	}()

	return nil
}

// StopSession stops an active Claude session
func (s *Service) StopSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd, ok := s.activeProcs[sessionID]
	if !ok {
		return fmt.Errorf("session not running: %s", sessionID)
	}

	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	delete(s.activeProcs, sessionID)
	if sess, ok := s.sessions[sessionID]; ok {
		sess.State = SessionIdle
	}

	return nil
}

// GetSessionState returns the current state of a session
func (s *Service) GetSessionState(sessionID string) SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return SessionError
	}
	return sess.State
}

// ClearSessionHistory clears the message history of a session
func (s *Service) ClearSessionHistory(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sess.Messages = make([]Message, 0)
	go s.saveSessions()
	return nil
}

// Shutdown cleans up all resources
func (s *Service) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop all active one-shot processes
	for id, cmd := range s.activeProcs {
		cmd.Process.Kill()
		delete(s.activeProcs, id)
	}

	// Stop all persistent processes
	for id, proc := range s.persistentProcs {
		proc.cancel()
		proc.stdin.Close()
		proc.cmd.Wait()
		delete(s.persistentProcs, id)
	}

	// Final save
	s.saveSessions()
}

// requiresPermission checks if a tool requires user permission before execution
// Tools like Write, Bash, Edit typically need approval unless auto-approved
func requiresPermission(toolName string) bool {
	// Tools that modify files or execute commands require permission
	dangerousTools := map[string]bool{
		"Write":        true,
		"Edit":         true,
		"Bash":         true,
		"bash":         true,
		"NotebookEdit": true,
		"KillShell":    true,
	}
	return dangerousTools[toolName]
}

// extractFilePath extracts file path from tool input JSON
func extractFilePath(input string) string {
	// Try to parse as JSON and extract file_path
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}

	// Try common field names
	for _, field := range []string{"file_path", "path", "filePath"} {
		if path, ok := data[field].(string); ok {
			return path
		}
	}
	return ""
}

// extractTextFromJSON extracts text content from Claude CLI JSON output
// Handles various formats that Claude CLI might produce
func extractTextFromJSON(output map[string]interface{}) string {
	var result strings.Builder

	// Check for "type" field to determine format
	outputType, _ := output["type"].(string)

	switch outputType {
	case "assistant":
		// Format: {"type": "assistant", "message": {"content": [{"type": "text", "text": "..."}]}}
		if msg, ok := output["message"].(map[string]interface{}); ok {
			if contents, ok := msg["content"].([]interface{}); ok {
				for _, c := range contents {
					if block, ok := c.(map[string]interface{}); ok {
						if block["type"] == "text" {
							if text, ok := block["text"].(string); ok {
								result.WriteString(text)
							}
						}
					}
				}
			}
		}

	case "content_block_delta":
		// Streaming: {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "..."}}
		if delta, ok := output["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				result.WriteString(text)
			}
		}

	case "message":
		// Format: {"type": "message", "content": [{"type": "text", "text": "..."}]}
		if contents, ok := output["content"].([]interface{}); ok {
			for _, c := range contents {
				if block, ok := c.(map[string]interface{}); ok {
					if block["type"] == "text" {
						if text, ok := block["text"].(string); ok {
							result.WriteString(text)
						}
					}
				}
			}
		}

	case "result":
		// Final result: {"type": "result", "result": "...", "cost": ...}
		if text, ok := output["result"].(string); ok {
			result.WriteString(text)
		}

	default:
		// Try common fields
		if content, ok := output["content"].(string); ok {
			result.WriteString(content)
		}
		if text, ok := output["text"].(string); ok {
			result.WriteString(text)
		}
		if message, ok := output["message"].(string); ok {
			result.WriteString(message)
		}
		if data, ok := output["data"].(string); ok {
			result.WriteString(data)
		}
	}

	return result.String()
}
