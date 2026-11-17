package drive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"painless/internal/domain"
	"painless/internal/storage"

	"github.com/shirou/gopsutil/v4/disk"
)

// Service provides volume inspection, library detection, and custom mount management.
type Service struct {
	mu           sync.RWMutex
	customMounts map[string]domain.DriveInfo
}

// NewService creates a new drive detection service.
func NewService() *Service {
	return &Service{
		customMounts: make(map[string]domain.DriveInfo),
	}
}

// AddCustomMount mounts a user-specified folder as a custom virtual drive.
func (s *Service) AddCustomMount(path string, label string) (*domain.DriveInfo, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	fi, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", cleanPath)
	}

	if label == "" {
		label = filepath.Base(cleanPath)
	}

	// Read usage statistics
	usage, err := disk.Usage(cleanPath)
	var total, free uint64
	var usedPct float64
	var fstype string
	if err == nil && usage != nil {
		total = usage.Total
		free = usage.Free
		usedPct = usage.UsedPercent
		fstype = usage.Fstype
	}

	hasLib := checkHasLibrary(cleanPath)

	drive := domain.DriveInfo{
		ID:             "custom:" + cleanPath,
		Name:           label,
		Type:           domain.DriveTypeCustom,
		Path:           cleanPath,
		FileSystem:     fstype,
		TotalBytes:     total,
		FreeBytes:      free,
		UsedPercentage: usedPct,
		Status:         domain.DriveStatusMounted,
		HasLibrary:     hasLib,
	}

	s.mu.Lock()
	s.customMounts[cleanPath] = drive
	s.mu.Unlock()

	return &drive, nil
}

// RemoveCustomMount unmounts a previously added custom directory.
func (s *Service) RemoveCustomMount(path string) bool {
	cleanPath := filepath.Clean(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.customMounts[cleanPath]; ok {
		delete(s.customMounts, cleanPath)
		return true
	}
	return false
}

// ListDrives lists all system removable drives and user-added custom mounts.
func (s *Service) ListDrives(ctx context.Context) ([]domain.DriveInfo, error) {
	drives, err := detectSystemDrives(ctx)
	if err != nil {
		drives = nil // fallback to custom mounts only if system detect fails
	}

	// Append custom user mounts
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, custom := range s.customMounts {
		// Refresh dynamic status and library presence
		custom.HasLibrary = checkHasLibrary(custom.Path)
		if usage, err := disk.Usage(custom.Path); err == nil && usage != nil {
			custom.TotalBytes = usage.Total
			custom.FreeBytes = usage.Free
			custom.UsedPercentage = usage.UsedPercent
		}
		drives = append(drives, custom)
	}

	return drives, nil
}

// checkHasLibrary reports whether .media_library.db exists at root.
func checkHasLibrary(rootPath string) bool {
	dbPath := filepath.Join(rootPath, storage.LibraryDBFilename)
	info, err := os.Stat(dbPath)
	return err == nil && !info.IsDir()
}
