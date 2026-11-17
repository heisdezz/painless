package gdrive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/api/drive/v3"
)

// DatabaseSnapshotter defines the contract for generating a consistent SQLite backup on disk.
type DatabaseSnapshotter interface {
	CreateBackup(backupPath string) error
}

// BackupResult details the uploaded Google Drive file.
type BackupResult struct {
	FileID     string    `json:"file_id"`
	FileName   string    `json:"file_name"`
	WebLink    string    `json:"web_link,omitempty"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type countingReader struct {
	reader     io.Reader
	totalBytes int64
	readBytes  int64
	onProgress func(written, total int64)
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	if n > 0 {
		cr.readBytes += int64(n)
		if cr.onProgress != nil {
			cr.onProgress(cr.readBytes, cr.totalBytes)
		}
	}
	return n, err
}

// BackupDatabase takes an online snapshot of the SQLite database and streams it to Google Drive.
func (c *Client) BackupDatabase(
	ctx context.Context,
	snapshotter DatabaseSnapshotter,
	targetFolderID string,
	tempDir string,
	onProgress func(written, total int64),
) (*BackupResult, error) {
	if snapshotter == nil {
		return nil, fmt.Errorf("snapshotter is required")
	}
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	backupFileName := fmt.Sprintf("media_library_backup_%s.db", timestamp)
	localSnapshotPath := filepath.Join(tempDir, backupFileName)

	// 1. Generate live SQLite online backup (non-blocking snapshot)
	if err := snapshotter.CreateBackup(localSnapshotPath); err != nil {
		return nil, fmt.Errorf("failed to create database snapshot: %w", err)
	}
	defer func() {
		_ = os.Remove(localSnapshotPath)
	}()

	// 2. Upload snapshot to Google Drive
	return c.UploadFile(ctx, localSnapshotPath, targetFolderID, onProgress)
}

// UploadFile streams a file from the local filesystem to Google Drive.
func (c *Client) UploadFile(
	ctx context.Context,
	localFilePath string,
	targetFolderID string,
	onProgress func(written, total int64),
) (*BackupResult, error) {
	if c.service == nil {
		return nil, fmt.Errorf("google drive service is uninitialized")
	}

	info, err := os.Stat(localFilePath)
	if err != nil {
		return nil, fmt.Errorf("local file not found %s: %w", localFilePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot upload directory as single file: %s", localFilePath)
	}

	f, err := os.Open(localFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed opening file for upload: %w", err)
	}
	defer f.Close()

	reader := &countingReader{
		reader:     f,
		totalBytes: info.Size(),
		onProgress: onProgress,
	}

	driveFile := &drive.File{
		Name:     filepath.Base(localFilePath),
		MimeType: "application/x-sqlite3",
	}
	if targetFolderID != "" {
		driveFile.Parents = []string{targetFolderID}
	}

	insertCall := c.service.Files.Create(driveFile).Media(reader).Fields("id, name, webViewLink, size")
	created, err := insertCall.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed uploading file %s to Google Drive: %w", localFilePath, err)
	}

	return &BackupResult{
		FileID:     created.Id,
		FileName:   created.Name,
		WebLink:    created.WebViewLink,
		Size:       created.Size,
		UploadedAt: time.Now().UTC(),
	}, nil
}
