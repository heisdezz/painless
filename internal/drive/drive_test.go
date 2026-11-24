package drive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"painless/internal/domain"
	"painless/internal/storage"
)

func TestSystemDriveDetection(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	drives, err := svc.ListDrives(ctx)
	if err != nil {
		t.Fatalf("ListDrives failed: %v", err)
	}

	if len(drives) == 0 {
		t.Logf("no partitions detected on host (possibly container sandbox)")
	}

	for _, d := range drives {
		if d.Path == "" || d.ID == "" {
			t.Errorf("drive has empty ID or Path: %+v", d)
		}
		if d.IsSystemBoot() {
			t.Errorf("system boot drive should be filtered out: %+v", d)
		}
	}
}

func TestAddCustomMountRejectsSystemBootPath(t *testing.T) {
	svc := NewService()
	for _, p := range []string{"/boot", "/boot/efi", "/efi"} {
		if _, err := svc.AddCustomMount(p, "Boot"); err == nil {
			t.Errorf("AddCustomMount(%q) succeeded, want error", p)
		}
	}
}

func TestCustomMountManagement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "painless_custom_drive_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := NewService()

	// 1. Add custom mount
	drive, err := svc.AddCustomMount(tempDir, "Test Drive")
	if err != nil {
		t.Fatalf("AddCustomMount failed: %v", err)
	}
	if drive.Name != "Test Drive" || drive.Type != domain.DriveTypeCustom {
		t.Errorf("unexpected custom drive: %+v", drive)
	}
	if drive.HasLibrary {
		t.Errorf("expected HasLibrary to be false initially")
	}

	// 2. Create library db in directory
	dbPath := filepath.Join(tempDir, storage.LibraryDBFilename)
	if err := os.WriteFile(dbPath, []byte("sqlite-placeholder"), 0644); err != nil {
		t.Fatalf("failed to write library db: %v", err)
	}

	// Check that ListDrives detects HasLibrary
	drives, err := svc.ListDrives(context.Background())
	if err != nil {
		t.Fatalf("ListDrives failed: %v", err)
	}

	var found *domain.DriveInfo
	for _, d := range drives {
		if d.Path == tempDir {
			found = &d
			break
		}
	}
	if found == nil {
		t.Fatalf("custom drive not found in ListDrives")
	}
	if !found.HasLibrary {
		t.Errorf("expected HasLibrary to be true after creating %s", storage.LibraryDBFilename)
	}

	// 3. Remove custom mount
	if !svc.RemoveCustomMount(tempDir) {
		t.Errorf("expected RemoveCustomMount to return true")
	}
}

func TestMonitorEventEmission(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "painless_monitor_drive_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := NewService()
	monitor := NewMonitor(svc, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)
	defer monitor.Stop()

	// Add custom mount
	_, err = svc.AddCustomMount(tempDir, "Monitored Drive")
	if err != nil {
		t.Fatalf("AddCustomMount failed: %v", err)
	}

	// Expect Mount event
	select {
	case event := <-monitor.Events():
		if event.Type != domain.DriveEventMounted {
			t.Errorf("expected Mounted event, got %v", event.Type)
		}
		if event.Drive.Path != tempDir {
			t.Errorf("expected path %s, got %s", tempDir, event.Drive.Path)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for mount event")
	}

	// Remove custom mount
	svc.RemoveCustomMount(tempDir)

	// Expect Unmount event
	select {
	case event := <-monitor.Events():
		if event.Type != domain.DriveEventUnmounted {
			t.Errorf("expected Unmounted event, got %v", event.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for unmount event")
	}
}
