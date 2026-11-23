package shell

import (
	"context"
	"fmt"

	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"painless/internal/domain"
	"painless/internal/logging"
	"painless/internal/state"
)

// driveLister is the part of the drive service the shell needs. A fake can
// stand in for it in tests.
type driveLister interface {
	ListDrives(context.Context) ([]domain.DriveInfo, error)
}

// driveController bridges the drive service and its monitor to the central
// store. Scans run off the UI thread; the store publishes bus events that
// the window bridge turns into redraws.
type driveController struct {
	store   *state.Store
	lister  driveLister
	toaster *Toaster
	logs    *logging.Logger
	events  <-chan domain.DriveEvent
	ctx     context.Context
}

func newDriveController(store *state.Store, lister driveLister, toaster *Toaster, logs *logging.Logger, events <-chan domain.DriveEvent) *driveController {
	return &driveController{store: store, lister: lister, toaster: toaster, logs: logs, events: events}
}

// logf appends one entry to the logs console when a logger is wired; tests
// build the controller without one. Safe from any goroutine.
func (c *driveController) logf(level domain.LogLevel, format string, args ...any) {
	if c.logs != nil {
		c.logs.Log(level, "drives", format, args...)
	}
}

// scan replaces the store's drive list. Safe from any goroutine.
func (c *driveController) scan(ctx context.Context) (int, error) {
	drives, err := c.lister.ListDrives(ctx)
	if err != nil {
		c.logf(domain.LogLevelError, "scan failed: %v", err)
		return 0, err
	}
	c.store.ApplyDrives(drives)
	return len(drives), nil
}

// Refresh is the action behind the Refresh button. It must be called on the
// UI thread; the actual scan is posted to a worker so input never blocks.
func (c *driveController) Refresh(ctx context.Context) {
	if c.ctx != nil {
		ctx = c.ctx
	}
	c.toaster.Show(ToastMessage{
		Title:       "Scanning for drives",
		Description: "Looking for removable volumes",
		Type:        feedback.ToastLoading,
	})
	go func() {
		count, err := c.scan(ctx)
		if err != nil {
			c.toaster.Publish(ToastMessage{
				Title: "Drive scan failed", Description: err.Error(), Type: feedback.ToastError,
			})
			return
		}
		c.logf(domain.LogLevelInfo, "manual scan found %d drive(s)", count)
		c.toaster.Publish(ToastMessage{
			Title:       "Drive scan complete",
			Description: fmt.Sprintf("%d drive(s) available", count),
			Type:        feedback.ToastSuccess,
		})
	}()
}

// Start seeds the store with the drives present right now and then watches
// monitor events for insertions and removals until ctx is cancelled.
func (c *driveController) Start(ctx context.Context) {
	c.ctx = ctx
	if count, err := c.scan(ctx); err != nil {
		c.toaster.Publish(ToastMessage{
			Title: "Drive scan failed", Description: err.Error(), Type: feedback.ToastError,
		})
	} else {
		c.logf(domain.LogLevelInfo, "startup scan found %d drive(s)", count)
	}
	if c.events == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-c.events:
				if !ok {
					return
				}
				if count, err := c.scan(ctx); err != nil {
					c.toaster.Publish(ToastMessage{
						Title: "Drive scan failed", Description: err.Error(), Type: feedback.ToastError,
					})
				} else {
					c.logf(domain.LogLevelInfo, "drive change detected, rescanned: %d drive(s)", count)
				}
			}
		}
	}()
}
