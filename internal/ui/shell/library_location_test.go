package shell

import (
	"context"
	"path/filepath"
	"testing"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
)

func TestLibraryRootSelectionPropagates(t *testing.T) {
	driveRoot := t.TempDir()
	rootDB, err := storage.Open(driveRoot, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rootDB.Raw().Exec(`INSERT INTO media_items (file_hash, original_relative_path, current_relative_path, file_size, mime_type, created_at)
		VALUES ('root-item', 'a.jpg', 'a.jpg', 10, 'image/jpeg', '2026-01-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	_ = rootDB.Close()

	libraryRoot := filepath.Join(driveRoot, "media", "albums")
	libDB, err := storage.Open(libraryRoot, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"lib-one", "lib-two"} {
		if _, err := libDB.Raw().Exec(`INSERT INTO media_items (file_hash, original_relative_path, current_relative_path, file_size, mime_type, created_at)
			VALUES (?, ?, ?, 10, 'image/jpeg', '2026-01-01 00:00:00')`, hash, hash+".jpg", hash+".jpg"); err != nil {
			t.Fatal(err)
		}
	}
	_ = libDB.Close()

	store := state.NewStore()
	store.ApplyDrives([]domain.DriveInfo{{
		ID: "drive", Name: "Drive", Path: driveRoot,
		Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	shell := New(store)
	library := newLibraryController(store, shell)
	library.sync()
	defer library.Close()

	count := func() int64 {
		t.Helper()
		n, err := storage.NewMediaRepo(library.db).Count(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := count(); got != 1 {
		t.Fatalf("initial catalog count = %d, want 1", got)
	}

	library.selectLibraryRoot(driveRoot, libraryRoot)
	if got := count(); got != 2 {
		t.Fatalf("catalog count after choosing library root = %d, want 2", got)
	}
	if got := shell.Discover().AlbumFolder(); got != libraryRoot {
		t.Fatalf("Discover album folder = %q, want %q", got, libraryRoot)
	}
	if got := loadLibraryRoot(driveRoot); got != libraryRoot {
		t.Fatalf("persisted library root = %q, want %q", got, libraryRoot)
	}

	// A fresh controller on the same drive must reuse the chosen location.
	reopenedStore := state.NewStore()
	reopenedStore.ApplyDrives([]domain.DriveInfo{{
		ID: "drive", Name: "Drive", Path: driveRoot,
		Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	reopened := newLibraryController(reopenedStore, New(reopenedStore))
	reopened.sync()
	defer reopened.Close()
	if got, err := storage.NewMediaRepo(reopened.db).Count(context.Background()); err != nil || got != 2 {
		t.Fatalf("count after reopen = %d, err %v; want 2", got, err)
	}
}
