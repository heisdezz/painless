package wiring

import (
	"testing"

	"painless/internal/domain"

	mui "github.com/egoist/mygo/ui"
)

// TestSettingsScreenSmoke renders the real settings screen backed by the
// real controllers against a temporary drive and walks its tabs.
func TestSettingsScreenSmoke(t *testing.T) {
	root := t.TempDir()
	seedLibrary(t, root).Close()
	c := New(newDriveStore(t, root))
	defer c.Close()

	var toasts []string
	screen := c.SettingsScreen(nil, func(level domain.LogLevel, title, body string) {
		toasts = append(toasts, title+": "+body)
	})
	tt := mui.NewTester(screen.View, 1200, 800)

	if !tt.HasText("Database health") {
		t.Fatalf("backups tab missing: %q", tt.Texts())
	}

	for _, tab := range []string{"Mobile sync", "Google Drive", "Logs"} {
		if err := tt.Click(tab); err != nil {
			t.Fatalf("click %q: %v", tab, err)
		}
		tt.Frame()
	}
	if !tt.HasText("Logs") {
		t.Fatalf("logs tab did not render: %q", tt.Texts())
	}
}
