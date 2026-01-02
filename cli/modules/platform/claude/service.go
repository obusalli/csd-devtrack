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

// Service manages Claude Code integration
type Service struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	dataDir      string
	claudePath   string
	isInstalled  bool
	activeProcs  map[string]*exec.Cmd
	outputChans  map[string]chan ClaudeOutput
}

// NewService creates a new Claude service
func NewService(dataDir string) *Service {
	s := &Service{
		sessions:    make(map[string]*Session),
		dataDir:     dataDir,
		activeProcs: make(map[string]*exec.Cmd),
		outputChans: make(map[string]chan ClaudeOutput),
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

	// Build claude command
	args := []string{
		"-p", message,
		"--output-format", "stream-json",
	}

	// If session has history, use --continue
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

		// Read stdout (JSON stream)
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

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Parse JSON output
			var output map[string]interface{}
			if err := json.Unmarshal([]byte(line), &output); err != nil {
				// Not JSON, treat as plain text
				fullContent.WriteString(line)
				if outputChan != nil {
					outputChan <- ClaudeOutput{Type: "text", Content: line}
				}
				continue
			}

			// Handle different output types
			if content, ok := output["content"].(string); ok {
				fullContent.WriteString(content)
				if outputChan != nil {
					outputChan <- ClaudeOutput{Type: "text", Content: content}
				}
			}

			if toolUse, ok := output["tool_use"].(map[string]interface{}); ok {
				toolName := ""
				if name, ok := toolUse["name"].(string); ok {
					toolName = name
				}
				if outputChan != nil {
					outputChan <- ClaudeOutput{Type: "tool_use", Tool: toolName}
				}
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

	for id, cmd := range s.activeProcs {
		cmd.Process.Kill()
		delete(s.activeProcs, id)
	}

	// Final save
	s.saveSessions()
}
