package shell

import (
	"context"
	"path/filepath"
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
	actions.OpenInViewer(item, page.Items)
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

	if err := tt.Click("Back to Library"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenDashboard {
		t.Fatalf("screen after viewer Close = %q, want dashboard (the route the viewer was opened from)", store.State.Screen())
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

// A catalog deep-link must carry the page with it: otherwise Next/Prev have
// an empty item list and silently do nothing.
func TestViewerDeepLinkKeepsNavigation(t *testing.T) {
	root := t.TempDir()
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := storage.NewMediaRepo(db)
	items := []domain.MediaItem{
		{FileHash: "nav-a", CurrentRelativePath: "photos/a.jpg", MimeType: "image/jpeg", FileSize: 1024},
		{FileHash: "nav-b", CurrentRelativePath: "photos/b.jpg", MimeType: "image/jpeg", FileSize: 1024},
	}
	if _, err := repo.InsertBatch(context.Background(), items); err != nil {
		t.Fatal(err)
	}
	page, err := repo.List(context.Background(), domain.MediaListParams{Limit: 10})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("seeded items = %+v, %v", page, err)
	}

	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)
	store.ApplyDrives([]domain.DriveInfo{{
		ID: "nav-drive", Name: "Nav drive", Path: root,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})

	first := filepath.Base(page.Items[0].CurrentRelativePath)
	second := filepath.Base(page.Items[1].CurrentRelativePath)
	(libraryActions{store: store, viewer: shell.Viewer()}).OpenInViewer(page.Items[0], page.Items)
	tt := mui.NewTester(shell.View, 1200, 800)
	tt.Frame()
	if !tt.HasText(first) {
		t.Fatalf("viewer texts after deep link = %q", tt.Texts())
	}
	if err := tt.Click("Next Item"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(second) {
		t.Fatalf("Next Item did not advance: %q", tt.Texts())
	}
	if err := tt.Click("Previous Item"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(first) {
		t.Fatalf("Previous Item did not go back: %q", tt.Texts())
	}
}

// TestViewerBackReturnsToAlbums checks that closing the viewer from the album
// detail returns to the albums screen, not the catalog.
func TestViewerBackReturnsToAlbums(t *testing.T) {
	store := state.NewStore()
	shell := New(store)

	store.Navigate(state.ScreenAlbums)
	shell.openInViewer(domain.MediaItem{ID: 1, CurrentRelativePath: "photos/a.jpg"})
	if store.State.Screen() != state.ScreenViewer {
		t.Fatalf("screen after openInViewer = %q, want viewer", store.State.Screen())
	}
	if got := store.State.BackScreen(); got != state.ScreenAlbums {
		t.Fatalf("back screen = %q, want albums", got)
	}

	shell.Viewer().Close()
	if got := store.State.Screen(); got != state.ScreenAlbums {
		t.Fatalf("screen after Close = %q, want albums", got)
	}
}
