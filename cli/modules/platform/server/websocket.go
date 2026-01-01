package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"csd-devtrack/cli/modules/platform/eventbus"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		return true
	},
}

// WSClient represents a WebSocket client
type WSClient struct {
	id       string
	conn     *websocket.Conn
	send     chan []byte
	hub      *WSHub
	subID    string
	filters  []eventbus.EventType
	mu       sync.Mutex
}

// WSHub manages all WebSocket connections
type WSHub struct {
	clients    map[string]*WSClient
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan []byte
	bus        *eventbus.Bus
	mu         sync.RWMutex
	nextID     int
}

// NewWSHub creates a new WebSocket hub
func NewWSHub(bus *eventbus.Bus) *WSHub {
	return &WSHub{
		clients:    make(map[string]*WSClient),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan []byte, 256),
		bus:        bus,
	}
}

// Run starts the hub
func (h *WSHub) Run() {
	// Subscribe to all events
	h.bus.Subscribe(nil, func(event *eventbus.Event) {
		data, err := event.JSON()
		if err != nil {
			return
		}
		h.broadcastEvent(data)
	})

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.id] = client
			h.mu.Unlock()
			log.Printf("WebSocket client connected: %s", client.id)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.id]; ok {
				delete(h.clients, client.id)
				close(client.send)
				log.Printf("WebSocket client disconnected: %s", client.id)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// broadcastEvent sends an event to all connected clients
func (h *WSHub) broadcastEvent(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.send <- data:
		default:
			// Client buffer full, skip
		}
	}
}

// ClientCount returns the number of connected clients
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS handles WebSocket connections
func (h *WSHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	h.mu.Lock()
	h.nextID++
	clientID := "ws-" + string(rune('a'+h.nextID%26)) + string(rune('0'+h.nextID/26))
	h.mu.Unlock()

	client := &WSClient{
		id:   clientID,
		conn: conn,
		send: make(chan []byte, 256),
		hub:  h,
	}

	h.register <- client

	// Start read/write goroutines
	go client.writePump()
	go client.readPump()
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages (commands from client)
		c.handleMessage(message)
	}
}

// ClientMessage represents a message from a WebSocket client
type ClientMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// handleMessage handles messages from clients
func (c *WSClient) handleMessage(message []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "subscribe":
		// Client wants to subscribe to specific events
		if types, ok := msg.Payload["types"].([]interface{}); ok {
			c.mu.Lock()
			c.filters = make([]eventbus.EventType, len(types))
			for i, t := range types {
				c.filters[i] = eventbus.EventType(t.(string))
			}
			c.mu.Unlock()
		}

	case "ping":
		// Respond with pong
		response := map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(response)
		c.send <- data

	case "get_history":
		// Send recent event history
		limit := 100
		if l, ok := msg.Payload["limit"].(float64); ok {
			limit = int(l)
		}
		events := c.hub.bus.GetHistory(limit)
		response := map[string]interface{}{
			"type":   "history",
			"events": events,
		}
		data, _ := json.Marshal(response)
		c.send <- data
	}
}
