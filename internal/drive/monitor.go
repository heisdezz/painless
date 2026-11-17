package drive

import (
	"context"
	"sync"
	"time"

	"painless/internal/domain"
)

// DefaultPollInterval is the default duration between disk enumeration checks.
const DefaultPollInterval = 3 * time.Second

// Monitor watches for drive mount and unmount events.
type Monitor struct {
	svc          *Service
	pollInterval time.Duration
	events       chan domain.DriveEvent
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	knownDrives  map[string]domain.DriveInfo
	mu           sync.Mutex
}

// NewMonitor creates a Monitor.
func NewMonitor(svc *Service, interval time.Duration) *Monitor {
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	return &Monitor{
		svc:          svc,
		pollInterval: interval,
		events:       make(chan domain.DriveEvent, 32),
		knownDrives:  make(map[string]domain.DriveInfo),
	}
}

// Events returns the channel receiving drive insertion and removal events.
func (m *Monitor) Events() <-chan domain.DriveEvent {
	return m.events
}

// Start initiates background monitoring until the context is cancelled.
func (m *Monitor) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	m.wg.Add(1)

	// Seed initial drives
	if initial, err := m.svc.ListDrives(ctx); err == nil {
		m.mu.Lock()
		for _, d := range initial {
			m.knownDrives[d.ID] = d
		}
		m.mu.Unlock()
	}

	go m.run(ctx)
}

// Stop shuts down the background monitor.
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	close(m.events)
}

func (m *Monitor) run(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

func (m *Monitor) poll(ctx context.Context) {
	current, err := m.svc.ListDrives(ctx)
	if err != nil {
		return
	}

	currentMap := make(map[string]domain.DriveInfo, len(current))
	for _, d := range current {
		currentMap[d.ID] = d
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Detect new drives (mounted)
	for id, drive := range currentMap {
		if _, existed := m.knownDrives[id]; !existed {
			m.emit(domain.DriveEvent{
				Type:      domain.DriveEventMounted,
				Drive:     drive,
				Timestamp: time.Now().UTC(),
			})
		}
	}

	// Detect removed drives (unmounted)
	for id, drive := range m.knownDrives {
		if _, stillPresent := currentMap[id]; !stillPresent {
			unmounted := drive
			unmounted.Status = domain.DriveStatusUnmounted
			m.emit(domain.DriveEvent{
				Type:      domain.DriveEventUnmounted,
				Drive:     unmounted,
				Timestamp: time.Now().UTC(),
			})
		}
	}

	m.knownDrives = currentMap
}

func (m *Monitor) emit(event domain.DriveEvent) {
	select {
	case m.events <- event:
	default:
		// Drop if buffer full to avoid blocking monitoring
	}
}
