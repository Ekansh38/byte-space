// engine/events.go
package engine

import "time"

type EventType string

const (
	// Flow events (journey of data)
	EventClientToEngine   EventType = "client→engine"
	EventEngineToTTY      EventType = "engine→tty"
	EventTTYToProgram     EventType = "tty→program"
	EventProgramToTTY     EventType = "program→tty"
	EventTTYToClient      EventType = "tty→client"
	
	// State change events
	EventTTYCreated        EventType = "tty_created"
	EventTTYModeChanged    EventType = "tty_mode_changed"
	EventForegroundChanged EventType = "foreground_changed"
	EventProgramStarted    EventType = "program_started"
	EventProgramExited     EventType = "program_exited"
	EventSessionCreated    EventType = "session_created"
)

type Event struct {
	Timestamp time.Time
	Type      EventType
	Data      map[string]interface{}
}

type EventBus struct {
	subscribers []chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make([]chan Event, 0),
	}
}

func (eb *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 100)
	eb.subscribers = append(eb.subscribers, ch)
	return ch
}

func (eb *EventBus) Publish(eventType EventType, data map[string]interface{}) {
	event := Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Data:      data,
	}
	
	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}
