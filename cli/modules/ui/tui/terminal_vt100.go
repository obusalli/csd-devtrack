package tui

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/muesli/termenv"
	"github.com/vito/vt100"
)

// TerminalVT100 represents an embedded terminal using vito/vt100
type TerminalVT100 struct {
	mu sync.RWMutex

	// Session info
	SessionID   string
	WorkDir     string
	ClaudePath  string
	SessionFile string

	// vt100 terminal
	vt *vt100.VT100

	// PTY
	ptmx *os.File
	cmd  *exec.Cmd

	// Terminal state
	width  int
	height int
	state  TerminalState

	// ESC ESC detection
	lastEscTime time.Time

	// Callbacks
	onOutput func()
	onExit   func()
}

// NewTerminalVT100 creates a new terminal using vito/vt100
func NewTerminalVT100(sessionID, workDir, claudePath string) *TerminalVT100 {
	return &TerminalVT100{
		SessionID:  sessionID,
		WorkDir:    workDir,
		ClaudePath: claudePath,
		width:      80,
		height:     24,
		state:      TerminalIdle,
	}
}

// SetSize sets the terminal size
func (t *TerminalVT100) SetSize(width, height int) {
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

	// Resize vt100 terminal
	if t.vt != nil {
		t.vt.Resize(height, width)
	}

	// Resize PTY
	if t.ptmx != nil {
		pty.Setsize(t.ptmx, &pty.Winsize{
			Rows: uint16(height),
			Cols: uint16(width),
		})
	}
}

// Start starts the process in the terminal
func (t *TerminalVT100) Start(sessionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TerminalRunning {
		return nil
	}

	// Create vt100 terminal
	t.vt = vt100.NewVT100(t.height, t.width)

	// Enable debug logging to see what's happening
	debugFile, _ := os.OpenFile("/tmp/vt100_debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if debugFile != nil {
		t.vt.DebugLogs = debugFile
	}

	// Build command
	args := []string{}
	if sessionID != "" && isValidUUID(sessionID) {
		args = append(args, "--resume", sessionID)
	}

	t.cmd = exec.Command(t.ClaudePath, args...)
	t.cmd.Dir = t.WorkDir
	t.cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
	)

	// Start with PTY
	var err error
	t.ptmx, err = pty.Start(t.cmd)
	if err != nil {
		return err
	}

	// Set initial size
	pty.Setsize(t.ptmx, &pty.Winsize{
		Rows: uint16(t.height),
		Cols: uint16(t.width),
	})

	t.state = TerminalRunning

	// Read loop
	go t.readLoop()

	// Wait for process exit
	go t.waitLoop()

	return nil
}

// readLoop reads from PTY and updates vt100
func (t *TerminalVT100) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Log error if needed
			}
			return
		}

		if n > 0 {
			t.mu.Lock()
			if t.vt != nil {
				t.vt.Write(buf[:n])
			}
			t.mu.Unlock()

			if t.onOutput != nil {
				t.onOutput()
			}
		}
	}
}

// waitLoop waits for process to exit
func (t *TerminalVT100) waitLoop() {
	if t.cmd != nil {
		t.cmd.Wait()
	}

	t.mu.Lock()
	t.state = TerminalExited
	t.mu.Unlock()

	if t.onExit != nil {
		t.onExit()
	}
}

// Write sends input to the terminal
func (t *TerminalVT100) Write(data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.ptmx == nil || t.state != TerminalRunning {
		return nil
	}

	_, err := t.ptmx.Write(data)
	return err
}

// WriteString sends a string to the terminal
func (t *TerminalVT100) WriteString(s string) error {
	return t.Write([]byte(s))
}

// HandleKey processes a key press
func (t *TerminalVT100) HandleKey(key string) (consumed bool, exitTerminal bool) {
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

// View returns the terminal view as a string with ANSI colors
func (t *TerminalVT100) View() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.vt == nil {
		return "[No terminal initialized]"
	}

	var lines []string
	for y := 0; y < t.vt.Height && y < len(t.vt.Content); y++ {
		var line strings.Builder
		var lastFormat vt100.Format
		hasFormat := false

		for x := 0; x < t.vt.Width && x < len(t.vt.Content[y]); x++ {
			r := t.vt.Content[y][x]
			f := t.vt.Format[y][x]

			// Check if format changed
			formatChanged := !hasFormat || !formatsEqual(f, lastFormat)
			if formatChanged {
				ansi := formatToANSI(f)
				if ansi != "" {
					line.WriteString(ansi)
				}
				lastFormat = f
				hasFormat = true
			}

			// Filter out control characters and convert nulls to spaces
			if r == 0 || r < 32 {
				line.WriteRune(' ')
			} else {
				line.WriteRune(r)
			}
		}
		line.WriteString("\x1b[0m") // Reset at end of line
		lines = append(lines, line.String())
	}

	return strings.Join(lines, "\n")
}

// formatsEqual compares two formats
func formatsEqual(a, b vt100.Format) bool {
	if a.Intensity != b.Intensity {
		return false
	}
	if a.Reverse != b.Reverse {
		return false
	}
	// Compare colors using their string representation
	if colorString(a.Fg) != colorString(b.Fg) {
		return false
	}
	if colorString(a.Bg) != colorString(b.Bg) {
		return false
	}
	return true
}

// colorString returns a string representation of a termenv.Color
func colorString(c termenv.Color) string {
	if c == nil {
		return ""
	}
	return c.Sequence(false)
}

// formatToANSI converts vt100.Format to ANSI escape sequence
func formatToANSI(f vt100.Format) string {
	var parts []string

	// Intensity
	switch f.Intensity {
	case vt100.Bold:
		parts = append(parts, "1")
	case vt100.Faint:
		parts = append(parts, "2")
	}

	// Reverse
	if f.Reverse {
		parts = append(parts, "7")
	}

	// Foreground color - use termenv's Sequence method
	if f.Fg != nil {
		parts = append(parts, f.Fg.Sequence(false))
	}

	// Background color
	if f.Bg != nil {
		parts = append(parts, f.Bg.Sequence(true))
	}

	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

// State returns the current terminal state
func (t *TerminalVT100) State() TerminalState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// IsRunning returns true if the terminal is running
func (t *TerminalVT100) IsRunning() bool {
	return t.State() == TerminalRunning
}

// Stop stops the terminal process
func (t *TerminalVT100) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx != nil {
		t.ptmx.Close()
		t.ptmx = nil
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	t.state = TerminalExited
}

// SetCallbacks sets the callback functions
func (t *TerminalVT100) SetCallbacks(onOutput, onExit func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onOutput = onOutput
	t.onExit = onExit
}

// LineCount returns the total number of lines
func (t *TerminalVT100) LineCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.vt != nil {
		return t.vt.UsedHeight()
	}
	return t.height
}

// Width returns the terminal width
func (t *TerminalVT100) Width() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.width
}

// Height returns the terminal height
func (t *TerminalVT100) Height() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.height
}

// GetLines returns all lines from the terminal
func (t *TerminalVT100) GetLines() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.vt == nil {
		return nil
	}
	var lines []string
	for y := 0; y < t.vt.Height && y < len(t.vt.Content); y++ {
		lines = append(lines, string(t.vt.Content[y]))
	}
	return lines
}
