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
func formatToolUseForDisplay(toolName string, input map[string]interface{}) string {
	var sb strings.Builder

	switch toolName {
	case "Read":
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			filePath, _ = input["path"].(string)
		}
		fileName := filepath.Base(filePath)
		sb.WriteString(fmt.Sprintf("● Read(%s)\n", fileName))
		if limit, ok := input["limit"].(float64); ok {
			sb.WriteString(fmt.Sprintf("  ⎿  Read %d lines\n", int(limit)))
		} else {
			sb.WriteString("  ⎿  Read file\n")
		}

	case "Edit":
		filePath, _ := input["file_path"].(string)
		fileName := filepath.Base(filePath)
		oldStr, _ := input["old_string"].(string)
		newStr, _ := input["new_string"].(string)

		sb.WriteString(fmt.Sprintf("● Update(%s)\n", fileName))

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

		// Format diff with markers
		sb.WriteString(formatDiffWithMarkers(oldStr, newStr, 2))

	case "Write":
		filePath, _ := input["file_path"].(string)
		fileName := filepath.Base(filePath)
		contentStr, _ := input["content"].(string)
		lines := strings.Count(contentStr, "\n") + 1
		sb.WriteString(fmt.Sprintf("● Write(%s)\n", fileName))
		sb.WriteString(fmt.Sprintf("  ⎿  Wrote %d lines\n", lines))

	case "Bash":
		cmd, _ := input["command"].(string)
		desc, _ := input["description"].(string)
		if desc != "" {
			sb.WriteString(fmt.Sprintf("● Bash(%s)\n", desc))
		} else {
			cmdDisplay := cmd
			if len(cmdDisplay) > 50 {
				cmdDisplay = cmdDisplay[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("● Bash(%s)\n", cmdDisplay))
		}
		sb.WriteString("  ⎿  Executed\n")

	case "Glob", "Grep":
		pattern, _ := input["pattern"].(string)
		if len(pattern) > 40 {
			pattern = pattern[:40] + "..."
		}
		sb.WriteString(fmt.Sprintf("● %s(%s)\n", toolName, pattern))
		sb.WriteString("  ⎿  Searched\n")

	default:
		sb.WriteString(fmt.Sprintf("● %s\n", toolName))
	}

	return sb.String()
}

