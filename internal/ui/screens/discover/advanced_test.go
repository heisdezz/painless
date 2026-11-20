package discover

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/fileops"
	"painless/internal/scanner"
)

func TestIgnoreListPresetsAndReset(t *testing.T) {
	screen := New(Options{})
	tt := mui.NewTester(screen.View, 1000, 800)
	if !tt.HasText("Folder & Path Ignorelist") || !tt.HasText("node_modules/") {
		t.Fatalf("ignorelist texts = %q", tt.Texts())
	}

	if err := tt.Click("+ temp"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("+ cache"); err != nil {
		t.Fatal(err)
	}
	if len(screen.ignoreRules) != 2 {
		t.Fatalf("rules after presets = %v", screen.ignoreRules)
	}

	screen.ignoreInput = "custom-tmp"
	tt.Frame()
	if err := tt.Click("+ Add"); err != nil {
		t.Fatal(err)
	}
	if len(screen.ignoreRules) != 3 || screen.ignoreRules[2] != "custom-tmp" {
		t.Fatalf("rules after add = %v", screen.ignoreRules)
	}

	if err := tt.Click("Reset Defaults"); err != nil {
		t.Fatal(err)
	}
	if len(screen.ignoreRules) != 0 {
		t.Fatalf("rules after reset = %v", screen.ignoreRules)
	}
}

func TestScanTargetPickerUpdatesPath(t *testing.T) {
	root := t.TempDir()
	chosen := filepath.Join(root, "picks")
	if err := os.MkdirAll(chosen, 0o755); err != nil {
		t.Fatal(err)
	}
	var gotDefault string
	updates := make(chan func(), 8)
	screen := New(Options{
		DriveRoot: root,
		DirectoryPicker: func(def string) (string, error) {
			gotDefault = def
			return chosen, nil
		},
		Update: func(apply func()) { updates <- apply },
	})
	defer func() { <-screen.Close() }()

	tt := mui.NewTester(screen.View, 1000, 800)
	if err := tt.Click("Pick location"); err != nil {
		t.Fatal(err)
	}
	select {
	case apply := <-updates:
		apply()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the screen update")
	}
	if got := screen.TargetPath(); got != chosen {
		t.Fatalf("target path = %q, want %q", got, chosen)
	}
	if gotDefault != root {
		t.Fatalf("picker default = %q, want %q", gotDefault, root)
	}
}

func TestScannerStatusPanel(t *testing.T) {
	screen := New(Options{CatalogCount: func() int { return 15278 }})
	tt := mui.NewTester(screen.View, 1000, 800)
	for _, text := range []string{"Scanner Status", "IDLE", "Active Ignore Rules", "SQLite Cataloged"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %q", text, tt.Texts())
		}
	}
	if !tt.HasText("15,278") {
		t.Fatalf("catalog count missing: %q", tt.Texts())
	}

	screen.isScanning = true
	screen.scanProg = scanner.Progress{IsRunning: true, FilesScanned: 1234}
	screen.ignoreRules = []string{"temp"}
	tt.Frame()
	if !tt.HasText("SCANNING") || !tt.HasText("1,234") || !tt.HasText("5 rules") {
		t.Fatalf("scanning texts = %q", tt.Texts())
	}

	screen.isPaused = true
	tt.Frame()
	if !tt.HasText("PAUSED") {
		t.Fatalf("paused texts = %q", tt.Texts())
	}
}

func TestAlbumFolderCardRescanAndPicker(t *testing.T) {
	root := t.TempDir()
	chosen := filepath.Join(root, "media", "albums-2")
	if err := os.MkdirAll(chosen, 0755); err != nil {
		t.Fatal(err)
	}
	var scans []scanner.Config
	updates := make(chan func(), 16)
	screen := New(Options{
		DriveRoot: root,
		AlbumScannerFactory: func(cfg scanner.Config, _ func(scanner.Progress)) Scanner {
			scans = append(scans, cfg)
			return &mockScanner{}
		},
		DirectoryPicker: func(string) (string, error) { return chosen, nil },
		Update:          func(apply func()) { updates <- apply },
	})
	defer func() { <-screen.Close() }()
	applyUpdate := func() {
		t.Helper()
		select {
		case apply := <-updates:
			apply()
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for the screen update")
		}
	}
	tt := mui.NewTester(screen.View, 1000, 800)
	if !tt.HasText("Album folder") || !tt.HasText(filepath.Join(root, "media", "albums")) {
		t.Fatalf("album folder texts = %q", tt.Texts())
	}

	if err := tt.Click("Choose folder"); err != nil {
		t.Fatal(err)
	}
	applyUpdate() // The folder picker result starts indexing.
	if got := screen.AlbumFolder(); got != chosen {
		t.Fatalf("album folder = %q, want %q", got, chosen)
	}
	if len(scans) != 1 || scans[0].TargetDir != chosen || scans[0].DriveRoot != root {
		t.Fatalf("scan after choosing folder = %+v (closed=%v scanning=%v executing=%v factory=%v err=%q)", scans, screen.closed, screen.isScanning, screen.isExecuting, screen.albumScannerFactory != nil, screen.lastScanErr)
	}
	for screen.IsScanning() {
		applyUpdate()
	}
	tt.Frame()
	if !tt.HasText(chosen) {
		t.Fatalf("chosen folder not rendered: %q", tt.Texts())
	}

	if err := tt.Click("Rescan album folder"); err != nil {
		t.Fatal(err)
	}
	if len(scans) != 2 || scans[1].TargetDir != chosen || scans[1].DriveRoot != root {
		t.Fatalf("scan after rescanning folder = %+v", scans)
	}
	if got := screen.TargetPath(); got != root {
		t.Fatalf("discovery path changed to %q", got)
	}
}

func TestScaffoldBannerTracksRealFolders(t *testing.T) {
	root := t.TempDir()
	screen := New(Options{DriveRoot: root})
	tt := mui.NewTester(screen.View, 1000, 800)
	if !tt.HasText("Missing library folders") {
		t.Fatalf("expected a missing-folder banner, got %q", tt.Texts())
	}

	// Existing managed folders and the album source must silence the banner.
	if err := fileops.ScaffoldLibrary(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "media", "albums"), 0o755); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Missing library folders") {
		t.Fatalf("banner still shown for existing folders: %q", tt.Texts())
	}
}

func TestIgnorePatternMatching(t *testing.T) {
	if !scanner.MatchesIgnore("TempFiles", []string{"temp"}) {
		t.Fatal("substring pattern did not match")
	}
	if scanner.MatchesIgnore("photos", []string{"temp", "cache"}) {
		t.Fatal("unrelated pattern matched")
	}
	if !scanner.ShouldIgnoreDirWith("albums", []string{"albums/"}) {
		t.Fatal("exact pattern did not match")
	}
	if !scanner.ShouldIgnoreDir(".hidden") {
		t.Fatal("protected hidden rule did not match")
	}
}
