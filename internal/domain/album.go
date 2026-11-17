package domain

import (
	"strings"
	"time"
)

// Album represents a collection mapping to a physical directory on the drive.
type Album struct {
	ID           int64     `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	RelativePath string    `json:"relative_path" db:"relative_path"`
	Description  *string   `json:"description,omitempty" db:"description"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// AlbumSummary contains an album with its associated media count and cover preview.
type AlbumSummary struct {
	Album
	MediaCount  int        `json:"media_count"`
	TotalBytes  int64      `json:"total_bytes"`
	PreviewItem *MediaItem `json:"preview_item,omitempty"`
}

// ValidateAlbumName checks that the album name does not contain illegal filesystem characters.
func ValidateAlbumName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > 255 {
		return false
	}
	// Characters prohibited on FAT32/exFAT/NTFS/ext4 directory names
	const invalidChars = "/\\:*?\"<>|"
	for _, c := range trimmed {
		if strings.ContainsRune(invalidChars, c) {
			return false
		}
	}
	// Reserved OS directory names
	lower := strings.ToLower(trimmed)
	if lower == "." || lower == ".." || lower == "unknown" || lower == ".media_thumbnails" || lower == ".media_backups" {
		return false
	}
	return true
}

// CreateAlbumInput defines parameters for creating an album.
type CreateAlbumInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// EditAlbumInput defines parameters for editing an album.
type EditAlbumInput struct {
	ID          int64   `json:"id"`
	NewName     string  `json:"new_name"`
	Description *string `json:"description,omitempty"`
}
