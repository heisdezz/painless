package thumbnail

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// Throttler suspends every active thumbnail subprocess while a video plays.
// New work waits at the gate until generation is resumed.
type Throttler struct {
	mu     sync.Mutex
	paused bool
	active map[int]*exec.Cmd
}

// NewThrottler returns an idle, running throttler.
func NewThrottler() *Throttler {
	return &Throttler{active: make(map[int]*exec.Cmd)}
}

// Pause suspends all registered subprocesses. It is idempotent.
func (t *Throttler) Pause() {
	t.mu.Lock()
	if t.paused {
		t.mu.Unlock()
		return
	}
	t.paused = true
	commands := make([]*exec.Cmd, 0, len(t.active))
	for _, cmd := range t.active {
		commands = append(commands, cmd)
	}
	t.mu.Unlock()
	for _, cmd := range commands {
		_ = pauseProcess(cmd)
	}
}

// Resume continues all suspended subprocesses and lets queued work start.
func (t *Throttler) Resume() {
	t.mu.Lock()
	if !t.paused {
		t.mu.Unlock()
		return
	}
	t.paused = false
	commands := make([]*exec.Cmd, 0, len(t.active))
	for _, cmd := range t.active {
		commands = append(commands, cmd)
	}
	t.mu.Unlock()
	for _, cmd := range commands {
		_ = resumeProcess(cmd)
	}
}

// Paused reports whether generation is currently suspended.
func (t *Throttler) Paused() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.paused
}

// Wait blocks while paused, returning early if ctx is cancelled.
func (t *Throttler) Wait(ctx context.Context) error {
	for {
		t.mu.Lock()
		paused := t.paused
		t.mu.Unlock()
		if !paused {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// register records a started command and suspends it immediately when a
// pause has already been requested. The returned function unregisters it.
func (t *Throttler) register(cmd *exec.Cmd) func() {
	if cmd == nil || cmd.Process == nil {
		return func() {}
	}
	pid := cmd.Process.Pid
	t.mu.Lock()
	t.active[pid] = cmd
	paused := t.paused
	t.mu.Unlock()
	if paused {
		_ = pauseProcess(cmd)
	}
	return func() {
		t.mu.Lock()
		delete(t.active, pid)
		t.mu.Unlock()
	}
}
