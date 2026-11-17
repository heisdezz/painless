package wiring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/ui/screens/settings"
)

// backupTimeout bounds one VACUUM INTO snapshot of a large library.
const backupTimeout = 2 * time.Minute

// Compile-time proof the controller satisfies the settings screen contract.
var _ settings.BackupsController = (*Backups)(nil)

// Backups implements settings.BackupsController against the active drive's
// portable SQLite library. Connections are opened per operation so the
// controller never fights the library controller for the live handle; the
// extra short-lived WAL connections are cheap.
type Backups struct {
	store *state.Store
}

// NewBackups builds the backup controller for a store.
func NewBackups(store *state.Store) *Backups { return &Backups{store: store} }

// activeRoot returns the active drive root, or an error when none.
func (b *Backups) activeRoot() (string, error) {
	drive, ok := b.store.State.ActiveDrive()
	if !ok || drive.Path == "" {
		return "", errors.New("no active drive")
	}
	return drive.Path, nil
}

// DBPath is the library database location for the health card.
func (b *Backups) DBPath() string {
	root, err := b.activeRoot()
	if err != nil {
		return ""
	}
	return filepath.Join(root, storage.LibraryDBFilename)
}

// DBSizeBytes is the current database file size, 0 when unknown.
func (b *Backups) DBSizeBytes() int64 {
	info, err := os.Stat(b.DBPath())
	if err != nil {
		return 0
	}
	return info.Size()
}

// withLibrary runs fn on a short-lived connection to the active drive's
// library, closing the connection even when fn fails.
func withLibrary(root string, fn func(db *storage.DB) error) error {
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		return fmt.Errorf("open library: %w", err)
	}
	if err := fn(db); err != nil {
		return errors.Join(err, db.Close())
	}
	return db.Close()
}

// CreateBackup snapshots the database into .media_backups.
func (b *Backups) CreateBackup() (string, error) {
	root, err := b.activeRoot()
	if err != nil {
		return "", err
	}
	var path string
	err = withLibrary(root, func(db *storage.DB) error {
		ctx, cancel := context.WithTimeout(context.Background(), backupTimeout)
		defer cancel()
		var err error
		path, err = db.CreateBackup(ctx, root)
		return err
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ListBackups returns existing snapshots, newest first.
func (b *Backups) ListBackups() ([]settings.BackupInfo, error) {
	root, err := b.activeRoot()
	if err != nil {
		return nil, err
	}
	rows, err := storage.ListBackups(root)
	if err != nil {
		return nil, err
	}
	out := make([]settings.BackupInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, settings.BackupInfo{
			Name:      row.Name,
			Path:      row.Path,
			SizeBytes: row.SizeBytes,
			CreatedAt: row.CreatedAt.Local().Format(time.RFC3339),
		})
	}
	return out, nil
}

// RestoreBackup copies a snapshot over the live database file. The copy is
// staged next to the destination and swapped in with a rename so a crash
// mid-copy cannot truncate the library. Other components' open connections
// keep serving cached pages until they reopen; the screen confirms the
// action with a modal first, so callers accept that trade-off.
func (b *Backups) RestoreBackup(path string) error {
	root, err := b.activeRoot()
	if err != nil {
		return err
	}
	dest := filepath.Join(root, storage.LibraryDBFilename)
	if path == dest {
		return errors.New("cannot restore the database onto itself")
	}
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer src.Close()
	tmp, err := os.CreateTemp(root, ".restore-*"+storage.LibraryDBFilename)
	if err != nil {
		return fmt.Errorf("stage restore: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("copy backup: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("save staged restore: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace database: %w", err)
	}
	return nil
}

// IntegrityCheck runs PRAGMA integrity_check and returns its report.
func (b *Backups) IntegrityCheck() (string, bool) {
	root, err := b.activeRoot()
	if err != nil {
		return err.Error(), false
	}
	var report string
	err = withLibrary(root, func(db *storage.DB) error {
		return db.Raw().QueryRow("PRAGMA integrity_check").Scan(&report)
	})
	if err != nil {
		return err.Error(), false
	}
	return report, report == "ok"
}
