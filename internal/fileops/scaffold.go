package fileops

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// AlbumsDirName is the directory holding organized album folders.
	AlbumsDirName = "albums"

	// UnknownDirName is the fallback directory for consolidated unassigned items.
	UnknownDirName = "unknown"

	// ThumbnailsDirName is the hidden directory storing generated WebP thumbnails.
	ThumbnailsDirName = ".media_thumbnails"

	// BackupsDirName is the hidden directory storing SQLite database snapshots.
	BackupsDirName = ".media_backups"

	// SettingsDirName is the hidden directory storing per-drive app settings.
	SettingsDirName = ".painless"

	// DatabaseFileName is the portable SQLite database name.
	DatabaseFileName = ".media_library.db"
)

// AlbumsDir returns the absolute path to the albums directory on the target drive.
func AlbumsDir(driveRoot string) string {
	return filepath.Join(driveRoot, AlbumsDirName)
}

// UnknownDir returns the absolute path to the unknown directory on the target drive.
func UnknownDir(driveRoot string) string {
	return filepath.Join(driveRoot, UnknownDirName)
}

// ThumbnailsDir returns the absolute path to the thumbnails cache directory on the target drive.
func ThumbnailsDir(driveRoot string) string {
	return filepath.Join(driveRoot, ThumbnailsDirName)
}

// BackupsDir returns the absolute path to the backups directory on the target drive.
func BackupsDir(driveRoot string) string {
	return filepath.Join(driveRoot, BackupsDirName)
}

// SettingsDir returns the absolute path to the settings directory on the target drive.
func SettingsDir(driveRoot string) string {
	return filepath.Join(driveRoot, SettingsDirName)
}

// ScaffoldLibrary creates the standard directory hierarchy on the specified drive root.
func ScaffoldLibrary(driveRoot string) error {
	if driveRoot == "" {
		return fmt.Errorf("drive root cannot be empty")
	}

	dirs := []string{
		AlbumsDir(driveRoot),
		UnknownDir(driveRoot),
		ThumbnailsDir(driveRoot),
		BackupsDir(driveRoot),
		SettingsDir(driveRoot),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to scaffold directory %s: %w", dir, err)
		}
	}

	return nil
}

// EnsureAlbumDir ensures that a specific album directory exists within albums/.
func EnsureAlbumDir(driveRoot, albumName string) (string, error) {
	sanitized := SanitizePathSegment(albumName)
	if sanitized == "" {
		return "", fmt.Errorf("invalid album name: %q", albumName)
	}

	albumPath := filepath.Join(AlbumsDir(driveRoot), sanitized)
	if err := os.MkdirAll(albumPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create album directory %s: %w", albumPath, err)
	}

	return albumPath, nil
}
