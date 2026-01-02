package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TerminalTmux represents a terminal using tmux
type TerminalTmux struct {
	mu sync.RWMutex

	// Session info
	SessionID  string
	WorkDir    string
	ClaudePath string
	tmuxName   string

	// Terminal state
	width        int
	height       int
	state        TerminalState
	content      string
	pendingStart bool   // true if Start() was called but session not yet created
	startSession string // session ID to resume when actually starting

	// Scrolling
	scrollOffset int // 0 = at bottom, positive = scrolled up
	totalLines   int

	// ESC ESC detection
	lastEscTime time.Time

	// Callbacks
	onOutput func()
	onExit   func()

	// Stop channel
	stopCh chan struct{}
}

// NewTerminalTmux creates a new tmux-based terminal
func NewTerminalTmux(sessionID, workDir, claudePath string) *TerminalTmux {
	// Generate unique tmux session name with csd-dt- prefix
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	tmuxName := fmt.Sprintf("csd-dt-%s", shortID)

	return &TerminalTmux{
		SessionID:  sessionID,
		WorkDir:    workDir,
		ClaudePath: claudePath,
		tmuxName:   tmuxName,
		width:      80,
		height:     24,
		state:      TerminalIdle,
		stopCh:     make(chan struct{}),
	}
}

// SetSize sets the terminal size
func (t *TerminalTmux) SetSize(width, height int) {
	t.mu.Lock()

	if width < 10 {
		width = 10
	}
	if height < 5 {
		height = 5
	}

	if t.width == width && t.height == height {
		t.mu.Unlock()
		return
	}

	t.width = width
	t.height = height

	// If we have a pending start and now have real dimensions, start the session
	if t.pendingStart && (width != 80 || height != 24) {
		sessionID := t.startSession
		t.mu.Unlock()
		t.doStart(sessionID)
		return
	}

	// Resize tmux pane if running
	if t.state == TerminalRunning {
		tmuxName := t.tmuxName
		t.mu.Unlock()

		// Set window-size to manual to allow resize of detached session
		exec.Command("tmux", "set-option", "-t", tmuxName, "window-size", "manual").Run()
		exec.Command("tmux", "resize-window", "-t", tmuxName, "-x", fmt.Sprintf("%d", width), "-y", fmt.Sprintf("%d", height)).Run()

		// Send SIGWINCH to process inside
		pidOut, _ := exec.Command("tmux", "display-message", "-t", tmuxName, "-p", "#{pane_pid}").Output()
		if pid := strings.TrimSpace(string(pidOut)); pid != "" {
			exec.Command("kill", "-WINCH", pid).Run()
		}
		return
	}
	t.mu.Unlock()
}

// Start starts Claude in a tmux session
func (t *TerminalTmux) Start(sessionID string) error {
	t.mu.Lock()

	if t.state == TerminalRunning {
		t.mu.Unlock()
		return nil
	}

	// Check if session already exists
	checkCmd := exec.Command("tmux", "has-session", "-t", t.tmuxName)
	if err := checkCmd.Run(); err == nil {
		// Session exists, reuse it - force resize with proper method
		exec.Command("tmux", "set-option", "-t", t.tmuxName, "window-size", "manual").Run()
		exec.Command("tmux", "resize-window", "-t", t.tmuxName, "-x", fmt.Sprintf("%d", t.width), "-y", fmt.Sprintf("%d", t.height)).Run()
		t.state = TerminalRunning
		t.stopCh = make(chan struct{})
		t.mu.Unlock()
		go t.captureLoop()
		go t.monitorLoop()
		return nil
	}

	// If dimensions are still default (80x24), defer actual start until SetSize is called
	if t.width == 80 && t.height == 24 {
		t.pendingStart = true
		t.startSession = sessionID
		t.mu.Unlock()
		return nil
	}

	// Actually create the session (release lock first)
	t.mu.Unlock()
	return t.doStart(sessionID)
}

