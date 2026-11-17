package scanner

import (
	"path/filepath"
	"strings"
)

var defaultIgnoredDirs = map[string]bool{
	".git":                      true,
	".svn":                      true,
	".hg":                       true,
	"node_modules":              true,
	"System Volume Information": true,
	"$RECYCLE.BIN":              true,
	".Trashes":                  true,
	".Spotlight-V100":           true,
	".fseventsd":                true,
	".media_thumbnails":         true,
	".media_backups":            true,
	"lost+found":                true,
}

var defaultIgnoredFiles = map[string]bool{
	".media_library.db":     true,
	".media_library.db-wal": true,
	".media_library.db-shm": true,
	".DS_Store":             true,
	"Thumbs.db":             true,
	"desktop.ini":           true,
}

// ShouldIgnoreDir returns true if a directory should be skipped by the scanner.
func ShouldIgnoreDir(dirName string) bool {
	base := filepath.Base(dirName)
	if defaultIgnoredDirs[base] {
		return true
	}
	// Skip hidden system directories (starting with '.')
	if strings.HasPrefix(base, ".") && len(base) > 1 {
		return true
	}
	return false
}

// ShouldIgnoreFile returns true if a file should be skipped by the scanner.
func ShouldIgnoreFile(fileName string) bool {
	base := filepath.Base(fileName)
	if defaultIgnoredFiles[base] {
		return true
	}
	// Skip AppleDouble shadow files (e.g. ._IMG_1234.jpg)
	if strings.HasPrefix(base, "._") {
		return true
	}
	// Skip hidden files
	if strings.HasPrefix(base, ".") {
		return true
	}
	return false
}
