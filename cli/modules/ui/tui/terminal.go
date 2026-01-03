package tui

import (
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/taigrr/bubbleterm/emulator"
)

// TerminalState represents the state of a terminal session
type TerminalState int

const (
	TerminalIdle TerminalState = iota
	TerminalRunning
	TerminalWaiting // Claude waiting for input
	TerminalExited
)

// Terminal represents an embedded terminal for a Claude session
type Terminal struct {
	mu sync.RWMutex

	// Session info
	SessionID   string
	WorkDir     string
	ClaudePath  string
	SessionFile string

	// bubbleterm emulator
	emu *emulator.Emulator

	// Terminal state
	width  int
	height int

	// State
	state TerminalState

	// ESC ESC detection
	lastEscTime time.Time

	// Callbacks
	onOutput func() // Called when new output is available
	onExit   func() // Called when process exits
}

// NewTerminal creates a new terminal for a Claude session
func NewTerminal(sessionID, workDir, claudePath string) *Terminal {
	return &Terminal{
		SessionID:  sessionID,
		WorkDir:    workDir,
		ClaudePath: claudePath,
		width:      80,
		height:     24,
		state:      TerminalIdle,
	}
}

// SetSize sets the terminal size (only resizes if dimensions changed)
func (t *Terminal) SetSize(width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if width < 10 {
		width = 10
	}
	if height < 5 {
		height = 5
	}

	// Only resize if dimensions actually changed
	if t.width == width && t.height == height {
		return
	}

	t.width = width
	t.height = height

	// Resize emulator if initialized
	if t.emu != nil {
		t.emu.Resize(width, height)
	}
}

// Start starts Claude in the terminal
// If sessionID is provided and is a valid UUID, it will try to resume that session
func (t *Terminal) Start(sessionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TerminalRunning {
		return nil // Already running
	}

	// Create emulator
	emu, err := emulator.New(t.width, t.height)
	if err != nil {
		return err
	}
	t.emu = emu

	// Build claude command
	args := []string{}

	// Try to resume if we have a valid UUID session ID
	if sessionID != "" && isValidUUID(sessionID) {
		args = append(args, "--resume", sessionID)
	}

	cmd := exec.Command(t.ClaudePath, args...)
	cmd.Dir = t.WorkDir
	cmd.Env = append(cmd.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
	)

	// Start command in emulator
	if err := t.emu.StartCommand(cmd); err != nil {
		t.emu.Close()
		t.emu = nil
		return err
	}

	t.state = TerminalRunning

	// Monitor process exit
	go t.waitLoop()

	return nil
}

