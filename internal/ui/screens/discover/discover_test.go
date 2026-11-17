package discover

import (
	"context"
	"sync"
	"testing"
	"time"

	"painless/internal/domain"
	"painless/internal/fileops"
	"painless/internal/scanner"
)

type mockScanner struct {
	mu      sync.Mutex
	running bool
	paused  bool
	stopped bool
	prog    scanner.Progress
}

func (m *mockScanner) Scan(ctx context.Context) error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	for i := 1; i <= 5; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			m.mu.Lock()
			m.prog.FilesScanned = i * 10
			m.prog.MediaFound = i * 5
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *mockScanner) Pause() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = true
}

func (m *mockScanner) Resume() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = false
}

func (m *mockScanner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	m.running = false
}

func (m *mockScanner) Status() scanner.Progress {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prog
}

func TestDiscoverScreen_InitAndStaging(t *testing.T) {
	stager := fileops.NewStagingEngine()

	redrawCount := 0
	screen := New(Options{
		Stager: stager,
		RefreshWindow: func() {
			redrawCount++
		},
	})

	screen.SetTargetPath("/media/usb")
	if screen.TargetPath() != "/media/usb" {
		t.Errorf("expected target path /media/usb, got %s", screen.TargetPath())
	}

	if screen.StagedCount() != 0 {
		t.Errorf("expected 0 staged items initially")
	}

	// Stage an item
	item := domain.MediaItem{
		ID:                  101,
		FileHash:            "hash101",
		CurrentRelativePath: "photos/sample.jpg",
	}
	screen.StageItem(item, "Vacation")

	if screen.StagedCount() != 1 {
		t.Fatalf("expected 1 staged item, got %d", screen.StagedCount())
	}

	// Batch assign to unknown
	screen.batchAssignUnknown()
	ops := stager.ListOperations()
	if len(ops) != 1 || ops[0].TargetAlbumName != "" {
		t.Errorf("expected item to be assigned to unknown")
	}

	// Clear staging
	screen.ClearStaging()
	if screen.StagedCount() != 0 {
		t.Errorf("expected 0 staged items after clear")
	}
}

func TestDiscoverScreen_ScannerControls(t *testing.T) {
	mScanner := &mockScanner{}

	screen := New(Options{
		ScannerFactory: func(cfg scanner.Config, onProgress func(scanner.Progress)) Scanner {
			return mScanner
		},
	})

	screen.SetTargetPath("/test/path")

	// Start scan
	screen.startScan()
	if !screen.IsScanning() {
		t.Errorf("expected isScanning=true after startScan")
	}

	// Pause
	screen.pauseScan()
	if !screen.isPaused {
		t.Errorf("expected isPaused=true after pauseScan")
	}

	// Resume
	screen.resumeScan()
	if screen.isPaused {
		t.Errorf("expected isPaused=false after resumeScan")
	}

	// Stop
	screen.stopScan()
	if screen.IsScanning() {
		t.Errorf("expected isScanning=false after stopScan")
	}
}

func TestDiscoverScreen_ConsolidationExecution(t *testing.T) {
	stager := fileops.NewStagingEngine()
	screen := New(Options{
		Stager: stager,
	})
	screen.SetTargetPath("/tmp/drive")

	item := domain.MediaItem{
		ID:                  201,
		FileHash:            "hash201",
		CurrentRelativePath: "in/pic.jpg",
	}
	screen.StageItem(item, "Trips")

	if screen.StagedCount() != 1 {
		t.Fatalf("expected 1 staged item")
	}

	screen.runConsolidation()

	// Wait for background consolidation goroutine to finish
	for i := 0; i < 50; i++ {
		screen.mu.RLock()
		done := screen.execFinished
		screen.mu.RUnlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	screen.mu.RLock()
	defer screen.mu.RUnlock()
	if !screen.execFinished {
		t.Fatalf("expected consolidation to finish")
	}
	if screen.isExecuting {
		t.Errorf("expected isExecuting to be false")
	}
}
