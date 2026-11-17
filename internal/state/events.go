package state

import (
	"sync"
	"time"

	"painless/internal/domain"
)

// EventKind classifies state change notifications for the UI bridge.
type EventKind string

const (
	EventDrivesChanged     EventKind = "drives_changed"
	EventActiveDriveChange EventKind = "active_drive_changed"
	EventScreenChanged     EventKind = "screen_changed"
	EventSelectionChanged  EventKind = "selection_changed"
	EventMediaChanged      EventKind = "media_changed"
	EventStagingChanged    EventKind = "staging_changed"
	EventJobStarted        EventKind = "job_started"
	EventJobProgress       EventKind = "job_progress"
	EventJobFinished       EventKind = "job_finished"
	EventToast             EventKind = "toast"
)

// Toast is a transient notification payload for feedback.Toast.
type Toast struct {
	Level domain.LogLevel // info, warn, error
	Title string
	Body  string
}

// Event is a notification published by application state or background
// workers. Payloads are pointers and are nil when not applicable; they are
// treated as read-only by subscribers.
type Event struct {
	Kind    EventKind
	At      time.Time
	Drive   *domain.DriveEvent // EventDrivesChanged, EventActiveDriveChange
	DriveID string             // EventActiveDriveChange
	Job     *Job               // EventJobStarted, EventJobProgress, EventJobFinished
	Toast   *Toast             // EventToast
	Detail  string             // free-form context, e.g. changed album name
}

// Subscription receives events published on the bus. Events may be dropped
// when a subscriber falls behind; state reads remain the source of truth.
type Subscription struct {
	C   <-chan Event
	bus *Bus
	id  int64
}

// Cancel unsubscribes and releases the channel.
func (sub Subscription) Cancel() {
	sub.bus.unsubscribe(sub.id)
}

// Bus is a multi-subscriber event bus decoupling background workers from
// the UI. Publish never blocks: events are delivered by non-blocking send
// and may be coalesced or dropped under load.
type Bus struct {
	mu   sync.RWMutex
	subs map[int64]chan Event
	next int64
}

// NewBus creates an empty event bus.
func NewBus() *Bus {
	return &Bus{subs: map[int64]chan Event{}}
}

// Subscribe registers a new subscription with the given channel buffer.
// A buffer of at least 32 is recommended for UI bridges.
func (b *Bus) Subscribe(buffer int) Subscription {
	if buffer < 1 {
		buffer = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, buffer)
	b.subs[id] = ch
	return Subscription{C: ch, bus: b, id: id}
}

// Publish fans an event out to every subscriber without blocking. The At
// timestamp is filled in when zero.
func (b *Bus) Publish(ev Event) {
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Subscriber is behind; drop rather than block the worker.
		}
	}
}

// SubscriberCount reports how many active subscriptions exist.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func (b *Bus) unsubscribe(id int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		delete(b.subs, id)
		close(ch)
	}
}
