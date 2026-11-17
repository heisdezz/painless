package shell

import (
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"painless/internal/domain"
	"painless/internal/state"
)

// ToastMessage is the plain data shown in a notification.
type ToastMessage struct {
	ID          string
	Title       string
	Description string
	Type        feedback.ToastType
	Timeout     time.Duration
}

// Toaster collects notifications and shows them in a single ToastHost.
// Worker messages travel through the state bus; only the UI thread calls
// Show or Host, so no *mui.Context or toast element is kept by a worker.
type Toaster struct {
	store   *state.Store
	pending []ToastMessage
}

func newToaster(store *state.Store) *Toaster {
	return &Toaster{store: store}
}

// Show queues a toast from a view or an input handler (UI thread only).
func (t *Toaster) Show(message ToastMessage) {
	t.pending = append(t.pending, message)
}

// Publish notifies the app from any goroutine. The window bridge delivers
// the matching event back on the UI thread, where HandleEvent queues it.
func (t *Toaster) Publish(message ToastMessage) {
	level := domain.LogLevelInfo
	switch message.Type {
	case feedback.ToastWarning:
		level = domain.LogLevelWarn
	case feedback.ToastError:
		level = domain.LogLevelError
	}
	t.store.Notify(level, message.Title, message.Description)
}

// HandleEvent applies a toast event delivered by the window bridge. Call
// only on the UI thread.
func (t *Toaster) HandleEvent(event state.Event) {
	if event.Kind != state.EventToast || event.Toast == nil {
		return
	}
	t.Show(ToastMessage{
		Title:       event.Toast.Title,
		Description: event.Toast.Body,
		Type:        toastTypeFor(event.Toast.Level),
	})
}

func toastTypeFor(level domain.LogLevel) feedback.ToastType {
	switch level {
	case domain.LogLevelWarn:
		return feedback.ToastWarning
	case domain.LogLevelError:
		return feedback.ToastError
	default:
		return feedback.ToastInfo
	}
}

// Success, Info, Warn and Error are shortcuts for the common toast kinds.
func (t *Toaster) Success(title, description string) {
	t.Show(ToastMessage{Title: title, Description: description, Type: feedback.ToastSuccess})
}

func (t *Toaster) Info(title, description string) {
	t.Show(ToastMessage{Title: title, Description: description, Type: feedback.ToastInfo})
}

func (t *Toaster) Warn(title, description string) {
	t.Show(ToastMessage{Title: title, Description: description, Type: feedback.ToastWarning})
}

func (t *Toaster) Error(title, description string) {
	t.Show(ToastMessage{Title: title, Description: description, Type: feedback.ToastError})
}

// Host drains queued toasts and draws the stack. Call once per view pass.
func (t *Toaster) Host(c *mui.Context) {
	for _, message := range t.pending {
		feedback.ShowToast(c, feedback.ToastProps{
			ID:          message.ID,
			Title:       message.Title,
			Description: message.Description,
			Type:        message.Type,
			Timeout:     message.Timeout,
		})
	}
	t.pending = nil
	feedback.ToastHost(c, feedback.ToastHostProps{Placement: feedback.ToastBottomRight})
}
