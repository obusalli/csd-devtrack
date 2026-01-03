package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TerminalScreen represents a terminal using GNU screen
type TerminalScreen struct {
	mu sync.RWMutex

	// Session info
	SessionID   string
	WorkDir     string
	ClaudePath  string
	screenName  string

	// Terminal state
	width   int
	height  int
	state   TerminalState
	content string

	// Capture file
	captureFile string

	// ESC ESC detection
	lastEscTime time.Time

	// Callbacks
	onOutput func()
	onExit   func()

	// Stop channel
	stopCh chan struct{}
}

// NewTerminalScreen creates a new screen-based terminal
func NewTerminalScreen(sessionID, workDir, claudePath string) *TerminalScreen {
	// Generate unique screen name with csd-dt- prefix
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	screenName := fmt.Sprintf("csd-dt-%s", shortID)
	captureFile := fmt.Sprintf("/tmp/csd-dt-capture-%s.txt", shortID)

	return &TerminalScreen{
		SessionID:   sessionID,
		WorkDir:     workDir,
		ClaudePath:  claudePath,
		screenName:  screenName,
		captureFile: captureFile,
		width:       80,
		height:      24,
		state:       TerminalIdle,
		stopCh:      make(chan struct{}),
	}
}

// SetSize sets the terminal size
func (t *TerminalScreen) SetSize(width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if width < 10 {
		width = 10
	}
	if height < 5 {
		height = 5
	}

	if t.width == width && t.height == height {
		return
	}

	t.width = width
	t.height = height

	// Resize screen session if running
	if t.state == TerminalRunning {
		// screen doesn't have a direct resize command, but we can try
		exec.Command("screen", "-S", t.screenName, "-X", "width", "-w", fmt.Sprintf("%d", width)).Run()
		exec.Command("screen", "-S", t.screenName, "-X", "height", "-w", fmt.Sprintf("%d", height)).Run()
	}
}

// Start starts Claude in a screen session
func (t *TerminalScreen) Start(sessionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TerminalRunning {
		return nil
	}

	// Build command for Claude
	claudeCmd := t.ClaudePath
	if sessionID != "" && isValidUUID(sessionID) {
		claudeCmd = fmt.Sprintf("%s --resume %s", t.ClaudePath, sessionID)
	}

	// Start screen session with Claude
	// -dmS: detached, create session with name
	// -s: shell to use (we use bash -c to run claude)
	cmd := exec.Command("screen", "-dmS", t.screenName, "-s", "/bin/bash")
	cmd.Dir = t.WorkDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start screen: %w", err)
	}

	// Wait a bit for screen to start
	time.Sleep(100 * time.Millisecond)

	// Send the claude command to screen
	stuffCmd := exec.Command("screen", "-S", t.screenName, "-X", "stuff", claudeCmd+"\n")
	stuffCmd.Dir = t.WorkDir
	if err := stuffCmd.Run(); err != nil {
		return fmt.Errorf("failed to send command to screen: %w", err)
	}

	t.state = TerminalRunning
	t.stopCh = make(chan struct{})

	// Start capture loop
	go t.captureLoop()

	// Monitor for exit
	go t.monitorLoop()

	return nil
}

// captureLoop periodically captures screen content
func (t *TerminalScreen) captureLoop() {
	ticker := time.NewTicker(150 * time.Millisecond)
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

// capture captures the current screen content
func (t *TerminalScreen) capture() {
	// Use hardcopy to capture screen content (without -h to get only visible area)
	cmd := exec.Command("screen", "-S", t.screenName, "-X", "hardcopy", t.captureFile)
	if err := cmd.Run(); err != nil {
		return
	}

	// Read captured content
	data, err := os.ReadFile(t.captureFile)
	if err != nil {
		return
	}

	t.mu.Lock()
	newContent := string(data)
	changed := newContent != t.content
	t.content = newContent
	t.mu.Unlock()

	if changed && t.onOutput != nil {
		t.onOutput()
	}
}

// monitorLoop monitors the screen session
func (t *TerminalScreen) monitorLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			// Check if screen session still exists
			cmd := exec.Command("screen", "-ls", t.screenName)
			output, _ := cmd.Output()
			if !strings.Contains(string(output), t.screenName) {
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
func (t *TerminalScreen) Write(data []byte) error {
	t.mu.RLock()
	if t.state != TerminalRunning {
		t.mu.RUnlock()
		return nil
	}
	t.mu.RUnlock()

	// Send keys using screen stuff command
	// We need to escape special characters
	text := string(data)
	cmd := exec.Command("screen", "-S", t.screenName, "-X", "stuff", text)
	return cmd.Run()
}

// WriteString sends a string to the terminal
func (t *TerminalScreen) WriteString(s string) error {
	return t.Write([]byte(s))
}

// HandleKey processes a key press
func (t *TerminalScreen) HandleKey(key string) (consumed bool, exitTerminal bool) {
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

	data := keyToBytes(key)
	if data != nil {
		t.Write(data)
	}

	return true, false
}

// View returns the terminal view
func (t *TerminalScreen) View() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.content == "" {
		return "[Waiting for screen...]"
	}

	// Split into lines and limit to terminal height
	lines := strings.Split(t.content, "\n")

	// Take last 'height' lines if we have more
	if len(lines) > t.height {
		lines = lines[len(lines)-t.height:]
	}

	// Truncate each line to width
	var result []string
	for _, line := range lines {
		if len(line) > t.width {
			line = line[:t.width]
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// State returns the current terminal state
func (t *TerminalScreen) State() TerminalState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// IsRunning returns true if the terminal is running
func (t *TerminalScreen) IsRunning() bool {
	return t.State() == TerminalRunning
}

// Stop stops the terminal
func (t *TerminalScreen) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopCh != nil {
		close(t.stopCh)
	}

	// Kill the screen session
	exec.Command("screen", "-S", t.screenName, "-X", "quit").Run()

	// Clean up capture file
	os.Remove(t.captureFile)

	t.state = TerminalExited
}

// SetCallbacks sets the callback functions
func (t *TerminalScreen) SetCallbacks(onOutput, onExit func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onOutput = onOutput
	t.onExit = onExit
}

// LineCount returns the number of lines
func (t *TerminalScreen) LineCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(strings.Split(t.content, "\n"))
}

// Width returns the terminal width
func (t *TerminalScreen) Width() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.width
}

// Height returns the terminal height
func (t *TerminalScreen) Height() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.height
}

// GetLines returns all lines
func (t *TerminalScreen) GetLines() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return strings.Split(t.content, "\n")
}
