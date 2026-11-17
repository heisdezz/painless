package shell

import (
	"context"

	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"

	"painless/internal/drive"
	"painless/internal/state"
	uibridge "painless/internal/ui"
	"painless/internal/wiring"
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

// Open creates the main window, its state store, the drive monitor and the
// event bridge. Call from mygo.App.WhenReady. Scans seed the store before the
// first frame; monitor events and the Refresh button rescan on demand.
func Open() *mygo.Window {
	store := state.NewStore()
	shell := New(store)
	controllers := wiring.New(store)
	shell.SetSettings(controllers.SettingsScreen(shell.Logger(), store.Notify))
	ctx, cancel := context.WithCancel(context.Background())

	drives := drive.NewService()
	monitor := drive.NewMonitor(drives, drive.DefaultPollInterval)
	controller := newDriveController(store, drives, shell.toaster, monitor.Events())
	shell.attachDrives(controller)

	library := newLibraryController(store, shell)
	win := mygo.NewWindow(shell.WindowOptions())
	library.update = win.Update
	bridge := uibridge.NewBridge(win, func(event state.Event) {
		shell.toaster.HandleEvent(event)
		library.HandleEvent(event)
	})
	cancelBridge := bridge.Start(store)

	monitor.Start(ctx)
	controller.Start(ctx)
	library.sync()

	win.OnClosed(func() {
		cancel()
		monitor.Stop()
		library.Close()
		controllers.Close()
		cancelBridge()
	})
	return win
}
