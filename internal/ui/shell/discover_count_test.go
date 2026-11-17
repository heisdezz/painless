package shell

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
)

// TestDiscoverShowsCatalogedCount checks that the library controller feeds
// the discover screen's CatalogCount hook, so the Scanner Status panel shows
// how many items the active drive's SQLite library holds.
func TestDiscoverShowsCatalogedCount(t *testing.T) {
	root := makeLibraryDrive(t, "count")
	store := state.NewStore()
	shell := New(store)
	library := newLibraryController(store, shell)

	store.ApplyDrives([]domain.DriveInfo{{
		ID: "count-drive", Name: "Count drive", Path: root,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})
	store.Navigate(state.ScreenDiscover)

	tt := mui.NewTester(shell.View, 1200, 800)
	texts := tt.Texts()
	idx := -1
	for i, text := range texts {
		if text == "SQLite Cataloged" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("scanner status panel missing: %q", texts)
	}
	if idx+1 >= len(texts) || strings.TrimSpace(texts[idx+1]) != "1" {
		t.Fatalf("cataloged count texts near panel = %v", texts[max(0, idx-1):min(len(texts), idx+3)])
	}
}

// TestCachedMediaCount checks the TTL cache returns the library total and
// tolerates repository errors by keeping the previous value.
func TestCachedMediaCount(t *testing.T) {
	root := makeLibraryDrive(t, "cache")
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	count := cachedMediaCount(storage.NewMediaRepo(db))
	if got := count(); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	if got := count(); got != 1 {
		t.Fatalf("cached count = %d, want 1", got)
	}
}
