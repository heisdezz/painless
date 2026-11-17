package wiring

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"painless/internal/domain"
	"painless/internal/state"
)

// freePort reserves an ephemeral port and releases it for the test server.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return port
}

// pingURL checks whether the sync server answers /api/ping.
func pingURL(t *testing.T, port int) bool {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/ping", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestMobileStartStopAndPing(t *testing.T) {
	root := t.TempDir()
	st := newDriveStore(t, root)
	m := NewMobile(st)
	defer m.Close()
	port := freePort(t)

	if m.URL() != "" {
		t.Fatalf("URL before start = %q, want empty", m.URL())
	}
	m.SetPort(port)
	m.SetEnabled(true)

	waitFor(t, "server ping", func() bool { return pingURL(t, port) })
	if !m.Enabled() {
		t.Fatal("Enabled = false after SetEnabled(true)")
	}
	if m.Port() != port {
		t.Fatalf("Port = %d, want %d", m.Port(), port)
	}
	if m.URL() == "" {
		t.Fatal("URL empty while running")
	}

	token := m.Token()
	if len(token) != 32 {
		t.Fatalf("Token length = %d, want 32-hex", len(token))
	}
	// Token and port persisted to the drive settings file.
	persisted := loadDriveSettings(root)
	if persisted.MobileToken != token || persisted.MobilePort != port {
		t.Fatalf("persisted settings = %+v", persisted)
	}

	// Regeneration swaps the token and keeps the server reachable.
	next := m.RegenerateToken()
	if next == token {
		t.Fatal("RegenerateToken returned the same token")
	}
	if m.Token() != next {
		t.Fatalf("Token = %q, want regenerated %q", m.Token(), next)
	}
	waitFor(t, "server restart after token rotation", func() bool { return pingURL(t, port) })

	// Stop tears the endpoint down.
	m.SetEnabled(false)
	waitFor(t, "server stop", func() bool {
		return !pingURL(t, port)
	})
	if m.URL() != "" {
		t.Fatalf("URL after stop = %q, want empty", m.URL())
	}
}

func TestMobileWithoutDriveStaysIdle(t *testing.T) {
	st := state.NewStore()
	m := NewMobile(st)
	defer m.Close()

	m.SetEnabled(true)
	if !m.Enabled() {
		t.Fatal("Enabled should reflect the requested state")
	}
	if m.URL() != "" {
		t.Fatalf("URL without drive = %q, want empty", m.URL())
	}
	if m.Port() != 51790 {
		t.Fatalf("default Port = %d, want %d", m.Port(), 51790)
	}
}

func TestMobileFollowsDriveAttach(t *testing.T) {
	root := t.TempDir()
	st := state.NewStore()
	m := NewMobile(st)
	defer m.Close()
	port := freePort(t)

	if pingURL(t, port) {
		t.Fatal("server started without an active drive")
	}

	// Attach the drive, then enable: the server follows the drive.
	st.ApplyDrives([]domain.DriveInfo{{ID: "drive-1", Path: root, Name: "Test"}})
	st.SetActiveDrive("drive-1")
	m.SetEnabled(true)
	m.SetPort(port)
	waitFor(t, "start after enable", func() bool { return pingURL(t, port) })
	token := m.Token()

	// Detach: the pump should stop the server.
	st.ApplyDrives(nil)
	waitFor(t, "auto-stop after drive detach", func() bool { return !pingURL(t, port) })

	// Re-attach: sync is still enabled for the session, so the pump
	// restarts it with the persisted token.
	st.ApplyDrives([]domain.DriveInfo{{ID: "drive-1", Path: root, Name: "Test"}})
	waitFor(t, "auto-start after drive re-attach", func() bool { return pingURL(t, port) })
	if m.Token() != token {
		t.Fatalf("token after re-attach = %q, want %q", m.Token(), token)
	}
}

func TestMobileSettingsPersistAcrossReload(t *testing.T) {
	root := t.TempDir()
	st := newDriveStore(t, root)
	m := NewMobile(st)
	defer m.Close()

	m.SetEnabled(false)
	m.SetPort(51999)
	m.RegenerateToken()

	// Simulate a fresh controller over the same drive: settings reload.
	reloaded := loadDriveSettings(root)
	if reloaded.MobilePort != 51999 {
		t.Fatalf("persisted = %+v", reloaded)
	}
	if len(reloaded.MobileToken) != 32 {
		t.Fatalf("persisted token %q", reloaded.MobileToken)
	}
	if _, err := os.Stat(filepath.Join(root, settingsDirName, settingsFileName)); err != nil {
		t.Fatalf("settings file missing: %v", err)
	}
}

func TestMobileSettingsFileIsJSON(t *testing.T) {
	root := t.TempDir()
	st := newDriveStore(t, root)
	m := NewMobile(st)
	defer m.Close()

	m.RegenerateToken()
	data, err := os.ReadFile(filepath.Join(root, settingsDirName, settingsFileName))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("settings file is not JSON: %v", err)
	}
	if _, ok := doc["mobile_token"]; !ok {
		t.Fatalf("mobile_token key missing: %s", data)
	}
}
