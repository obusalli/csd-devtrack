package eventbus

import (
	"encoding/json"
	"sync"
	"time"
)

// EventType identifies the type of event
type EventType string

const (
	// State events
	EventProjectsUpdated  EventType = "projects_updated"
	EventProcessesUpdated EventType = "processes_updated"
	EventBuildsUpdated    EventType = "builds_updated"
	EventLogsUpdated      EventType = "logs_updated"
	EventGitUpdated       EventType = "git_updated"
	EventConfigUpdated    EventType = "config_updated"

	// Action events
	EventBuildStarted   EventType = "build_started"
	EventBuildFinished  EventType = "build_finished"
	EventProcessStarted EventType = "process_started"
	EventProcessStopped EventType = "process_stopped"
	EventProcessCrashed EventType = "process_crashed"

	// Notification events
	EventNotification EventType = "notification"

	// Log events
	EventLogLine EventType = "log_line"
)

// Event represents an event in the system
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NewEvent creates a new event
func NewEvent(eventType EventType) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}
}

// WithSource sets the source
func (e *Event) WithSource(source string) *Event {
	e.Source = source
	return e
}

// WithData adds data to the event
func (e *Event) WithData(key string, value interface{}) *Event {
	if e.Data == nil {
		e.Data = make(map[string]interface{})
	}
	e.Data[key] = value
	return e
}

// JSON returns the event as JSON
func (e *Event) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// Subscriber is a function that handles events
type Subscriber func(event *Event)

// Subscription represents a subscription to events
type Subscription struct {
	id         string
	eventTypes []EventType // nil means all events
	handler    Subscriber
}

// Bus is the central event bus
type Bus struct {
	mu            sync.RWMutex
	subscribers   map[string]*Subscription
	nextID        int
	eventHistory  []*Event
	historyLimit  int
}

// NewBus creates a new event bus
func NewBus() *Bus {
	return &Bus{
		subscribers:  make(map[string]*Subscription),
		eventHistory: make([]*Event, 0),
		historyLimit: 1000,
	}
}

// Subscribe registers a subscriber for specific event types
// Pass nil for eventTypes to subscribe to all events
func (b *Bus) Subscribe(eventTypes []EventType, handler Subscriber) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := string(rune('a'+b.nextID%26)) + string(rune('0'+b.nextID/26))

	b.subscribers[id] = &Subscription{
		id:         id,
		eventTypes: eventTypes,
		handler:    handler,
	}

	return id
}

// Unsubscribe removes a subscriber
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, id)
}

// Publish publishes an event to all matching subscribers
func (b *Bus) Publish(event *Event) {
	b.mu.RLock()
	subscribers := make([]*Subscription, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subscribers = append(subscribers, sub)
	}
	b.mu.RUnlock()

	// Store in history
	b.mu.Lock()
	b.eventHistory = append(b.eventHistory, event)
	if len(b.eventHistory) > b.historyLimit {
		b.eventHistory = b.eventHistory[1:]
	}
	b.mu.Unlock()

	// Notify subscribers
	for _, sub := range subscribers {
		if b.matchesSubscription(event, sub) {
			go sub.handler(event)
		}
	}
}

// matchesSubscription checks if an event matches a subscription
func (b *Bus) matchesSubscription(event *Event, sub *Subscription) bool {
	if sub.eventTypes == nil {
		return true
	}

	for _, et := range sub.eventTypes {
		if et == event.Type {
			return true
		}
	}
	return false
}

// GetHistory returns recent events
func (b *Bus) GetHistory(limit int) []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > len(b.eventHistory) {
		limit = len(b.eventHistory)
	}

	start := len(b.eventHistory) - limit
	result := make([]*Event, limit)
	copy(result, b.eventHistory[start:])
	return result
}

// GetHistoryByType returns recent events of specific types
func (b *Bus) GetHistoryByType(eventTypes []EventType, limit int) []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*Event, 0)
	typeSet := make(map[EventType]bool)
	for _, et := range eventTypes {
		typeSet[et] = true
	}

	for i := len(b.eventHistory) - 1; i >= 0 && len(result) < limit; i-- {
		if typeSet[b.eventHistory[i].Type] {
			result = append([]*Event{b.eventHistory[i]}, result...)
		}
	}

	return result
}

// Global event bus instance
var globalBus *Bus
var globalBusOnce sync.Once

// Global returns the global event bus
func Global() *Bus {
	globalBusOnce.Do(func() {
		globalBus = NewBus()
	})
	return globalBus
}