// doStart actually creates the tmux session (called when dimensions are known)
// Note: caller must NOT hold the lock
func (t *TerminalTmux) doStart(sessionID string) error {
	t.mu.RLock()
	width := t.width
	height := t.height
	tmuxName := t.tmuxName
	workDir := t.WorkDir
	claudePath := t.ClaudePath
	t.mu.RUnlock()

	// Build command for Claude
	claudeArgs := []string{}
	if sessionID != "" && isValidUUID(sessionID) {
		claudeArgs = append(claudeArgs, "--resume", sessionID)
	}

	// Create new tmux session with Claude
	// -d: detached
	// -s: session name
	// -x/-y: dimensions
	args := []string{"new-session", "-d", "-s", tmuxName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
		claudePath}
	args = append(args, claudeArgs...)

	cmd := exec.Command("tmux", args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start tmux session: %w", err)
	}

	// Set window-size to manual and resize
	exec.Command("tmux", "set-option", "-t", tmuxName, "window-size", "manual").Run()
	exec.Command("tmux", "resize-window", "-t", tmuxName, "-x", fmt.Sprintf("%d", width), "-y", fmt.Sprintf("%d", height)).Run()

	// Wait for Claude to start, then send SIGWINCH via tmux respawn
	time.Sleep(200 * time.Millisecond)

	// Get pane PID and send SIGWINCH
	pidOut, _ := exec.Command("tmux", "display-message", "-t", tmuxName, "-p", "#{pane_pid}").Output()
	if pid := strings.TrimSpace(string(pidOut)); pid != "" {
		// Send SIGWINCH to the process group
		exec.Command("kill", "-WINCH", pid).Run()
	}

	t.mu.Lock()
	t.state = TerminalRunning
	t.pendingStart = false
	t.stopCh = make(chan struct{})
	t.mu.Unlock()

	// Start capture loop
	go t.captureLoop()

	// Monitor for exit
	go t.monitorLoop()

	return nil
}

// captureLoop periodically captures tmux pane content
func (t *TerminalTmux) captureLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.capture()
		}
	}
}

// capture captures the current tmux pane content with ANSI codes
func (t *TerminalTmux) capture() {
	// capture-pane -p: print to stdout
	// capture-pane -e: include escape sequences (colors!)
	// capture-pane -S: start line (negative = scrollback)
	// Capture more history for scrollback (-500 lines before visible)
	cmd := exec.Command("tmux", "capture-pane", "-t", t.tmuxName, "-p", "-e", "-S", "-500")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	t.mu.Lock()
	newContent := string(output)
	changed := newContent != t.content
	t.content = newContent
	t.totalLines = len(strings.Split(newContent, "\n"))
	t.mu.Unlock()

	if changed && t.onOutput != nil {
		t.onOutput()
	}
}

// monitorLoop monitors the tmux session
func (t *TerminalTmux) monitorLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			// Check if tmux session still exists
			cmd := exec.Command("tmux", "has-session", "-t", t.tmuxName)
			if err := cmd.Run(); err != nil {
				// Session no longer exists
				t.mu.Lock()
				t.state = TerminalExited
				t.mu.Unlock()
				if t.onExit != nil {
					t.onExit()
				}
				return
			}
		}
	}
}

// Write sends input to the terminal
func (t *TerminalTmux) Write(data []byte) error {
	t.mu.RLock()
	if t.state != TerminalRunning {
		t.mu.RUnlock()
		return nil
	}
	t.mu.RUnlock()

	// Send keys using tmux send-keys
	// -l: literal (don't interpret special characters)
	// We need to be careful with special keys
	text := string(data)

	// For control characters and escape sequences, don't use -l
	if len(data) == 1 && data[0] < 32 {
		// Control character - send as key name or hex
		switch data[0] {
		case 0x03:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-c").Run()
		case 0x04:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-d").Run()
		case 0x1a:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-z").Run()
		case 0x0c:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-l").Run()
		case 0x01:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-a").Run()
		case 0x05:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-e").Run()
		case 0x0b:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-k").Run()
		case 0x15:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-u").Run()
		case 0x17:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "C-w").Run()
		case '\r':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Enter").Run()
		case '\t':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Tab").Run()
		case 0x7f:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "BSpace").Run()
		case 0x1b:
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Escape").Run()
		}
	}

	// Check for escape sequences (arrows, etc.)
	if len(data) >= 3 && data[0] == 0x1b && data[1] == '[' {
		switch data[2] {
		case 'A':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Up").Run()
		case 'B':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Down").Run()
		case 'C':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Right").Run()
		case 'D':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Left").Run()
		case 'H':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "Home").Run()
		case 'F':
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "End").Run()
		}
		if len(data) >= 4 && data[2] == '5' && data[3] == '~' {
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "PPage").Run()
		}
		if len(data) >= 4 && data[2] == '6' && data[3] == '~' {
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "NPage").Run()
		}
		if len(data) >= 4 && data[2] == '3' && data[3] == '~' {
			return exec.Command("tmux", "send-keys", "-t", t.tmuxName, "DC").Run()
		}
	}

	// Regular text - send literally
	cmd := exec.Command("tmux", "send-keys", "-t", t.tmuxName, "-l", text)
	return cmd.Run()
}

// WriteString sends a string to the terminal
func (t *TerminalTmux) WriteString(s string) error {
	return t.Write([]byte(s))
}

