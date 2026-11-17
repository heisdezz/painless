package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
)

func TestOpenMemoryDatabase(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	// Verify schema tables were created by migrations
	tables := []string{"albums", "media_items", "tags", "media_tags", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := db.Raw().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("expected table %s to exist, got error: %v", table, err)
		}
	}
}

func TestOpenDiskDatabase(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "painless_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := Open(tempDir, OpenOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify WAL mode
	var journalMode string
	if err := db.Raw().QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode 'wal', got '%s'", journalMode)
	}

	// Verify foreign keys enabled
	var foreignKeys int
	if err := db.Raw().QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("failed to query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("expected foreign_keys 1, got %d", foreignKeys)
	}
}

func TestWithTxCommitAndRollback(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test successful commit
	err = db.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO albums (name, relative_path) VALUES (?, ?)", "Summer", "albums/Summer")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx commit failed: %v", err)
	}

	var count int
	if err := db.Raw().QueryRow("SELECT count(*) FROM albums WHERE name='Summer'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected 1 album, got %d (err: %v)", count, err)
	}

	// Test rollback on error
	forcedErr := errors.New("something went wrong")
	err = db.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO albums (name, relative_path) VALUES (?, ?)", "Winter", "albums/Winter")
		if err != nil {
			return err
		}
		return forcedErr
	})
	if !errors.Is(err, forcedErr) {
		t.Fatalf("expected forcedErr, got %v", err)
	}

	if err := db.Raw().QueryRow("SELECT count(*) FROM albums WHERE name='Winter'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("expected 0 winter albums after rollback, got %d", count)
	}
}

func TestBackupCreationAndListing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "painless_backup_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := Open(tempDir, OpenOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Insert test album
	_, err = db.Raw().Exec("INSERT INTO albums (name, relative_path) VALUES (?, ?)", "Travel", "albums/Travel")
	if err != nil {
		t.Fatalf("failed to insert album: %v", err)
	}

	backupPath, err := db.CreateBackup(ctx, tempDir)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file does not exist at %s: %v", backupPath, err)
	}

	backups, err := ListBackups(tempDir)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}

	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].SizeBytes == 0 {
		t.Errorf("backup size should be > 0 bytes")
	}
}

func TestReadOnlyMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "painless_ro_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize database first
	initDB, err := Open(tempDir, OpenOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("initial Open failed: %v", err)
	}
	initDB.Close()

	// Reopen in read-only mode
	roDB, err := Open(tempDir, OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("read-only Open failed: %v", err)
	}
	defer roDB.Close()

	if !roDB.IsReadOnly() {
		t.Errorf("expected IsReadOnly() to be true")
	}

	// Writes must fail in read-only mode
	ctx := context.Background()
	err = roDB.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO albums (name, relative_path) VALUES (?, ?)", "Fail", "albums/Fail")
		return err
	})
	if err == nil {
		t.Fatalf("expected write transaction to fail in read-only mode")
	}
}
