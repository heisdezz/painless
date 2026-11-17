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
	return ShouldIgnoreDirWith(dirName, nil)
}

// ShouldIgnoreDirWith also applies the caller's custom ignore patterns. A
// pattern matches a directory by name, case-insensitively; a trailing slash
// makes it an exact directory-name match.
func ShouldIgnoreDirWith(dirName string, patterns []string) bool {
	base := filepath.Base(dirName)
	if defaultIgnoredDirs[base] {
		return true
	}
	// Skip hidden system directories (starting with '.')
	if strings.HasPrefix(base, ".") && len(base) > 1 {
		return true
	}
	return MatchesIgnore(base, patterns)
}

// MatchesIgnore reports whether a name matches one of the custom patterns.
func MatchesIgnore(name string, patterns []string) bool {
	lower := strings.ToLower(name)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			if lower == strings.TrimSuffix(pattern, "/") {
				return true
			}
			continue
		}
		if strings.Contains(lower, pattern) {
			return true
		}
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
