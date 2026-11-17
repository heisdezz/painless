package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupFolder is the directory on the drive storing database backups.
const BackupFolder = ".media_backups"

// BackupInfo contains metadata about an archived database snapshot.
type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateBackup executes an online VACUUM INTO snapshot of the database.
func (d *DB) CreateBackup(ctx context.Context, driveRoot string) (string, error) {
	if driveRoot == "" {
		return "", fmt.Errorf("driveRoot path cannot be empty")
	}

	backupDir := filepath.Join(driveRoot, BackupFolder)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	backupFilename := fmt.Sprintf("backup_%s.db", timestamp)
	backupPath := filepath.Join(backupDir, backupFilename)

	// SQLite 3.27+ supports VACUUM INTO which generates an online atomic snapshot
	query := fmt.Sprintf("VACUUM INTO '%s';", strings.ReplaceAll(filepath.ToSlash(backupPath), "'", "''"))
	if _, err := d.db.ExecContext(ctx, query); err != nil {
		return "", fmt.Errorf("online backup failed: %w", err)
	}

	return backupPath, nil
}

// ListBackups scans .media_backups and returns all found backups sorted newest first.
func ListBackups(driveRoot string) ([]BackupInfo, error) {
	backupDir := filepath.Join(driveRoot, BackupFolder)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(backupDir, entry.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}
