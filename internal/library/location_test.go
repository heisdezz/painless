package library

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates parent directories and writes data, for tests that
// need to place files without going through the package API.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func TestLoadRootMissingFile(t *testing.T) {
	root := t.TempDir()
	if got := LoadRoot(root); got != "" {
		t.Fatalf("LoadRoot without location file = %q, want empty", got)
	}
	if got := ResolveRoot(root); got != root {
		t.Fatalf("ResolveRoot without location file = %q, want drive root", got)
	}
}

func TestLoadRootIgnoresOutsidePaths(t *testing.T) {
	drive := t.TempDir()
	outside := t.TempDir()
	if err := SaveRoot(drive, outside); err == nil {
		t.Fatal("SaveRoot accepted a path outside the drive")
	}
	// Corrupt or foreign JSON must also yield "".
	if err := writeFile(filepath.Join(drive, ".painless", "library.json"), []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	if got := LoadRoot(drive); got != "" {
		t.Fatalf("LoadRoot with corrupt file = %q, want empty", got)
	}
}

func TestSaveAndResolveRoot(t *testing.T) {
	drive := t.TempDir()
	libRoot := filepath.Join(drive, "media", "albums")
	if err := SaveRoot(drive, libRoot); err != nil {
		t.Fatalf("SaveRoot: %v", err)
	}
	if got := LoadRoot(drive); got != libRoot {
		t.Fatalf("LoadRoot = %q, want %q", got, libRoot)
	}
	if got := ResolveRoot(drive); got != libRoot {
		t.Fatalf("ResolveRoot = %q, want %q", got, libRoot)
	}

	// The root itself is a valid choice.
	if err := SaveRoot(drive, drive); err != nil {
		t.Fatalf("SaveRoot drive root: %v", err)
	}
	if got := LoadRoot(drive); got != drive {
		t.Fatalf("LoadRoot after choosing root = %q, want %q", got, drive)
	}
}

func TestWithinDrive(t *testing.T) {
	drive := t.TempDir()
	inside := filepath.Join(drive, "media", "albums")
	cases := []struct {
		path string
		want bool
	}{
		{drive, true},
		{inside, true},
		{filepath.Join(drive, "..", "elsewhere"), false},
		{"relative/path", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := WithinDrive(drive, tc.path); got != tc.want {
			t.Fatalf("WithinDrive(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
