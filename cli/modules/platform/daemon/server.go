package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"csd-devtrack/cli/modules"
	"csd-devtrack/cli/modules/ui/core"
)

// Server is the daemon server that manages state and processes
type Server struct {
	socketPath string
	listener   net.Listener
	presenter  core.Presenter

	// Client management (only one client at a time)
	clientMu   sync.Mutex
	client     net.Conn
	clientDone chan struct{}

	// TUI state persistence (for reattach)
	tuiState *TUIState

	// Log buffer for sending to newly connected clients
	logBuffer     []core.LogLineVM
	logBufferSize int

	// Shutdown
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewServer creates a new daemon server
func NewServer(presenter core.Presenter) *Server {
	return &Server{
		socketPath:    GetSocketPath(),
		presenter:     presenter,
		done:          make(chan struct{}),
		logBuffer:     make([]core.LogLineVM, 0, 1000),
		logBufferSize: 1000, // Keep last 1000 log lines
	}
}


// instanceName is the current daemon instance name (empty = default)
var instanceName string

// SetInstanceName sets the daemon instance name for multi-instance support
func SetInstanceName(name string) {
	instanceName = name
}

// GetInstanceName returns the current daemon instance name
func GetInstanceName() string {
	return instanceName
}

// GetSocketPath returns the platform-specific socket path
func GetSocketPath() string {
	return GetSocketPathForInstance(instanceName)
}

// ValidateInstanceName validates that instance name contains only allowed characters
func ValidateInstanceName(name string) error {
	if name == "" {
		return nil
	}
	for _, c := range name {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return fmt.Errorf("invalid instance name: only A-Z, a-z, 0-9, _, - are allowed")
		}
	}
	return nil
}

