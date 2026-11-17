package shell

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/logging"
	"painless/internal/state"
)

func TestShellHostsSettingsAndLogs(t *testing.T) {
	shell, store := newTestShell()
	tt := mui.NewTester(shell.View, 1200, 800)

	if shell.Settings() == nil {
		t.Fatal("shell did not construct a default settings screen")
	}
	if shell.Logger() == nil {
		t.Fatal("shell did not construct a logger")
	}

	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenSettings {
		t.Fatalf("screen after clicking Settings = %q", store.State.Screen())
	}
	tt.Frame()
	// Default shell has no controllers wired, so tabs show empty states.
	if !tt.HasText("Backups unavailable") || !tt.HasText("Backups, mobile pairing and application preferences.") {
		t.Fatalf("settings screen not hosted: %q", tt.Texts())
	}

	// The standalone logs entry routes to the settings screen's logs tab.
	if err := tt.Click("Logs"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenLogs {
		t.Fatalf("screen after clicking Logs = %q", store.State.Screen())
	}
	tt.Frame()
	if !tt.HasText("Auto-scroll") {
		t.Fatalf("logs tab not shown for ScreenLogs: %q", tt.Texts())
	}
}

func TestShellSettingsLoggerFeedsConsole(t *testing.T) {
	shell, _ := newTestShell()
	log := shell.Logger()
	log.Warn("test", "hello from the wiring test")

	screen := shell.Settings()
	entries := screen.Logs().Query(logging.Filter{})
	if len(entries) != 1 || entries[0].Message != "hello from the wiring test" {
		t.Fatalf("entries = %+v", entries)
	}
}