// HandleKey processes a key press
func (t *TerminalTmux) HandleKey(key string) (consumed bool, exitTerminal bool) {
	// Check for ESC ESC
	if key == "esc" {
		now := time.Now()
		if now.Sub(t.lastEscTime) < 300*time.Millisecond {
			t.lastEscTime = time.Time{}
			return true, true
		}
		t.lastEscTime = now
		go func() {
			time.Sleep(350 * time.Millisecond)
			t.mu.RLock()
			lastEsc := t.lastEscTime
			t.mu.RUnlock()
			if !lastEsc.IsZero() && time.Since(lastEsc) >= 300*time.Millisecond {
				t.Write([]byte{0x1b})
				t.mu.Lock()
				t.lastEscTime = time.Time{}
				t.mu.Unlock()
			}
		}()
		return true, false
	}

	if !t.lastEscTime.IsZero() {
		t.Write([]byte{0x1b})
		t.lastEscTime = time.Time{}
	}

	// Handle scrolling locally (don't send to tmux)
	if key == "pgup" {
		t.ScrollUp(t.height / 2)
		return true, false
	}
	if key == "pgdown" {
		t.ScrollDown(t.height / 2)
		return true, false
	}

	data := keyToBytes(key)
	if data != nil {
		t.Write(data)
	}

	return true, false
}

// ScrollUp scrolls the view up
func (t *TerminalTmux) ScrollUp(lines int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.scrollOffset += lines
	maxScroll := t.totalLines - t.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scrollOffset > maxScroll {
		t.scrollOffset = maxScroll
	}
}

// ScrollDown scrolls the view down
func (t *TerminalTmux) ScrollDown(lines int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.scrollOffset -= lines
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
}

// ScrollToBottom scrolls to the bottom
func (t *TerminalTmux) ScrollToBottom() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scrollOffset = 0
}

// View returns the terminal view
func (t *TerminalTmux) View() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.content == "" {
		return "[Waiting for tmux...]"
	}

	// The content already includes ANSI codes from capture-pane -e
	lines := strings.Split(t.content, "\n")

	// Remove trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	totalLines := len(lines)
	visibleHeight := t.height
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Calculate visible window based on scroll offset
	// scrollOffset 0 = bottom, positive = scrolled up
	endLine := totalLines - t.scrollOffset
	if endLine > totalLines {
		endLine = totalLines
	}
	if endLine < 1 {
		endLine = 1
	}

	startLine := endLine - visibleHeight
	if startLine < 0 {
		startLine = 0
	}

	// Extract visible lines
	var visibleLines []string
	if startLine < totalLines {
		end := endLine
		if end > totalLines {
			end = totalLines
		}
		visibleLines = lines[startLine:end]
	}

	// Build result - no padding here, renderTerminalPanel handles it
	result := strings.Join(visibleLines, "\n")

	// Add scroll indicator at end if scrolled up
	if t.scrollOffset > 0 {
		result += "\n" + lipglossStyle(fmt.Sprintf("[↑ %d lines - PgDn: down]", t.scrollOffset))
	}

	return result
}

// truncateANSILine truncates a line with ANSI codes to visible width
func truncateANSILine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	var result strings.Builder
	visibleLen := 0
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}

		if inEscape {
			result.WriteRune(r)
			// End of escape sequence
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		// Visible character
		if visibleLen >= maxWidth {
			break
		}
		result.WriteRune(r)
		visibleLen++
	}

	return result.String()
}

// lipglossStyle returns a muted style string (helper to avoid import in View)
func lipglossStyle(s string) string {
	return "\x1b[90m" + s + "\x1b[0m" // Gray/muted color
}

// State returns the current terminal state
func (t *TerminalTmux) State() TerminalState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// IsRunning returns true if the terminal is running
func (t *TerminalTmux) IsRunning() bool {
	return t.State() == TerminalRunning
}

// Stop stops the terminal
func (t *TerminalTmux) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopCh != nil {
		close(t.stopCh)
	}

	// Kill the tmux session
	exec.Command("tmux", "kill-session", "-t", t.tmuxName).Run()

	t.state = TerminalExited
}

// SetCallbacks sets the callback functions
func (t *TerminalTmux) SetCallbacks(onOutput, onExit func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onOutput = onOutput
	t.onExit = onExit
}

// LineCount returns the number of lines
func (t *TerminalTmux) LineCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(strings.Split(t.content, "\n"))
}

// Width returns the terminal width
func (t *TerminalTmux) Width() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.width
}

// Height returns the terminal height
func (t *TerminalTmux) Height() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.height
}

// GetLines returns all lines
func (t *TerminalTmux) GetLines() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return strings.Split(t.content, "\n")
}
