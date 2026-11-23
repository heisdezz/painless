package shell

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/wiring"
)

// The shell hosts the settings screen built by the wiring package; this
// checks that the real controllers' values reach the rendered tabs.
func TestShellShowsWiredSettingsValues(t *testing.T) {
	root := makeLibraryDrive(t, "wired")
	store := state.NewStore()
	store.ApplyDrives([]domain.DriveInfo{{
		ID: "wired-drive", Name: "Wired drive", Path: root,
		Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	shell := New(store)
	controllers := wiring.New(store, shell.Logger())
	defer controllers.Close()
	shell.SetSettings(controllers.SettingsScreen(shell.Logger(), store.Notify))
	store.Navigate(state.ScreenSettings)

	tt := mui.NewTester(shell.View, 1200, 800)
	if !tt.HasText("Database health") || !tt.HasText(root) {
		t.Fatalf("backup tab texts = %q", tt.Texts())
	}
	if err := tt.Click("Mobile sync"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Enable mobile sync") || !tt.HasText("Listen port") {
		t.Fatalf("mobile tab texts = %q", tt.Texts())
	}
	if err := tt.Click("Google Drive"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Service account") {
		t.Fatalf("gdrive tab texts = %q", tt.Texts())
	}
}
