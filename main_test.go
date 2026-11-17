package main

import (
	"testing"

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
}
