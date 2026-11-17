package shell

import (
	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"

	"painless/internal/state"
	uibridge "painless/internal/ui"
)

const stateKey = "painless_main_window"

// WindowOptions describes the main window. StateKey preserves the geometry
// and maximized state the user left between launches.
func (s *Shell) WindowOptions() mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:     "painless",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		StateKey:  stateKey,
		Content:   mui.View(s.View),
	}
}

// Open creates the main window, its state store and the event bridge. Call
// from mygo.App.WhenReady. The returned cancel hook is wired to OnClosed.
func Open() *mygo.Window {
	store := state.NewStore()
	shell := New(store)
	win := mygo.NewWindow(shell.WindowOptions())
	bridge := uibridge.NewBridge(win, func(event state.Event) {
		shell.toaster.HandleEvent(event)
	})
	cancel := bridge.Start(store)
	win.OnClosed(func() { cancel() })
	return win
}
