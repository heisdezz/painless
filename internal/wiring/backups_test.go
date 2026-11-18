package wiring

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
)

// newDriveStore builds a store with one active drive at root.
func newDriveStore(t *testing.T, root string) *state.Store {
	t.Helper()
	st := state.NewStore()
	st.ApplyDrives([]domain.DriveInfo{{ID: "drive-1", Path: root, Name: "Test Drive"}})
	st.SetActiveDrive("drive-1")
	return st
}

// seedLibrary opens a library on root and adds a marker table with rows.
func seedLibrary(t *testing.T, root string) *storage.DB {
	t.Helper()
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	if _, err := db.Raw().Exec(`CREATE TABLE IF NOT EXISTS marker (id INTEGER PRIMARY KEY, note TEXT)`); err != nil {
		t.Fatalf("create marker: %v", err)
	}
	if _, err := db.Raw().Exec(`INSERT INTO marker (note) VALUES ('seed')`); err != nil {
		t.Fatalf("insert marker: %v", err)
	}
	return db
}

func TestBackupsCreateListIntegrity(t *testing.T) {
	root := t.TempDir()
	db := seedLibrary(t, root)
	b := NewBackups(newDriveStore(t, root))

	if path := b.DBPath(); path != filepath.Join(root, storage.LibraryDBFilename) {
		t.Fatalf("DBPath = %q", path)
	}
	if size := b.DBSizeBytes(); size <= 0 {
		t.Fatalf("DBSizeBytes = %d, want > 0", size)
	}

	backupPath, err := b.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if !strings.HasPrefix(backupPath, filepath.Join(root, storage.BackupFolder)) {
		t.Fatalf("backup path %q outside backup folder", backupPath)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	backups, err := b.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 || backups[0].Path != backupPath {
		t.Fatalf("ListBackups = %+v, want one entry for %q", backups, backupPath)
	}
	if backups[0].SizeBytes <= 0 || backups[0].Name == "" || backups[0].CreatedAt == "" {
		t.Fatalf("backup metadata incomplete: %+v", backups[0])
	}

	report, ok := b.IntegrityCheck()
	if !ok || report != "ok" {
		t.Fatalf("IntegrityCheck = (%q, %v)", report, ok)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}
}

func TestBackupsRestoreRecoversMarker(t *testing.T) {
	root := t.TempDir()
	db := seedLibrary(t, root)
	store := newDriveStore(t, root)
	b := NewBackups(store)

	events := make(chan state.Event, 4)
	sub := store.Bus.Subscribe(4)
	defer sub.Cancel()
	go func() {
		for e := range sub.C {
			events <- e
		}
		close(events)
	}()

	backupPath, err := b.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// Diverge the live database after the snapshot.
	if _, err := db.Raw().Exec(`ALTER TABLE marker ADD COLUMN extra TEXT`); err != nil {
		t.Fatalf("diverge: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := b.RestoreBackup(backupPath); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	select {
	case e := <-events:
		if e.Kind != state.EventLibraryRestored {
			t.Fatalf("event kind = %q, want library_restored", e.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EventLibraryRestored was not published after restore")
	}

	restored := seedLibraryNoMarker(t, root)
	defer restored.Close()
	var count int
	if err := restored.Raw().QueryRow(`SELECT COUNT(*) FROM marker`).Scan(&count); err != nil {
		t.Fatalf("marker table lost after restore: %v", err)
	}
	if count != 1 {
		t.Fatalf("marker rows = %d, want 1", count)
	}
}

// seedLibraryNoMarker opens the library without touching the marker table.
func seedLibraryNoMarker(t *testing.T, root string) *storage.DB {
	t.Helper()
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatalf("reopen library: %v", err)
	}
	return db
}

func TestBackupsRestoreRejectsSelf(t *testing.T) {
	root := t.TempDir()
	seedLibrary(t, root).Close()
	b := NewBackups(newDriveStore(t, root))
	if err := b.RestoreBackup(b.DBPath()); err == nil {
		t.Fatal("expected error restoring database onto itself")
	}
}

func TestBackupsWithoutDrive(t *testing.T) {
	st := state.NewStore()
	b := NewBackups(st)

	if b.DBPath() != "" {
		t.Fatalf("DBPath = %q, want empty", b.DBPath())
	}
	if b.DBSizeBytes() != 0 {
		t.Fatalf("DBSizeBytes = %d, want 0", b.DBSizeBytes())
	}
	if _, err := b.CreateBackup(); err == nil {
		t.Fatal("expected CreateBackup error without drive")
	}
	if _, err := b.ListBackups(); err == nil {
		t.Fatal("expected ListBackups error without drive")
	}
	if report, ok := b.IntegrityCheck(); ok || report == "" {
		t.Fatalf("IntegrityCheck = (%q, %v), want failure", report, ok)
	}
	if err := b.RestoreBackup("/nonexistent"); err == nil && !errors.Is(err, nil) {
		t.Fatal("unexpected success")
	} else if err == nil {
		t.Fatal("expected RestoreBackup error without drive")
	}
}
