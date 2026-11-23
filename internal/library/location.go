// Package library resolves the portable library location for a media
// drive. A drive's SQLite catalog lives either at the drive root or in a
// folder the user chose in Discover; the choice is persisted in the
// drive's `.painless/library.json` so any installation — desktop UI,
// mobile sync server, backup tools — finds the same catalog. Every
// consumer of "the library for this drive" must go through this package
// instead of guessing the drive root.
package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// LocationFile lives in the drive's hidden settings directory. It
// records the folder whose SQLite catalog backs every screen and
// service for this drive.
const LocationFile = ".painless/library.json"

type storedLocation struct {
	Root string `json:"root"`
}

// LoadRoot returns the persisted library root for a drive, or "" when no
// folder has been chosen (the catalog then lives at the drive root).
// Paths outside the drive are ignored.
func LoadRoot(driveRoot string) string {
	if driveRoot == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(driveRoot, filepath.FromSlash(LocationFile)))
	if err != nil {
		return ""
	}
	var stored storedLocation
	if err := json.Unmarshal(data, &stored); err != nil || !WithinDrive(driveRoot, stored.Root) {
		return ""
	}
	return stored.Root
}

// ResolveRoot returns the effective library root: the persisted choice
// when one exists, the drive root otherwise. This is the single entry
// point services should use to find the catalog.
func ResolveRoot(driveRoot string) string {
	if root := LoadRoot(driveRoot); root != "" {
		return root
	}
	return driveRoot
}

// SaveRoot persists the chosen library location on the drive.
func SaveRoot(driveRoot, root string) error {
	if !WithinDrive(driveRoot, root) {
		return os.ErrInvalid
	}
	path := filepath.Join(driveRoot, filepath.FromSlash(LocationFile))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(storedLocation{Root: root}, "", "  ")
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

// WithinDrive reports whether path lies inside driveRoot (or is the root
// itself). Relative paths and paths outside the drive are rejected.
func WithinDrive(driveRoot, path string) bool {
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
