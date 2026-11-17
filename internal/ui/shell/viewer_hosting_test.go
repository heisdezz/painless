package shell

import (
	"context"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
)

// TestViewerDeepLink checks that the catalog's OpenInViewer action and the
// dashboard's OpenViewer action open the viewer screen for the chosen item,
// and that closing the viewer returns to the library.
func TestViewerDeepLink(t *testing.T) {
	root := makeLibraryDrive(t, "view")
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)

	store.ApplyDrives([]domain.DriveInfo{{
		ID: "view-drive", Name: "View drive", Path: root,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})

	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	page, err := storage.NewMediaRepo(db).List(context.Background(), domain.MediaListParams{Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("seed items = %+v, %v", page, err)
	}
	item := page.Items[0]

	actions := libraryActions{store: store, viewer: shell.Viewer()}
	actions.OpenInViewer(item)
	if store.State.Screen() != state.ScreenViewer {
		t.Fatalf("screen after OpenInViewer = %q, want viewer", store.State.Screen())
	}

	tt := mui.NewTester(shell.View, 1200, 800)
	deadline := time.Now().Add(2 * time.Second)
	for {
		tt.Frame()
		if tt.HasText("view.jpg") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("viewer texts = %q", tt.Texts())
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := tt.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenCatalog {
		t.Fatalf("screen after viewer Close = %q, want catalog", store.State.Screen())
	}

	// The dashboard's recent-media carousel routes through the same viewer.
	dashActions := dashboardActions{store: store, viewer: shell.Viewer()}
	dashActions.OpenViewer(item, page.Items)
	if store.State.Screen() != state.ScreenViewer {
		t.Fatalf("screen after dashboard OpenViewer = %q, want viewer", store.State.Screen())
	}

	// A viewer without a bound drive still navigates; it shows its empty state.
	empty := dashboardActions{store: store}
	empty.OpenViewer(item, nil)
	if store.State.Screen() != state.ScreenViewer {
		t.Fatalf("screen without viewer = %q, want viewer", store.State.Screen())
	}
}
