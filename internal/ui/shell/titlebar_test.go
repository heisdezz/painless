package shell

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/state"
)

// fakeWindow records what the drawn window controls ask the window to do,
// and reports its state changes back as *mygo.Window does.
type fakeWindow struct {
	minimizeCalls int
	closeCalls    int
	invalidates   int
	maximized     bool
	maxListeners  []func()
	unmaxListener []func()
}

func (f *fakeWindow) Minimize()         { f.minimizeCalls++ }
func (f *fakeWindow) Close()            { f.closeCalls++ }
func (f *fakeWindow) Invalidate()       { f.invalidates++ }
func (f *fakeWindow) IsMaximized() bool { return f.maximized }

func (f *fakeWindow) ToggleMaximize() {
	f.maximized = !f.maximized
	listeners := f.maxListeners
	if !f.maximized {
		listeners = f.unmaxListener
	}
	for _, fn := range listeners {
		fn()
	}
}

func (f *fakeWindow) OnMaximize(fn func()) func() {
	f.maxListeners = append(f.maxListeners, fn)
	return func() {}
}

func (f *fakeWindow) OnUnmaximize(fn func()) func() {
	f.unmaxListener = append(f.unmaxListener, fn)
	return func() {}
}

// The window hides its own controls, so the top bar's buttons are the ones
// the user has: they drive the window and leave the app's screens alone.
func TestShellWindowControls(t *testing.T) {
	shell, store := newTestShell()
	win := &fakeWindow{}
	shell.SetWindow(win)
	tt := mui.NewTester(shell.View, 1200, 800)

	if err := tt.Click("Minimize"); err != nil {
		t.Fatal(err)
	}
	if win.minimizeCalls != 1 {
		t.Errorf("minimize calls = %d", win.minimizeCalls)
	}

	if err := tt.Click("Maximize"); err != nil {
		t.Fatal(err)
	}
	if !win.maximized {
		t.Fatal("the maximize button left the window unmaximized")
	}
	if win.invalidates == 0 {
		t.Error("maximizing did not redraw the shell")
	}
	if err := tt.Click("Restore"); err != nil {
		t.Errorf("a maximized window shows no Restore button: %v", err)
	}
	if win.maximized {
		t.Error("the restore button left the window maximized")
	}

	if err := tt.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if win.closeCalls != 1 {
		t.Errorf("close calls = %d", win.closeCalls)
	}
	if store.State.Screen() != state.ScreenDashboard {
		t.Errorf("screen after using the window controls = %q", store.State.Screen())
	}
}

// Without a window the shell still builds: the controls are simply not
// drawn, as before the window is created.
func TestShellWindowControlsWithoutWindow(t *testing.T) {
	shell, _ := newTestShell()
	tt := mui.NewTester(shell.View, 1200, 800)

	for _, label := range []string{"Minimize", "Maximize", "Close"} {
		if err := tt.Click(label); err == nil {
			t.Errorf("clicked %q with no window attached", label)
		}
	}
}
