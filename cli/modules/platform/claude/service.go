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
	"strings"
	"sync"
	"time"
)

// generateID generates a unique ID for sessions
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
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
		dataDir:         dataDir,
		activeProcs:     make(map[string]*exec.Cmd),
		persistentProcs: make(map[string]*persistentProcess),
		outputChans:     make(map[string]chan ClaudeOutput),
	}
	s.detectClaude()
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

// sessionsFilePath returns the path to the sessions JSON file
func (s *Service) sessionsFilePath() string {
	return filepath.Join(s.dataDir, "claude-sessions.json")
}

// loadSessions loads sessions from disk
func (s *Service) loadSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.sessionsFilePath())
	if err != nil {
		return // File doesn't exist yet
	}

	var sessions []*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return
	}

	for _, sess := range sessions {
		// Reset running sessions to idle on load
		if sess.State == SessionRunning {
			sess.State = SessionIdle
		}
		s.sessions[sess.ID] = sess
	}
}

// saveSessions persists sessions to disk
func (s *Service) saveSessions() error {
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.sessionsFilePath(), data, 0644)
}

// CreateSession creates a new Claude session for a project
func (s *Service) CreateSession(projectID, projectName, workDir, name string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	session := &Session{
		ID:           generateID(),
		Name:         name,
		ProjectID:    projectID,
		ProjectName:  projectName,
		WorkDir:      workDir,
		State:        SessionIdle,
		Messages:     make([]Message, 0),
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}

	s.sessions[session.ID] = session
	go s.saveSessions()

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

	// Use --continue if we have session history
	if len(sess.Messages) > 0 {
		args = append(args, "--continue")
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

// RenameSession renames a session (keeps project prefix)
func (s *Service) RenameSession(sessionID, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Ensure name has project prefix
	if !strings.HasPrefix(newName, sess.ProjectID+"-") {
		newName = sess.ProjectID + "-" + newName
	}

	sess.Name = newName
	go s.saveSessions()
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

	// If session has history, use --continue to maintain context
	if len(sess.Messages) > 1 {
		args = append(args, "--continue")
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