// GetSocketPathForInstance returns the socket path for a specific instance
func GetSocketPathForInstance(name string) string {
	prefix := ""
	if name != "" {
		prefix = name + "."
	}

	if runtime.GOOS == "windows" {
		return `\\.\pipe\` + prefix + `csd-devtrack`
	}

	// Unix socket in user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".csd-devtrack")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, prefix+"socket")
}

// GetPIDPath returns the path to the PID file
func GetPIDPath() string {
	return GetPIDPathForInstance(instanceName)
}

// GetPIDPathForInstance returns the PID path for a specific instance
func GetPIDPathForInstance(name string) string {
	prefix := ""
	if name != "" {
		prefix = name + "."
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".csd-devtrack")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, prefix+"daemon.pid")
}

// ListInstances returns a list of all running daemon instances (sorted)
func ListInstances() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".csd-devtrack")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	hasDefault := false
	var named []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match *.daemon.pid or daemon.pid files
		if strings.HasSuffix(name, ".daemon.pid") && name != "daemon.pid" {
			// Named instance: NAME.daemon.pid
			instanceName := strings.TrimSuffix(name, ".daemon.pid")
			named = append(named, instanceName)
		} else if name == "daemon.pid" {
			hasDefault = true
		}
	}

	// Sort named instances alphabetically
	sort.Strings(named)

	// Put default first, then named instances
	var instances []string
	if hasDefault {
		instances = append(instances, "(default)")
	}
	instances = append(instances, named...)

	return instances
}

// Start starts the daemon server
func (s *Server) Start() error {
	// Clean up any stale files from previous crashed daemon
	// IsRunning() will auto-cleanup if daemon isn't actually running
	if IsRunning() {
		return fmt.Errorf("daemon already running (PID %d)", GetServerPID())
	}

	// Force remove stale socket (we just verified no daemon is running)
	if runtime.GOOS != "windows" {
		os.Remove(s.socketPath)
	} else {
		os.Remove(s.socketPath + ".addr")
	}

	// Create listener
	var err error
	if runtime.GOOS == "windows" {
		// Windows named pipe
		s.listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("failed to create listener: %w", err)
		}
		// Write the port to a file for clients to find
		addr := s.listener.Addr().String()
		os.WriteFile(s.socketPath+".addr", []byte(addr), 0600)
	} else {
		// Unix socket
		s.listener, err = net.Listen("unix", s.socketPath)
		if err != nil {
			return fmt.Errorf("failed to create socket: %w", err)
		}
		os.Chmod(s.socketPath, 0600)
	}

	// Write PID file
	if err := os.WriteFile(GetPIDPath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		s.listener.Close()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the daemon server
func (s *Server) Stop() {
	close(s.done)

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close current client
	s.clientMu.Lock()
	if s.client != nil {
		s.client.Close()
	}
	s.clientMu.Unlock()

	// Wait for goroutines
	s.wg.Wait()

	// Cleanup
	os.Remove(s.socketPath)
	os.Remove(GetPIDPath())
	if runtime.GOOS == "windows" {
		os.Remove(s.socketPath + ".addr")
	}
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}

		// Only allow one client at a time
		s.clientMu.Lock()
		if s.client != nil {
			// Disconnect previous client
			s.client.Close()
			if s.clientDone != nil {
				<-s.clientDone // Wait for previous client handler to finish
			}
		}
		s.client = conn
		s.clientDone = make(chan struct{})
		s.clientMu.Unlock()

		s.wg.Add(1)
		go s.handleClient(conn, s.clientDone)
	}
}

// handleClient handles a connected client
func (s *Server) handleClient(conn net.Conn, done chan struct{}) {
	defer s.wg.Done()
	defer close(done)
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Handle messages from client
	// Note: Initial state is sent after receiving handshake (not immediately on connect)
	// This avoids issues with connectivity-check connections that close immediately
	for {
		select {
		case <-s.done:
			return
		default:
		}

		// Set read deadline for periodic checks
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return // Client disconnected
		}

		msg, err := DecodeMessage(line)
		if err != nil {
			continue
		}

		s.handleMessage(conn, msg)
	}
}

// handleMessage processes a message from the client
func (s *Server) handleMessage(conn net.Conn, msg *Message) {
	switch msg.Type {
	case MsgEvent:
		var payload EventPayload
		if err := msg.Decode(&payload); err != nil {
			s.sendError(conn, "invalid event payload")
			return
		}
		if payload.Event != nil && s.presenter != nil {
			s.presenter.HandleEvent(payload.Event)
		}

	case MsgGetState:
		s.sendStateOnly(conn) // Don't send TUI state on refresh, only on initial connect

	case MsgPing:
		s.sendPong(conn)

	case MsgSubscribe:
		// Already subscribed by connecting
		s.sendState(conn)

	case MsgSaveTUIState:
		// Client is detaching - save TUI state for next reattach
		var payload TUIStatePayload
		if err := msg.Decode(&payload); err != nil {
			s.sendError(conn, "invalid TUI state payload")
			return
		}
		s.clientMu.Lock()
		s.tuiState = payload.TUIState
		s.clientMu.Unlock()

	case MsgHandshake:
		// Client is sending its version info - this confirms it's a real client
		var payload HandshakePayload
		if err := msg.Decode(&payload); err != nil {
			s.sendError(conn, "invalid handshake payload")
			return
		}
		s.sendHandshakeResp(conn, payload.BuildHash)
		// Send initial state after handshake (real client, not just a connectivity check)
		s.sendState(conn)
	}
}

// sendHandshakeResp sends a handshake response to the client
func (s *Server) sendHandshakeResp(conn net.Conn, clientHash string) {
	serverHash := modules.BuildHash()
	compatible := clientHash == serverHash
	restartHint := !compatible && clientHash != "000000-dev00000" && serverHash != "000000-dev00000"

	payload := HandshakeRespPayload{
		BuildHash:   serverHash,
		Version:     modules.AppVersion,
		Compatible:  compatible,
		RestartHint: restartHint,
	}
	msg, err := NewMessage(MsgHandshakeResp, payload)
	if err != nil {
		return
	}

	data, err := msg.Encode()
	if err != nil {
		return
	}

	conn.Write(data)
}

// sendStateOnly sends just the app state without TUI state (for refresh)
func (s *Server) sendStateOnly(conn net.Conn) {
	if s.presenter == nil {
		return
	}

	// Skip refresh if still initializing (avoids deadlock)
	// Just send current state - it will be updated after init completes
	state := s.presenter.GetState()
	if state != nil && !state.Initializing {
		_ = s.presenter.Refresh()
		// Re-get state after refresh
		state = s.presenter.GetState()
	}

	if state == nil {
		return
	}

	payload := StatePayload{State: state}
	msg, err := NewMessage(MsgState, payload)
	if err != nil {
		return
	}

	data, err := msg.Encode()
	if err != nil {
		return
	}

	conn.Write(data)
}

// sendState sends the full state including TUI state (for initial connect)
func (s *Server) sendState(conn net.Conn) {
	s.sendStateOnly(conn)

	// Send buffered logs to newly connected client
	s.sendBufferedLogs(conn)

	// Also send saved TUI state if available (for reattach)
	s.clientMu.Lock()
	tuiState := s.tuiState
	s.tuiState = nil // Clear after sending - only restore once
	s.clientMu.Unlock()

	if tuiState != nil {
		s.sendTUIState(conn, tuiState)
	}
}

// sendBufferedLogs sends all buffered logs to a client
func (s *Server) sendBufferedLogs(conn net.Conn) {
	s.clientMu.Lock()
	logs := make([]core.LogLineVM, len(s.logBuffer))
	copy(logs, s.logBuffer)
	s.clientMu.Unlock()

	for _, line := range logs {
		payload := LogPayload{Line: line}
		msg, err := NewMessage(MsgLog, payload)
		if err != nil {
			continue
		}

		data, err := msg.Encode()
		if err != nil {
			continue
		}

		conn.Write(data)
	}
}

// sendTUIState sends the saved TUI state to a client
func (s *Server) sendTUIState(conn net.Conn, state *TUIState) {
	payload := TUIStatePayload{TUIState: state}
	msg, err := NewMessage(MsgTUIState, payload)
	if err != nil {
		return
	}

	data, err := msg.Encode()
	if err != nil {
		return
	}

	conn.Write(data)
}

// sendPong sends a pong response
func (s *Server) sendPong(conn net.Conn) {
	msg, _ := NewMessage(MsgPong, nil)
	data, _ := msg.Encode()
	conn.Write(data)
}

// sendError sends an error message
func (s *Server) sendError(conn net.Conn, message string) {
	payload := ErrorPayload{Message: message}
	msg, _ := NewMessage(MsgError, payload)
	data, _ := msg.Encode()
	conn.Write(data)
}

// BroadcastState sends state update to connected client
func (s *Server) BroadcastState(state *core.AppState) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.client == nil {
		return
	}

	payload := StatePayload{State: state}
	msg, err := NewMessage(MsgState, payload)
	if err != nil {
		return
	}

	data, err := msg.Encode()
	if err != nil {
		return
	}

	s.client.Write(data)
}

// BroadcastLog sends a log line to connected client
func (s *Server) BroadcastLog(line core.LogLineVM) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	// Always store in buffer (ring buffer behavior)
	if len(s.logBuffer) >= s.logBufferSize {
		// Remove oldest entry
		s.logBuffer = s.logBuffer[1:]
	}
	s.logBuffer = append(s.logBuffer, line)

	// Send to client if connected
	if s.client == nil {
		return
	}

	payload := LogPayload{Line: line}
	msg, err := NewMessage(MsgLog, payload)
	if err != nil {
		return
	}

	data, err := msg.Encode()
	if err != nil {
		return
	}

	s.client.Write(data)
}

// BroadcastNotification sends a notification to connected client
func (s *Server) BroadcastNotification(notification *core.Notification) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.client == nil {
		return
	}

	payload := NotifyPayload{Notification: notification}
	msg, err := NewMessage(MsgNotify, payload)
	if err != nil {
		return
	}

	data, err := msg.Encode()
	if err != nil {
		return
	}

	s.client.Write(data)
}

// HasClient returns true if a client is connected
func (s *Server) HasClient() bool {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	return s.client != nil
}

// IsRunning checks if a daemon server is already running
// Also cleans up stale socket/pid files if the daemon crashed
func IsRunning() bool {
	pidPath := GetPIDPath()
	socketPath := GetSocketPath()

	data, err := os.ReadFile(pidPath)
	if err != nil {
		// No PID file, clean up any stale socket
		cleanupStaleSocket(socketPath)
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		cleanupStaleFiles(pidPath, socketPath)
		return false
	}

	// Check if process exists
	if !isProcessRunning(pid) {
		cleanupStaleFiles(pidPath, socketPath)
		return false
	}

	// Process exists, but verify socket is actually connectable
	// (process might be zombie or socket might be stale)
	if !isSocketConnectable(socketPath) {
		cleanupStaleFiles(pidPath, socketPath)
		return false
	}

	return true
}

// isProcessRunning checks if a process with given PID is running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if runtime.GOOS == "windows" {
		// On Windows, FindProcess succeeds only if process exists
		return true
	}

	// On Unix, send signal 0 to check if process exists
	// Signal 0 doesn't actually send a signal, just checks
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isSocketConnectable checks if the socket can be connected to
func isSocketConnectable(socketPath string) bool {
	if runtime.GOOS == "windows" {
		addrData, err := os.ReadFile(socketPath + ".addr")
		if err != nil {
			return false
		}
		conn, err := net.DialTimeout("tcp", string(addrData), 500*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}

	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// cleanupStaleSocket removes a stale socket file
func cleanupStaleSocket(socketPath string) {
	if runtime.GOOS != "windows" {
		// Only owner should be able to remove
		info, err := os.Stat(socketPath)
		if err != nil {
			return
		}
		// Check if we own the socket
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if stat.Uid != uint32(os.Getuid()) {
				return // Not our socket, don't touch
			}
		}
		os.Remove(socketPath)
	} else {
		os.Remove(socketPath + ".addr")
	}
}

// cleanupStaleFiles removes stale PID and socket files
func cleanupStaleFiles(pidPath, socketPath string) {
	// Verify we own these files before removing
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(pidPath); err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				if stat.Uid != uint32(os.Getuid()) {
					return // Not our files
				}
			}
		}
	}
	os.Remove(pidPath)
	cleanupStaleSocket(socketPath)
}

// Wipe forces cleanup of all daemon files (socket, pid)
// Use this when files are orphaned and daemon isn't running
func Wipe() error {
	pidPath := GetPIDPath()
	socketPath := GetSocketPath()

	// First check if daemon is actually running
	if IsRunning() {
		return fmt.Errorf("daemon is running (PID %d), stop it first", GetServerPID())
	}

	// Force remove all files
	var errors []string

	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("pid file: %v", err))
	}

	if runtime.GOOS == "windows" {
		if err := os.Remove(socketPath + ".addr"); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("addr file: %v", err))
		}
	} else {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("socket: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("wipe errors: %v", errors)
	}

	return nil
}

// GetServerPID returns the PID of the running daemon, or 0 if not running
func GetServerPID() int {
	pidPath := GetPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0
	}

	return pid
}

