// Package ui wires the native MyGo interface to application state.
package ui

import (
	"sync"

	"github.com/egoist/mygo"

	"painless/internal/state"
)

// Bridge forwards application state events onto the window's main thread
// via mygo Window.Update, where view models may be safely read and the
// Content rebuilds.
//
// The bridge stores only the window handle and a callback; it never holds
// *ui.Context or ui.Element values, which expire at the end of each frame.
type Bridge struct {
	win     *mygo.Window
	onEvent func(state.Event)
}

// NewBridge creates a bridge delivering events to onEvent on the main
// thread. onEvent runs inside Window.Update and must only mutate plain
// view-model data; it must not call back into the bus synchronously.
func NewBridge(win *mygo.Window, onEvent func(state.Event)) *Bridge {
	return &Bridge{win: win, onEvent: onEvent}
}

// Start subscribes the bridge to the store's bus and begins forwarding
// events from a background goroutine. The returned cancel function
// unsubscribes and stops the forwarding goroutine; it is safe to call
// multiple times.
func (b *Bridge) Start(store *state.Store) (cancel func()) {
	sub := store.Bus.Subscribe(64)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for ev := range sub.C {
			if b.win.IsDestroyed() {
				return
			}
			b.win.Update(func() {
				b.onEvent(ev)
			})
		}
	}()
	var cancelOnce sync.Once
	return func() {
		cancelOnce.Do(func() {
			sub.Cancel()
			<-stopped
		})
	}
}
