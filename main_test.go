package main

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo"

	"painless/internal/ui/shell"
)

// The demo counter is gone; the entry point now only wires the shell window.
func TestShellWindowOptions(t *testing.T) {
	options := shell.New(nil).WindowOptions()
	if options.Title != "painless" {
		t.Errorf("title = %q", options.Title)
	}
	if options.Width != 1200 || options.Height != 800 ||
		options.MinWidth != 800 || options.MinHeight != 600 {
		t.Errorf("unexpected window size: %+v", options)
	}
	if options.StateKey != "painless_main_window" {
		t.Errorf("state key = %q", options.StateKey)
	}
	if options.Content == nil {
		t.Fatal("window has no native UI content")
	}
	// The view decorates the window, so the system's title bar must be
	// gone. On Linux that takes Frameless: a hidden title bar keeps the
	// compositor's own title bar wherever the compositor decorates
	// windows itself (KWin on Plasma, Hyprland, Sway). Elsewhere a hidden
	// title bar without the system's buttons removes it while keeping the
	// shadow, corners and resizable top edge a frameless window loses on
	// Windows.
	if runtime.GOOS == "linux" {
		if !options.Frameless {
			t.Error("window keeps the compositor's title bar: not frameless")
		}
		return
	}
	if options.TitleBarStyle != mygo.TitleBarHidden {
		t.Errorf("title bar style = %q, want %q", options.TitleBarStyle, mygo.TitleBarHidden)
	}
	if !options.HideWindowButtons {
		t.Error("window still draws the system's window buttons")
	}
}