// waitLoop waits for process to exit
func (t *Terminal) waitLoop() {
	// Wait for terminal to finish
	for {
		t.mu.RLock()
		if t.emu == nil {
			t.mu.RUnlock()
			return
		}
		exited := t.emu.IsProcessExited()
		t.mu.RUnlock()

		if exited {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.mu.Lock()
	t.state = TerminalExited
	t.mu.Unlock()

	if t.onExit != nil {
		t.onExit()
	}
}

// Write sends input to the terminal
func (t *Terminal) Write(data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.emu == nil || t.state != TerminalRunning {
		return nil
	}

	_, err := t.emu.Write(data)
	return err
}

// WriteString sends a string to the terminal
func (t *Terminal) WriteString(s string) error {
	return t.Write([]byte(s))
}

// HandleKey processes a key press, returns true if terminal consumed it
// Returns false if ESC ESC was detected (exit terminal mode)
func (t *Terminal) HandleKey(key string) (consumed bool, exitTerminal bool) {
	// Check for ESC ESC (double escape within 300ms)
	if key == "esc" {
		now := time.Now()
		if now.Sub(t.lastEscTime) < 300*time.Millisecond {
			// Double ESC detected - exit terminal mode
			t.lastEscTime = time.Time{}
			return true, true
		}
		t.lastEscTime = now
		// Send single ESC to terminal after a short delay
		// (in case no second ESC comes)
		go func() {
			time.Sleep(350 * time.Millisecond)
			t.mu.RLock()
			lastEsc := t.lastEscTime
			t.mu.RUnlock()
			if !lastEsc.IsZero() && time.Since(lastEsc) >= 300*time.Millisecond {
				t.Write([]byte{0x1b}) // ESC
				t.mu.Lock()
				t.lastEscTime = time.Time{}
				t.mu.Unlock()
			}
		}()
		return true, false
	}

	// Reset ESC detection for other keys
	if !t.lastEscTime.IsZero() {
		// There was a pending ESC, send it first
		t.Write([]byte{0x1b})
		t.lastEscTime = time.Time{}
	}

	// Map key to bytes
	data := keyToBytes(key)
	if data != nil {
		t.Write(data)
	}

	return true, false
}

// keyToBytes converts a bubbletea key to bytes
func keyToBytes(key string) []byte {
	switch key {
	case "esc":
		return []byte{0x1b}
	case "enter":
		return []byte{'\r'}
	case "backspace":
		return []byte{0x7f}
	case "tab":
		return []byte{'\t'}
	case "space":
		return []byte{' '}
	case "up":
		return []byte{0x1b, '[', 'A'}
	case "down":
		return []byte{0x1b, '[', 'B'}
	case "right":
		return []byte{0x1b, '[', 'C'}
	case "left":
		return []byte{0x1b, '[', 'D'}
	case "home":
		return []byte{0x1b, '[', 'H'}
	case "end":
		return []byte{0x1b, '[', 'F'}
	case "pgup":
		return []byte{0x1b, '[', '5', '~'}
	case "pgdown":
		return []byte{0x1b, '[', '6', '~'}
	case "delete":
		return []byte{0x1b, '[', '3', '~'}
	case "ctrl+c":
		return []byte{0x03}
	case "ctrl+d":
		return []byte{0x04}
	case "ctrl+z":
		return []byte{0x1a}
	case "ctrl+l":
		return []byte{0x0c}
	case "ctrl+a":
		return []byte{0x01}
	case "ctrl+e":
		return []byte{0x05}
	case "ctrl+k":
		return []byte{0x0b}
	case "ctrl+u":
		return []byte{0x15}
	case "ctrl+w":
		return []byte{0x17}
	default:
		// Regular characters
		if len(key) == 1 {
			return []byte(key)
		}
		// Runes (UTF-8)
		if len(key) > 1 {
			return []byte(key)
		}
	}
	return nil
}

// View returns the current terminal view as a string with ANSI colors
func (t *Terminal) View() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.emu == nil {
		return "[No terminal initialized]"
	}

	// GetScreen returns frame with rows containing ANSI codes
	frame := t.emu.GetScreen()
	if len(frame.Rows) == 0 {
		return "[Empty screen]"
	}

	// Post-process each row
	var result []string
	for _, row := range frame.Rows {
		// Replace explicit white foreground with default foreground
		row = strings.ReplaceAll(row, "\x1b[37m", "\x1b[39m")
		// Replace explicit black background with default background
		row = strings.ReplaceAll(row, "\x1b[40m", "\x1b[49m")

		// Force truncate to terminal width
		row = truncateANSIString(row, t.width)

		// Reset at end of line
		row = row + "\x1b[0m"

		result = append(result, row)
	}

	// Join rows with newlines
	return strings.Join(result, "\n")
}

// truncateANSIString truncates a string with ANSI codes to visible width
func truncateANSIString(s string, maxWidth int) string {
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


// GetLines returns all lines from the terminal
func (t *Terminal) GetLines() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.emu == nil {
		return nil
	}
	frame := t.emu.GetScreen()
	return frame.Rows
}

// State returns the current terminal state
func (t *Terminal) State() TerminalState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// IsRunning returns true if the terminal is running
func (t *Terminal) IsRunning() bool {
	return t.State() == TerminalRunning
}

// Stop stops the terminal process
func (t *Terminal) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.emu != nil {
		t.emu.Close()
		t.emu = nil
	}

	t.state = TerminalExited
}

// SetCallbacks sets the callback functions
func (t *Terminal) SetCallbacks(onOutput, onExit func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onOutput = onOutput
	t.onExit = onExit
}

// ScrollUp scrolls the view up (no-op - emulator doesn't expose scrollback)
func (t *Terminal) ScrollUp(lines int) {
	// bubbleterm emulator doesn't expose scrollback buffer
}

// ScrollDown scrolls the view down (no-op - emulator doesn't expose scrollback)
func (t *Terminal) ScrollDown(lines int) {
	// bubbleterm emulator doesn't expose scrollback buffer
}

// ScrollToBottom scrolls to the bottom (no-op - emulator doesn't expose scrollback)
func (t *Terminal) ScrollToBottom() {
	// bubbleterm emulator doesn't expose scrollback buffer
}

// LineCount returns the total number of lines
func (t *Terminal) LineCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.height
}

// Width returns the terminal width
func (t *Terminal) Width() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.width
}

// Height returns the terminal height
func (t *Terminal) Height() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.height
}

// isValidUUID checks if a string is a valid UUID format
func isValidUUID(s string) bool {
	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