// formatDiffWithMarkers creates a diff with {{-...}} and {{+...}} markers
func formatDiffWithMarkers(oldStr, newStr string, contextLines int) string {
	var sb strings.Builder

	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

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

	lineNum := startContext + 1
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

	// Added lines
	lineNum = commonPrefix + 1
	addedEnd := len(newLines) - commonSuffix
	for i := commonPrefix; i < addedEnd; i++ {
		sb.WriteString(fmt.Sprintf("{{+   %3d + %s}}\n", lineNum, newLines[i]))
		lineNum++
	}

	// Context after
	if commonSuffix > 0 && commonSuffix <= contextLines {
		contextStart := len(newLines) - commonSuffix
		lineNum = contextStart + 1
		for i := contextStart; i < len(newLines); i++ {
			sb.WriteString(fmt.Sprintf("      %3d   %s\n", lineNum, newLines[i]))
			lineNum++
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

// sessionWatcher watches a session file for changes
type sessionWatcher struct {
	sessionID  string
	filePath   string
	lastOffset int64
	cancel     context.CancelFunc
	outputCh   chan SessionUpdate
}

// SessionUpdate represents an update from watching a session
type SessionUpdate struct {
	SessionID string
	Type      string // "user", "assistant", "tool_use", "tool_result", "end"
	Content   string
	Role      string
	ToolName  string
	ToolInput map[string]interface{}
	Timestamp time.Time
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
	watchers        map[string]*sessionWatcher // File watchers for external sessions
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
		watchers:        make(map[string]*sessionWatcher),
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

		// Skip empty sessions
		if info.Size() == 0 {
			continue
		}

		// Load session metadata from JSONL (workDir will be extracted from cwd field)
		session := s.parseSessionFile(sessionID, sessionFile)
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
					if c, ok := msg["content"].([]interface{}); ok {
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

	// Generate session name with project prefix if not provided
	if name == "" {
		// Count existing sessions for this project
		count := 0
		for _, sess := range s.sessions {
			if sess.ProjectID == projectID {
				count++
			}
		}
		name = fmt.Sprintf("%s-session-%d", projectID, count+1)
	} else {
		// Ensure name has project prefix
		if !strings.HasPrefix(name, projectID+"-") {
			name = projectID + "-" + name
		}
	}

	// Build the expected session file path
	claudeProjectName := workDirToClaudeProject(workDir)
	sessionFile := filepath.Join(claudeProjectsDir(), claudeProjectName, sessionID+".jsonl")

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

// DeleteSession removes a session
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

	delete(s.sessions, sessionID)
	go s.saveSessions()
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

// WatchSession starts watching a session file for real-time updates
// Useful for following sessions running outside of csd-devtrack
func (s *Service) WatchSession(sessionID string) (chan SessionUpdate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already watching
	if w, exists := s.watchers[sessionID]; exists {
		return w.outputCh, nil
	}

	// Find session file
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if sess.SessionFile == "" {
		return nil, fmt.Errorf("session has no file: %s", sessionID)
	}

	// Get current file size as starting offset
	info, err := os.Stat(sess.SessionFile)
	if err != nil {
		return nil, fmt.Errorf("cannot stat session file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	outputCh := make(chan SessionUpdate, 100)

	watcher := &sessionWatcher{
		sessionID:  sessionID,
		filePath:   sess.SessionFile,
		lastOffset: info.Size(),
		cancel:     cancel,
		outputCh:   outputCh,
	}

	s.watchers[sessionID] = watcher

	// Start watching goroutine
	go s.watchSessionFile(ctx, watcher)

	return outputCh, nil
}

// StopWatchSession stops watching a session
func (s *Service) StopWatchSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, exists := s.watchers[sessionID]; exists {
		w.cancel()
		close(w.outputCh)
		delete(s.watchers, sessionID)
	}
}

// IsWatching returns true if a session is being watched
func (s *Service) IsWatching(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.watchers[sessionID]
	return exists
}

// watchSessionFile polls the session file for new content
func (s *Service) watchSessionFile(ctx context.Context, w *sessionWatcher) {
	ticker := time.NewTicker(500 * time.Millisecond) // Poll every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkSessionFileUpdates(w)
		}
	}
}

// checkSessionFileUpdates reads new lines from the session file
func (s *Service) checkSessionFileUpdates(w *sessionWatcher) {
	info, err := os.Stat(w.filePath)
	if err != nil {
		return
	}

	// Check if file has grown
	if info.Size() <= w.lastOffset {
		return
	}

	// Open and seek to last position
	file, err := os.Open(w.filePath)
	if err != nil {
		return
	}
	defer file.Close()

	if _, err := file.Seek(w.lastOffset, 0); err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		update := s.parseSessionLine(w.sessionID, line)
		if update != nil {
			select {
			case w.outputCh <- *update:
			default:
				// Channel full, skip
			}
		}
	}

	// Update offset
	newPos, _ := file.Seek(0, 1) // Get current position
	w.lastOffset = newPos
}

// parseSessionLine parses a single JSONL line into a SessionUpdate
func (s *Service) parseSessionLine(sessionID string, line []byte) *SessionUpdate {
	var entry map[string]interface{}
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	entryType, _ := entry["type"].(string)

	var timestamp time.Time
	if ts, ok := entry["timestamp"].(string); ok {
		timestamp, _ = time.Parse(time.RFC3339, ts)
	}

	switch entryType {
	case "user", "assistant":
		// Parse message content
		msg, ok := entry["message"].(map[string]interface{})
		if !ok {
			return nil
		}

		role, _ := msg["role"].(string)
		var contentBuilder strings.Builder

		if contents, ok := msg["content"].([]interface{}); ok {
			for _, item := range contents {
				block, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				blockType, _ := block["type"].(string)
				switch blockType {
				case "text":
					if text, ok := block["text"].(string); ok {
						contentBuilder.WriteString(text)
					}
				case "tool_use":
					toolName, _ := block["name"].(string)
					input, _ := block["input"].(map[string]interface{})
					contentBuilder.WriteString("\n")
					contentBuilder.WriteString(formatToolUseForDisplay(toolName, input))
				case "tool_result":
					// Tool result
					if content, ok := block["content"].(string); ok {
						contentBuilder.WriteString("\n  ⎿  ")
						if len(content) > 100 {
							contentBuilder.WriteString(content[:100] + "...")
						} else {
							contentBuilder.WriteString(content)
						}
						contentBuilder.WriteString("\n")
					}
				}
			}
		}

		return &SessionUpdate{
			SessionID: sessionID,
			Type:      entryType,
			Role:      role,
			Content:   contentBuilder.String(),
			Timestamp: timestamp,
		}

	case "result":
		// End of turn
		return &SessionUpdate{
			SessionID: sessionID,
			Type:      "end",
			Timestamp: timestamp,
		}
	}

	return nil
}

// Shutdown cleans up all resources
func (s *Service) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop all watchers
	for id, w := range s.watchers {
		w.cancel()
		close(w.outputCh)
		delete(s.watchers, id)
	}

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
