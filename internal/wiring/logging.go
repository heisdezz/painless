package wiring

import (
	"painless/internal/domain"
	"painless/internal/logging"
)

// logf appends one entry to a controller's logger when one is wired.
// Controllers built directly in tests carry a nil logger and stay silent.
// Safe from any goroutine: the ring-buffer logger is concurrency-safe.
func logf(l *logging.Logger, context string, level domain.LogLevel, format string, args ...any) {
	if l == nil {
		return
	}
	l.Log(level, context, format, args...)
}
