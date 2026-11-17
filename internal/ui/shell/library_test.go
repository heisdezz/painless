package shell

import (
	"context"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
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

func TestLibraryIgnoresUnrelatedEvents(t *testing.T) {
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)
	library.HandleEvent(state.Event{Kind: state.EventSelectionChanged})
	if library.db != nil || library.drive != "" {
		t.Fatal("unrelated event opened a library")
	}
}
