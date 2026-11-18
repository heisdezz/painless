package shell

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/ui/screens/discover"
)

func makeLibraryDrive(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = storage.NewMediaRepo(db).InsertBatch(context.Background(), []domain.MediaItem{{
		FileHash:            "hash-" + name,
		CurrentRelativePath: "photos/" + name + ".jpg",
		MimeType:            "image/jpeg",
		FileSize:            1024,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLibraryFollowsActiveDrive(t *testing.T) {
	catRoot := makeLibraryDrive(t, "cat")
	dogRoot := makeLibraryDrive(t, "dog")
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)

	store.ApplyDrives([]domain.DriveInfo{{
		ID: "cat-drive", Name: "Cat drive", Path: catRoot,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})

	tt := mui.NewTester(shell.View, 1200, 800)
	store.Navigate(state.ScreenDashboard)
	tt.Frame()
	if !tt.HasText("Cat drive") {
		t.Fatalf("dashboard did not receive the active drive: %q", tt.Texts())
	}

	store.Navigate(state.ScreenCatalog)
	tt.Frame()
	if !tt.HasText("cat.jpg") {
		t.Fatalf("catalog texts for first drive = %q", tt.Texts())
	}
	if library.db == nil || library.thumbs == nil {
		t.Fatal("library services were not opened for the active drive")
	}
	if got := shell.Discover().TargetPath(); got != catRoot {
		t.Fatalf("discover target = %q, want the drive root", got)
	}
	firstDB := library.db

	// Staged work belongs to the drive that produced it.
	shell.Discover().StageItem(domain.MediaItem{ID: 7, FileHash: "h7", CurrentRelativePath: "in/a.jpg"}, "Trips")
	if shell.Discover().StagedCount() != 1 {
		t.Fatal("staging did not record the item")
	}

	store.ApplyDrives([]domain.DriveInfo{{
		ID: "dog-drive", Name: "Dog drive", Path: dogRoot,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventDrivesChanged})
	tt.Frame()
	if !tt.HasText("dog.jpg") || tt.HasText("cat.jpg") {
		t.Fatalf("catalog texts after drive switch = %q", tt.Texts())
	}
	if library.db == firstDB {
		t.Fatal("database was not reopened for the new drive")
	}
	if got := shell.Discover().TargetPath(); got != dogRoot {
		t.Fatalf("discover target after switch = %q, want %q", got, dogRoot)
	}
	if shell.Discover().StagedCount() != 0 {
		t.Fatal("staging carried over to the new drive")
	}

	store.ApplyDrives(nil)
	library.HandleEvent(state.Event{Kind: state.EventDrivesChanged})
	tt.Frame()
	if !tt.HasText("No media found") {
		t.Fatalf("catalog texts with no drive = %q", tt.Texts())
	}
	if library.db != nil || library.thumbs != nil {
		t.Fatal("library services were not closed when the drive went away")
	}

	if err := tt.Click("Start Scan"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenDiscover {
		t.Fatalf("screen after Start Scan = %q", store.State.Screen())
	}
}

func TestLibraryLoadsAlbumFolder(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "media", "albums")
	writePhoto := func(dir, name string, shade uint8) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.Gray{Y: shade})
		err = png.Encode(f, img)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	writePhoto(filepath.Join(folder, "Trip"), "inside.png", 42)
	writePhoto(root, "outside.png", 100)

	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)
	defer library.Close()
	updates := make(chan func(), 64)
	library.update = func(apply func()) { updates <- apply }
	events := store.Bus.Subscribe(16)
	defer events.Cancel()
	store.ApplyDrives([]domain.DriveInfo{{
		ID: "photos", Path: root, Status: domain.DriveStatusMounted,
	}})
	library.sync()
	store.Navigate(state.ScreenCatalog)
	tt := mui.NewTester(shell.View, 1200, 800)
	waitForRefresh := func() {
		t.Helper()
		timeout := time.After(5 * time.Second)
		for {
			select {
			case apply := <-updates:
				apply()
			case event := <-events.C:
				if event.Kind == state.EventMediaChanged {
					library.HandleEvent(event)
					tt.Frame()
					return
				}
			case <-timeout:
				t.Fatal("album indexing did not refresh the library")
			}
		}
	}
	if tt.HasText("inside.png") || tt.HasText("outside.png") {
		t.Fatalf("catalog before an explicit scan = %q", tt.Texts())
	}

	// Opening a drive no longer scans on its own; the album folder is indexed
	// only when the user asks for it.
	shell.Discover().RescanAlbumFolder()
	waitForRefresh()
	tt.Frame()
	if !tt.HasText("inside.png") || tt.HasText("outside.png") || shell.Discover().StagedCount() != 0 {
		t.Fatalf("catalog after loading the default folder = %q; staged = %d", tt.Texts(), shell.Discover().StagedCount())
	}

	writePhoto(folder, "added.png", 65)
	shell.Discover().RescanAlbumFolder()
	waitForRefresh()
	page, err := storage.NewMediaRepo(library.db).List(context.Background(), domain.MediaListParams{Limit: 10})
	if err != nil || page.Total != 2 || !tt.HasText("added.png") {
		t.Fatalf("rescan page = %+v, error = %v, catalog = %q", page, err, tt.Texts())
	}
	for _, item := range page.Items {
		if _, err := os.Stat(filepath.Join(root, item.CurrentRelativePath)); err != nil {
			t.Fatalf("indexed path no longer exists: %v", err)
		}
	}
}

func TestLibraryDeleteStagesItem(t *testing.T) {
	root := makeLibraryDrive(t, "del")
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	media := storage.NewMediaRepo(db)
	page, err := media.List(context.Background(), domain.MediaListParams{Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("seed items = %+v, %v", page, err)
	}

	store := state.NewStore()
	screen := discover.New(discover.Options{})
	actions := libraryActions{store: store, media: media, discover: screen}
	actions.Delete([]int64{page.Items[0].ID})

	if screen.StagedCount() != 1 {
		t.Fatalf("staged = %d, want 1", screen.StagedCount())
	}
	if store.State.Screen() != state.ScreenDiscover {
		t.Fatalf("screen = %q, want discover", store.State.Screen())
	}
}

func TestLibraryIgnoresUnrelatedEvents(t *testing.T) {
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)
	library.HandleEvent(state.Event{Kind: state.EventSelectionChanged})
	if library.db != nil || library.drive != "" {
		t.Fatal("unrelated event opened a library")
	}
}
