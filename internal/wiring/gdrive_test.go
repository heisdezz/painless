package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"painless/internal/domain"
	"painless/internal/gdrive"
	"painless/internal/state"
)

// fakeDriveClient records calls and returns canned results.
type fakeDriveClient struct {
	mu          sync.Mutex
	testErr     error
	testCalls   int
	lastFolder  string
	backupRes   *gdrive.BackupResult
	backupErr   error
	backupCalls int
}

func (f *fakeDriveClient) TestConnection(ctx context.Context, folderID string) (*gdrive.AccountInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.testCalls++
	f.lastFolder = folderID
	if f.testErr != nil {
		return nil, f.testErr
	}
	return &gdrive.AccountInfo{Email: "svc@test.iam"}, nil
}

func (f *fakeDriveClient) BackupDatabase(ctx context.Context, snapshotter gdrive.DatabaseSnapshotter,
	targetFolderID, tempDir string, onProgress func(written, total int64)) (*gdrive.BackupResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.backupCalls++
	f.lastFolder = targetFolderID
	if f.backupErr != nil {
		return nil, f.backupErr
	}
	// Exercise the snapshotter so VACUUM INTO runs against the live db.
	dir := tempDir
	if dir == "" {
		var err error
		if dir, err = os.MkdirTemp("", "wiring-fake-*"); err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)
	}
	if err := snapshotter.CreateBackup(filepath.Join(dir, "snapshot.db")); err != nil {
		return nil, err
	}
	return &gdrive.BackupResult{FileID: "f1", FileName: "snapshot.db", Size: 1536}, nil
}

// installFake replaces the client factory and returns the shared fake.
func installFake(g *GDrive) *fakeDriveClient {
	fake := &fakeDriveClient{backupRes: &gdrive.BackupResult{}}
	g.newClient = func(ctx context.Context, credentials []byte) (driveClient, error) {
		if len(credentials) == 0 {
			return nil, errors.New("empty credentials")
		}
		return fake, nil
	}
	return fake
}

func TestGDriveCredentialsPersist(t *testing.T) {
	root := t.TempDir()
	g := NewGDrive(newDriveStore(t, root))
	defer g.Close()

	if json, folder := g.Credentials(); json != "" || folder != "" {
		t.Fatalf("initial credentials = (%q, %q)", json, folder)
	}
	g.SetCredentials(`{"client_email":"svc@test.iam"}`, "folder-1")
	if json, folder := g.Credentials(); json == "" || folder != "folder-1" {
		t.Fatalf("credentials after set = (%q, %q)", json, folder)
	}
	persisted := loadDriveSettings(root)
	if persisted.GDriveFolderID != "folder-1" || persisted.GDriveCredentials == "" {
		t.Fatalf("persisted = %+v", persisted)
	}
}

func TestGDriveTestConnectionFlow(t *testing.T) {
	root := t.TempDir()
	g := NewGDrive(newDriveStore(t, root))
	defer g.Close()
	fake := installFake(g)

	if err := g.TestConnection(); err == nil {
		t.Fatal("expected error without credentials")
	}
	g.SetCredentials(`{"client_email":"svc@test.iam"}`, "folder-9")
	if err := g.TestConnection(); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if fake.lastFolder != "folder-9" {
		t.Fatalf("folder passed = %q", fake.lastFolder)
	}

	fake.mu.Lock()
	fake.testErr = errors.New("quota exceeded")
	fake.mu.Unlock()
	if err := g.TestConnection(); err == nil || err.Error() != "quota exceeded" {
		t.Fatalf("TestConnection error = %v", err)
	}
}

func TestGDriveBackupNow(t *testing.T) {
	root := t.TempDir()
	seedLibrary(t, root).Close()
	g := NewGDrive(newDriveStore(t, root))
	defer g.Close()
	fake := installFake(g)

	if _, err := g.BackupNow(); err == nil {
		t.Fatal("expected error without credentials")
	}
	g.SetCredentials(`{"client_email":"svc@test.iam"}`, "folder-1")
	msg, err := g.BackupNow()
	if err != nil {
		t.Fatalf("BackupNow: %v", err)
	}
	if msg != fmt.Sprintf("Uploaded %s (%s)", "snapshot.db", formatBytes(1536)) {
		t.Fatalf("message = %q", msg)
	}
	if fake.backupCalls != 1 {
		t.Fatalf("backup calls = %d", fake.backupCalls)
	}

	// Without an active drive the backup fails before touching clients.
	st := state.NewStore()
	g2 := NewGDrive(st)
	defer g2.Close()
	installFake(g2)
	g2.SetCredentials(`{}`, "")
	if _, err := g2.BackupNow(); err == nil {
		t.Fatal("expected error without active drive")
	}
}

func TestGDriveClientCachedPerCredentials(t *testing.T) {
	root := t.TempDir()
	g := NewGDrive(newDriveStore(t, root))
	defer g.Close()
	fake := installFake(g)

	g.SetCredentials(`{"a":1}`, "")
	if err := g.TestConnection(); err != nil {
		t.Fatalf("first test: %v", err)
	}
	if err := g.TestConnection(); err != nil {
		t.Fatalf("second test: %v", err)
	}
	fake.mu.Lock()
	calls := fake.testCalls
	fake.mu.Unlock()
	_ = calls // factory-level caching is asserted via clientFor below

	g.mu.Lock()
	cached := g.client != nil && g.clientKey == `{"a":1}`
	g.mu.Unlock()
	if !cached {
		t.Fatal("client not cached for unchanged credentials")
	}

	// Changing credentials drops the cache.
	g.SetCredentials(`{"a":2}`, "")
	g.mu.Lock()
	cached = g.client == nil
	g.mu.Unlock()
	if !cached {
		t.Fatal("client cache not dropped on credential change")
	}
}

func TestGDriveFollowsDriveSwitch(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	st := newDriveStore(t, rootA)
	g := NewGDrive(st)
	defer g.Close()
	installFake(g)

	g.SetCredentials(`{"drive":"a"}`, "folder-a")

	// Switch to a drive with its own saved credentials.
	cfgB := driveSettings{GDriveCredentials: `{"drive":"b"}`, GDriveFolderID: "folder-b"}
	if err := cfgB.save(rootB); err != nil {
		t.Fatalf("save drive B settings: %v", err)
	}
	st.ApplyDrives([]domain.DriveInfo{
		{ID: "drive-1", Path: rootA, Name: "A"},
		{ID: "drive-2", Path: rootB, Name: "B"},
	})
	st.SetActiveDrive("drive-2")

	// The pump reloads async; poll until the new credentials surface.
	deadline := loop(2*time.Second, func() bool {
		json, folder := g.Credentials()
		return json == `{"drive":"b"}` && folder == "folder-b"
	})
	if !deadline {
		t.Fatalf("credentials after switch = (%q, %q)", jsonOf(g), folderOf(g))
	}

	// And back to drive A.
	st.SetActiveDrive("drive-1")
	deadline = loop(2*time.Second, func() bool {
		json, _ := g.Credentials()
		return json == `{"drive":"a"}`
	})
	if !deadline {
		t.Fatalf("credentials after switch back = %q", jsonOf(g))
	}
}

// loop polls cond until it passes or the timeout elapses.
func loop(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func jsonOf(g *GDrive) string {
	json, _ := g.Credentials()
	return json
}

func folderOf(g *GDrive) string {
	_, folder := g.Credentials()
	return folder
}
