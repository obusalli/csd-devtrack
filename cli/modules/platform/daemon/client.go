package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"csd-devtrack/cli/modules"
	"csd-devtrack/cli/modules/ui/core"
)

// Client connects to the daemon server
type Client struct {
	conn   net.Conn
	reader *bufio.Reader

	// Callbacks
	onState          func(*core.AppState)
	onLog            func(core.LogLineVM)
	onNotify         func(*core.Notification)
	onDisconnect     func()
	onTUIState       func(*TUIState) // Called when saved TUI state is received on reattach
	onVersionMismatch func(serverVersion, serverHash string) // Called when versions don't match

	// Server version info (populated after handshake)
	serverVersion string
	serverHash    string

	// Buffered messages (received before handlers are set)
	pendingLogs  []core.LogLineVM
	pendingState *core.AppState

	// State
	mu        sync.Mutex
	connected bool
	done      chan struct{}
	wg        sync.WaitGroup
}

// NewClient creates a new daemon client
func NewClient() *Client {
	return &Client{
		done: make(chan struct{}),
	}
}

// Connect connects to the daemon server
func (c *Client) Connect() error {
	socketPath := GetSocketPath()

	var conn net.Conn
	var err error

	if runtime.GOOS == "windows" {
		// Read TCP address from file
		addrData, err := os.ReadFile(socketPath + ".addr")
		if err != nil {
			return fmt.Errorf("daemon not running: %w", err)
		}
		conn, err = net.DialTimeout("tcp", string(addrData), 5*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}
	} else {
		conn, err = net.DialTimeout("unix", socketPath, 5*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}
	}

	c.mu.Lock()
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.connected = true
	c.mu.Unlock()

	// Start message receiver
	c.wg.Add(1)
	go c.receiveLoop()

	// Send handshake with our version
	if err := c.sendHandshake(); err != nil {
		// Non-fatal, just log and continue
	}

	return nil
}

// sendHandshake sends version handshake to the server
func (c *Client) sendHandshake() error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	payload := HandshakePayload{
		BuildHash: modules.BuildHash(),
		Version:   modules.AppVersion,
	}
	msg, err := NewMessage(MsgHandshake, payload)
	if err != nil {
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// GetServerVersion returns the server's version and build hash (after handshake)
func (c *Client) GetServerVersion() (version, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverVersion, c.serverHash
}

// SetVersionMismatchHandler sets the callback for version mismatch
func (c *Client) SetVersionMismatchHandler(handler func(serverVersion, serverHash string)) {
	c.mu.Lock()
	c.onVersionMismatch = handler
	c.mu.Unlock()
}

// Disconnect disconnects from the daemon (detach)
func (c *Client) Disconnect() {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return
	}
	c.connected = false
	c.mu.Unlock()

	close(c.done)

	if c.conn != nil {
		c.conn.Close()
	}

	c.wg.Wait()
}

// IsConnected returns true if connected to daemon
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// SetStateHandler sets the callback for state updates
// Also flushes any pending state that was received before the handler was set
func (c *Client) SetStateHandler(handler func(*core.AppState)) {
	c.mu.Lock()
	c.onState = handler
	pending := c.pendingState
	c.pendingState = nil
	c.mu.Unlock()

	// Flush pending state
	if handler != nil && pending != nil {
		handler(pending)
	}
}

// SetLogHandler sets the callback for log lines
// Also flushes any pending logs that were received before the handler was set
func (c *Client) SetLogHandler(handler func(core.LogLineVM)) {
	c.mu.Lock()
	c.onLog = handler
	pending := c.pendingLogs
	c.pendingLogs = nil
	c.mu.Unlock()

	// Flush pending logs
	if handler != nil {
		for _, line := range pending {
			handler(line)
		}
	}
}

// SetNotifyHandler sets the callback for notifications
func (c *Client) SetNotifyHandler(handler func(*core.Notification)) {
	c.mu.Lock()
	c.onNotify = handler
	c.mu.Unlock()
}

// SetDisconnectHandler sets the callback for disconnect events
func (c *Client) SetDisconnectHandler(handler func()) {
	c.mu.Lock()
	c.onDisconnect = handler
	c.mu.Unlock()
}

// SetTUIStateHandler sets the callback for TUI state restoration on reattach
func (c *Client) SetTUIStateHandler(handler func(*TUIState)) {
	c.mu.Lock()
	c.onTUIState = handler
	c.mu.Unlock()
}

// SaveTUIState saves the TUI state to the daemon before detaching
func (c *Client) SaveTUIState(state *TUIState) error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	payload := TUIStatePayload{TUIState: state}
	msg, err := NewMessage(MsgSaveTUIState, payload)
	if err != nil {
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// SendEvent sends an event to the daemon
func (c *Client) SendEvent(event *core.Event) error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	payload := EventPayload{Event: event}
	msg, err := NewMessage(MsgEvent, payload)
	if err != nil {
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// RequestState requests the full state from daemon
func (c *Client) RequestState() error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	msg, err := NewMessage(MsgGetState, nil)
	if err != nil {
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// Ping sends a ping to the daemon
func (c *Client) Ping() error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	msg, err := NewMessage(MsgPing, nil)
	if err != nil {
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// receiveLoop receives messages from the daemon
func (c *Client) receiveLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		if !c.connected || c.conn == nil {
			c.mu.Unlock()
			return
		}
		conn := c.conn
		reader := c.reader
		c.mu.Unlock()

		// Set read deadline for periodic checks
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			// Connection lost
			c.mu.Lock()
			c.connected = false
			handler := c.onDisconnect
			c.mu.Unlock()

			if handler != nil {
				handler()
			}
			return
		}

		msg, err := DecodeMessage(line)
		if err != nil {
			continue
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes a message from the daemon
func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case MsgState:
		var payload StatePayload
		if err := msg.Decode(&payload); err != nil {
			return
		}
		c.mu.Lock()
		handler := c.onState
		if handler == nil && payload.State != nil {
			// Buffer state until handler is set
			c.pendingState = payload.State
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
		if handler != nil && payload.State != nil {
			handler(payload.State)
		}

	case MsgLog:
		var payload LogPayload
		if err := msg.Decode(&payload); err != nil {
			return
		}
		c.mu.Lock()
		handler := c.onLog
		if handler == nil {
			// Buffer logs until handler is set
			c.pendingLogs = append(c.pendingLogs, payload.Line)
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
		handler(payload.Line)

	case MsgNotify:
		var payload NotifyPayload
		if err := msg.Decode(&payload); err != nil {
			return
		}
		c.mu.Lock()
		handler := c.onNotify
		c.mu.Unlock()
		if handler != nil && payload.Notification != nil {
			handler(payload.Notification)
		}

	case MsgPong:
		// Keepalive response, ignore

	case MsgError:
		var payload ErrorPayload
		if err := msg.Decode(&payload); err != nil {
			return
		}
		// Could log this or notify

	case MsgTUIState:
		var payload TUIStatePayload
		if err := msg.Decode(&payload); err != nil {
			return
		}
		c.mu.Lock()
		handler := c.onTUIState
		c.mu.Unlock()
		if handler != nil && payload.TUIState != nil {
			handler(payload.TUIState)
		}

	case MsgHandshakeResp:
		var payload HandshakeRespPayload
		if err := msg.Decode(&payload); err != nil {
			return
		}
		c.mu.Lock()
		c.serverVersion = payload.Version
		c.serverHash = payload.BuildHash
		handler := c.onVersionMismatch
		c.mu.Unlock()

		// Check for version mismatch
		if !payload.Compatible && handler != nil {
			handler(payload.Version, payload.BuildHash)
		}
	}
}
