package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// libraryLocationFile lives next to the drive's portable settings. It records
// the folder whose SQLite catalog backs every screen for this drive.
const libraryLocationFile = ".painless/library.json"

type storedLibraryLocation struct {
	Root string `json:"root"`
}

// loadLibraryRoot returns the persisted library root, or "" when no folder
// has been chosen. Paths outside the drive are ignored.
func loadLibraryRoot(driveRoot string) string {
	if driveRoot == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(driveRoot, filepath.FromSlash(libraryLocationFile)))
	if err != nil {
		return ""
	}
	var stored storedLibraryLocation
	if err := json.Unmarshal(data, &stored); err != nil || !isWithinDrive(driveRoot, stored.Root) {
		return ""
	}
	return stored.Root
}

// saveLibraryRoot persists the chosen library location on the drive.
func saveLibraryRoot(driveRoot, root string) error {
	if !isWithinDrive(driveRoot, root) {
		return os.ErrInvalid
	}
	path := filepath.Join(driveRoot, filepath.FromSlash(libraryLocationFile))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(storedLibraryLocation{Root: root}, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".library-*.json")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func isWithinDrive(driveRoot, path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	root := filepath.Clean(driveRoot)
	clean := filepath.Clean(path)
	if clean == root {
		return true
	}
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
