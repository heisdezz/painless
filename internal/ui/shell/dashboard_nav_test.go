package shell

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
)

// TestDashboardIsDefaultScreen verifies the app lands on the dashboard
// picker when no drive is active.
func TestDashboardIsDefaultScreen(t *testing.T) {
	store := state.NewStore()
	shell := New(store)

	if store.State.Screen() != state.ScreenDashboard {
		t.Fatalf("default screen = %q, want dashboard", store.State.Screen())
	}
	tt := mui.NewTester(shell.View, 1200, 800)
	if !tt.HasText("Dashboard") && !tt.HasText("Select a drive") && !tt.HasText("No drive selected") {
		t.Fatalf("dashboard picker not rendered: %q", tt.Texts())
	}
}

// TestDashboardNavEntryAndHosting checks the sidebar exposes the dashboard
// and that selecting a drive hosts the drive overview with live stats.
func TestDashboardNavEntryAndHosting(t *testing.T) {
	root := makeLibraryDrive(t, "dash")
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)

	store.ApplyDrives([]domain.DriveInfo{{
		ID: "dash-drive", Name: "Dash drive", Path: root,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted,
		TotalBytes: 1000, FreeBytes: 400, UsedPercentage: 60,
		HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})

	tt := mui.NewTester(shell.View, 1200, 800)
	if !tt.HasText("Home") {
		t.Fatalf("dashboard nav entry missing: %q", tt.Texts())
	}

	// The dashboard is the landing screen; it should surface the active
	// drive's library data without navigating.
	if !tt.HasText("Dash drive") {
		t.Fatalf("active drive header missing: %q", tt.Texts())
	}

	// Navigate away and back through the sidebar.
	if err := tt.Click("Library"); err != nil {
		t.Fatalf("click Library: %v", err)
	}
	tt.Frame()
	if store.State.Screen() != state.ScreenCatalog {
		t.Fatalf("screen = %q, want catalog", store.State.Screen())
	}
	if err := tt.Click("Home"); err != nil {
		t.Fatalf("click Home: %v", err)
	}
	tt.Frame()
	if store.State.Screen() != state.ScreenDashboard {
		t.Fatalf("screen = %q, want dashboard", store.State.Screen())
	}
	if !tt.HasText("Dash drive") {
		t.Fatalf("dashboard not hosted after nav: %q", tt.Texts())
	}
}

// TestDashboardDisconnectReturnsToPicker drives the disconnect action.
func TestDashboardDisconnectReturnsToPicker(t *testing.T) {
	root := makeLibraryDrive(t, "disc")
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)

	store.ApplyDrives([]domain.DriveInfo{{
		ID: "disc-drive", Name: "Disc drive", Path: root,
		Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})
	library.HandleEvent(state.Event{Kind: state.EventMediaChanged})

	// Disconnect via the store path the dashboard action uses.
	store.ClearActiveDrive()
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})

	if store.State.ActiveDriveID() != "" {
		t.Fatalf("active drive = %q, want empty", store.State.ActiveDriveID())
	}
	tt := mui.NewTester(shell.View, 1200, 800)
	texts := tt.Texts()
	joined := ""
	for _, s := range texts {
		joined += s + " "
	}
	if !tt.HasText("Disc drive") { // picker still lists the detected drive
		t.Fatalf("picker lost detected drives: %q", joined)
	}
}
