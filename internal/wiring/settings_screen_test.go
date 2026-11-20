package wiring

import (
	"testing"

	"painless/internal/domain"

	mui "github.com/egoist/mygo/ui"
)

// The production settings screen replaces the one built in Shell.New, so its
// General switches must be bound to the same store the viewer reads.
func TestSettingsScreenGeneralTogglesStore(t *testing.T) {
	root := t.TempDir()
	seedLibrary(t, root).Close()
	store := newDriveStore(t, root)
	c := New(store)
	defer c.Close()

	screen := c.SettingsScreen(nil, func(domain.LogLevel, string, string) {})
	tt := mui.NewTester(screen.View, 1200, 800)
	if err := tt.Click("General"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	var switches []mui.AccessInfo
	for _, info := range tt.AccessAll() {
		if info.Role == mui.RoleSwitch {
			switches = append(switches, info)
		}
	}
	if len(switches) < 2 {
		t.Fatalf("want thumbnails and autoplay switches, got %d", len(switches))
	}
	for _, sw := range switches {
		tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
		tt.Frame()
	}
	if store.ThumbnailsEnabled() || store.AutoplayEnabled() {
		t.Fatalf("General switches did not reach the store: thumbnails=%v autoplay=%v",
			store.ThumbnailsEnabled(), store.AutoplayEnabled())
	}
}

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
