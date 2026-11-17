// Package logging provides the centralized in-memory logging engine: a
// bounded ring buffer of structured entries with level filtering, live
// change notification, query support for the diagnostics console, and
// text export. Entries live only for the process lifetime; persistence
// is the user's export job.
package logging

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"painless/internal/domain"
)

// DefaultCapacity is the ring buffer size used when Options.Capacity is
// not set: generous enough for long scan sessions, small enough that the
// console stays snappy.
const DefaultCapacity = 2000

// levelRank orders severities for minimum-level filtering.
var levelRank = map[domain.LogLevel]int{
	domain.LogLevelInfo:  0,
	domain.LogLevelWarn:  1,
	domain.LogLevelError: 2,
}

// Options configures a Logger.
type Options struct {
	// Capacity bounds the ring buffer; zero selects DefaultCapacity.
	Capacity int
	// MinLevel drops entries below this severity. Empty accepts all.
	MinLevel domain.LogLevel
	// Notify, when set, runs after each accepted append. App wiring uses
	// it to request a UI frame; it must be cheap and non-blocking.
	Notify func()
}

// Logger is a concurrency-safe ring buffer of log entries.
type Logger struct {
	mu      sync.RWMutex
	ring    []domain.LogEntry
	head    int // index of the oldest entry
	count   int
	minRank int
	notify  func()
	dropped int64
	nowFunc func() time.Time
}

// New creates an empty logger.
func New(opts Options) *Logger {
	capacity := opts.Capacity
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	minRank, ok := levelRank[opts.MinLevel]
	if !ok {
		minRank = 0
	}
	return &Logger{
		ring:    make([]domain.LogEntry, capacity),
		minRank: minRank,
		notify:  opts.Notify,
		nowFunc: time.Now,
	}
}

// SetNotify replaces the change callback.
func (l *Logger) SetNotify(fn func()) {
	l.mu.Lock()
	l.notify = fn
	l.mu.Unlock()
}

// SetMinLevel drops future entries below level. Empty or unknown levels
// accept everything.
func (l *Logger) SetMinLevel(level domain.LogLevel) {
	rank, ok := levelRank[level]
	if !ok {
		rank = 0
	}
	l.mu.Lock()
	l.minRank = rank
	l.mu.Unlock()
}

// MinLevel reports the current minimum severity.
func (l *Logger) MinLevel() domain.LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for level, rank := range levelRank {
		if rank == l.minRank {
			return level
		}
	}
	return domain.LogLevelInfo
}

// Log records one entry. format is a printf-style message template;
// args may be nil. Entries below the minimum level are counted and
// dropped. The oldest entry is evicted once the ring is full.
func (l *Logger) Log(level domain.LogLevel, context, format string, args ...any) {
	if levelRank[level] < l.minRankLocked() {
		l.mu.Lock()
		l.dropped++
		l.mu.Unlock()
		return
	}
	entry := domain.LogEntry{
		Timestamp: l.nowFunc().UTC().Truncate(time.Second),
		Level:     level,
		Message:   strings.TrimRight(fmt.Sprintf(format, args...), "\n"),
		Context:   context,
	}

	l.mu.Lock()
	l.append(entry)
	notify := l.notify
	l.mu.Unlock()

	if notify != nil {
		notify()
	}
}

// Info logs at info level.
func (l *Logger) Info(context, format string, args ...any) {
	l.Log(domain.LogLevelInfo, context, format, args...)
}

// Warn logs at warn level.
func (l *Logger) Warn(context, format string, args ...any) {
	l.Log(domain.LogLevelWarn, context, format, args...)
}

// Error logs at error level.
func (l *Logger) Error(context, format string, args ...any) {
	l.Log(domain.LogLevelError, context, format, args...)
}

// minRankLocked reads minRank; callers must already hold at least a read
// lock, except Log which calls it before taking the write lock to avoid
// building entries that would be dropped anyway.
func (l *Logger) minRankLocked() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.minRank
}

// append inserts into the ring; l.mu must be held.
func (l *Logger) append(entry domain.LogEntry) {
	if l.count < len(l.ring) {
		l.ring[(l.head+l.count)%len(l.ring)] = entry
		l.count++
		return
	}
	l.ring[l.head] = entry
	l.head = (l.head + 1) % len(l.ring)
}

// Entries returns a snapshot of all buffered entries, oldest first.
func (l *Logger) Entries() []domain.LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.snapshot(func(domain.LogEntry) bool { return true })
}

// Filter selects entries for Query.
type Filter struct {
	// Search keeps entries whose message or context contains it
	// (case-insensitive). Empty matches everything.
	Search string
	// Level keeps entries at exactly this severity; empty keeps all.
	Level domain.LogLevel
	// Limit caps the result, keeping the newest entries; zero keeps all.
	Limit int
}

// Query returns the filtered snapshot, oldest first. Limit keeps the
// newest tail so the console shows the freshest lines.
func (l *Logger) Query(f Filter) []domain.LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	needle := strings.ToLower(f.Search)
	match := func(e domain.LogEntry) bool {
		if f.Level != "" && e.Level != f.Level {
			return false
		}
		if needle == "" {
			return true
		}
		return strings.Contains(strings.ToLower(e.Message), needle) ||
			strings.Contains(strings.ToLower(e.Context), needle)
	}
	all := l.snapshot(match)
	if f.Limit <= 0 || len(all) <= f.Limit {
		return all
	}
	return all[len(all)-f.Limit:]
}

// snapshot collects matching entries; l.mu must be held (read).
func (l *Logger) snapshot(match func(domain.LogEntry) bool) []domain.LogEntry {
	out := make([]domain.LogEntry, 0, l.count)
	for i := 0; i < l.count; i++ {
		entry := l.ring[(l.head+i)%len(l.ring)]
		if match(entry) {
			out = append(out, entry)
		}
	}
	return out
}

// Count returns the number of buffered entries.
func (l *Logger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.count
}

// Dropped reports how many entries were skipped by level filtering since
// process start.
func (l *Logger) Dropped() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.dropped
}

// Clear empties the buffer.
func (l *Logger) Clear() {
	l.mu.Lock()
	l.head, l.count = 0, 0
	l.mu.Unlock()
}

// Export writes entries as human-readable text lines, oldest first.
func (l *Logger) Export(w io.Writer, f Filter) error {
	for _, e := range l.Query(f) {
		line := e.Timestamp.Format("2006-01-02 15:04:05")
		if e.Context != "" {
			line += " [" + e.Context + "]"
		}
		line += " " + strings.ToUpper(string(e.Level)) + ": " + e.Message + "\n"
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}
