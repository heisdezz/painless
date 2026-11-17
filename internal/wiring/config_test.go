package wiring

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDriveSettingsRoundTrip(t *testing.T) {
	root := t.TempDir()
	if cfg := loadDriveSettings(root); cfg.MobileToken != "" || cfg.MobilePort != 0 {
		t.Fatalf("expected defaults for missing file, got %+v", cfg)
	}

	want := driveSettings{
		MobileToken:       "abc123",
		MobilePort:        52000,
		GDriveCredentials: `{"client_email":"svc@test.iam"}`,
		GDriveFolderID:    "folder-1",
	}
	if err := want.save(root); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := loadDriveSettings(root)
	if got != want {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
	if path := settingsPath(root); filepath.Dir(path) != filepath.Join(root, settingsDirName) {
		t.Fatalf("unexpected settings path %q", path)
	}
}

func TestDriveSettingsSaveAtomic(t *testing.T) {
	root := t.TempDir()
	cfg := driveSettings{MobileToken: "tok"}
	if err := cfg.save(root); err != nil {
		t.Fatalf("save: %v", err)
	}
	cfg.MobileToken = "tok2"
	if err := cfg.save(root); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got := loadDriveSettings(root)
	if got.MobileToken != "tok2" {
		t.Fatalf("token = %q, want tok2", got.MobileToken)
	}
	entries, err := os.ReadDir(filepath.Join(root, settingsDirName))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp*" || filepath.Base(e.Name()) != settingsFileName {
			t.Fatalf("leftover temp file %q", e.Name())
		}
	}
}

func TestDriveSettingsCorruptFileYieldsDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, settingsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := loadDriveSettings(root)
	if cfg != (driveSettings{}) {
		t.Fatalf("expected zero config for corrupt file, got %+v", cfg)
	}
}

func TestDriveSettingsSaveWithoutDriveFails(t *testing.T) {
	var cfg driveSettings
	if err := cfg.save(""); err == nil {
		t.Fatal("expected error saving with empty drive root")
	}
}

func TestNewToken(t *testing.T) {
	a, b := newToken(), newToken()
	if a == b {
		t.Fatal("expected distinct tokens")
	}
	if len(a) != 32 { // 16 bytes hex-encoded
		t.Fatalf("token length = %d, want 32", len(a))
	}
}
