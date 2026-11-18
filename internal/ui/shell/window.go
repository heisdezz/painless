package shell

import (
	"context"
	"runtime"

	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"

	"painless/internal/drive"
	"painless/internal/state"
	uibridge "painless/internal/ui"
	"painless/internal/wiring"
)

const stateKey = "painless_main_window"

// titleBarHeight is the height of the top bar, which doubles as the
// window's title bar: it holds the wordmark, the drive switcher and the
// window controls, and drags the window.
const titleBarHeight = 52

// windowHost is the part of *mygo.Window the title bar drives. The shell
// keeps it behind an interface so tests can watch what the window controls
// ask the window to do.
type windowHost interface {
	Minimize()
	ToggleMaximize()
	Close()
	IsMaximized() bool
	OnMaximize(func()) (off func())
	OnUnmaximize(func()) (off func())
	Invalidate()
}

// WindowOptions describes the main window. StateKey preserves the geometry
// and maximized state the user left between launches.
//
// The system's title bar and window buttons are gone, so the view
// decorates the window: the top bar drags it and draws minimize, maximize
// and close (drive_bar.go). How the system's half is removed differs by
// platform — on Linux a hidden title bar still leaves the compositor's own
// title bar wherever the compositor decorates windows itself (KWin on
// Plasma, Hyprland, Sway), so the window goes frameless instead and GTK
// draws nothing around the page but a shadow and a thin border. Windows
// and macOS keep the title bar's edges with a hidden title bar: its
// shadow, rounded corners and resizable top edge, which a Frameless
// window lacks on Windows.
func (s *Shell) WindowOptions() mygo.WindowOptions {
	options := mygo.WindowOptions{
		Title:     "painless",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		StateKey:  stateKey,
		Content:   mui.View(s.View),
	}
	if runtime.GOOS == "linux" {
		options.Frameless = true
		return options
	}
	options.TitleBarStyle = mygo.TitleBarHidden
	options.HideWindowButtons = true
	return options
}

// SetWindow gives the shell the window its drawn controls act on, and
// keeps the maximize button's glyph in step with the window's state, which
// the user can also change by double-clicking the title bar or with the
// system's own shortcuts. Call as soon as NewWindow returns, before the
// view's first frame.
func (s *Shell) SetWindow(w windowHost) {
	for _, off := range s.windowOffs {
		off()
	}
	s.windowOffs = nil
	s.win = w
	s.maximized = false
	if w == nil {
		return
	}
	s.maximized = w.IsMaximized()
	stateChanged := func() {
		s.maximized = w.IsMaximized()
		w.Invalidate()
	}
	s.windowOffs = append(s.windowOffs, w.OnMaximize(stateChanged), w.OnUnmaximize(stateChanged))
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
	shell.SetWindow(win)
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
