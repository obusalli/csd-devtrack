package daemon

import (
	"encoding/json"
	"time"

	"csd-devtrack/cli/modules/ui/core"
)

// MessageType identifies the type of message
type MessageType string

const (
	// Client -> Server
	MsgEvent        MessageType = "event"          // UI event (build, run, stop, etc.)
	MsgSubscribe    MessageType = "subscribe"      // Subscribe to state updates
	MsgGetState     MessageType = "get_state"      // Request full state
	MsgPing         MessageType = "ping"           // Keepalive
	MsgSaveTUIState MessageType = "save_tui_state" // Save TUI state on detach
	MsgHandshake    MessageType = "handshake"      // Version handshake

	// Server -> Client
	MsgState         MessageType = "state"          // Full state update
	MsgStateUpdate   MessageType = "state_update"   // Partial state update
	MsgLog           MessageType = "log"            // Log line
	MsgNotify        MessageType = "notify"         // Notification
	MsgPong          MessageType = "pong"           // Keepalive response
	MsgError         MessageType = "error"          // Error response
	MsgTUIState      MessageType = "tui_state"      // Saved TUI state on reconnect
	MsgHandshakeResp MessageType = "handshake_resp" // Version handshake response
)

// Message is the envelope for all daemon messages
type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	var data json.RawMessage
	if payload != nil {
		var err error
		data, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	return &Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   data,
	}, nil
}

// Decode decodes the payload into the given target
func (m *Message) Decode(target interface{}) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, target)
}

// EventPayload wraps a UI event for transmission
type EventPayload struct {
	Event *core.Event `json:"event"`
}

// StatePayload contains the full application state
type StatePayload struct {
	State *core.AppState `json:"state"`
}

// StateUpdatePayload contains a partial state update
type StateUpdatePayload struct {
	ViewType  core.ViewModelType `json:"view_type"`
	ViewModel interface{}        `json:"view_model"`
}

// LogPayload contains a log line
type LogPayload struct {
	Line core.LogLineVM `json:"line"`
}

// NotifyPayload contains a notification
type NotifyPayload struct {
	Notification *core.Notification `json:"notification"`
}

// ErrorPayload contains an error message
type ErrorPayload struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// SubscribePayload contains subscription options
type SubscribePayload struct {
	Views []core.ViewModelType `json:"views,omitempty"` // Empty = all views
}

// HandshakePayload contains client version information
type HandshakePayload struct {
	BuildHash string `json:"build_hash"` // 8-char build hash
	Version   string `json:"version"`    // Semantic version (e.g., "0.1.0")
}

// HandshakeRespPayload contains server version information
type HandshakeRespPayload struct {
	BuildHash   string `json:"build_hash"`    // Server's build hash
	Version     string `json:"version"`       // Server's version
	Compatible  bool   `json:"compatible"`    // True if client is compatible
	RestartHint bool   `json:"restart_hint"`  // True if daemon should be restarted
}

// Encode serializes a message to JSON with newline delimiter
func (m *Message) Encode() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	// Add newline as message delimiter
	return append(data, '\n'), nil
}

// DecodeMessage deserializes a message from JSON
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
