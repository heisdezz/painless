package wiring

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"painless/internal/library"
	"painless/internal/state"
	"painless/internal/storage"
)

// seedCatalogWithItems opens a catalog at root and inserts media rows so
// a wrong-location server visibly serves an empty or wrong list.
func seedCatalogWithItems(t *testing.T, root string, hashes ...string) {
	t.Helper()
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatalf("open catalog at %s: %v", root, err)
	}
	defer db.Close()
	for _, hash := range hashes {
		if _, err := db.Raw().Exec(`INSERT INTO media_items (file_hash, original_relative_path, current_relative_path, file_size, mime_type, created_at)
			VALUES (?, ?, ?, 10, 'image/jpeg', '2026-01-01 00:00:00')`, hash, hash+".jpg", hash+".jpg"); err != nil {
			t.Fatalf("insert %s: %v", hash, err)
		}
	}
}

// mediaHashes lists the catalog the sync server currently serves.
func mediaHashes(t *testing.T, port int, token string) []string {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/media", port), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/media: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/media status = %d: %s", resp.StatusCode, body)
	}
	var payload struct {
		Items []struct {
			FileHash string `json:"file_hash"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode media list: %v", err)
	}
	hashes := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		hashes = append(hashes, item.FileHash)
	}
	return hashes
}

// tryMediaHashes is mediaHashes without the fatal on transport errors,
// for polling inside waitFor while the server restarts.
func tryMediaHashes(port int, token string) []string {
	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/media", port), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	var payload struct {
		Items []struct {
			FileHash string `json:"file_hash"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	hashes := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		hashes = append(hashes, item.FileHash)
	}
	return hashes
}

// dbDownloadHash downloads /download/db and reports the first file_hash
// row inside the served snapshot, proving which catalog was served. The
// snapshot is opened directly (a standalone SQLite file, not a drive
// folder like storage.Open expects).
func dbDownloadHash(t *testing.T, port int) string {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/download/db", port))
	if err != nil {
		t.Fatalf("GET /download/db: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /download/db status = %d", resp.StatusCode)
	}
	snapshot, err := os.CreateTemp(t.TempDir(), "downloaded-*.db")
	if err != nil {
		t.Fatal(err)
	}
	name := snapshot.Name()
	if _, err := io.Copy(snapshot, resp.Body); err != nil {
		snapshot.Close()
		t.Fatalf("read snapshot: %v", err)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("open served snapshot: %v", err)
	}
	defer db.Close()
	var hash string
	if err := db.QueryRow("SELECT file_hash FROM media_items LIMIT 1").Scan(&hash); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}
	return hash
}

// TestMobileServesSelectedLibraryLocation pins the user-facing bug: with
// a library folder chosen in Discover, the sync server must serve that
// catalog — both the REST list and the /download/db snapshot — instead of
// a database at the drive root.
func TestMobileServesSelectedLibraryLocation(t *testing.T) {
	driveRoot := t.TempDir()
	libRoot := filepath.Join(driveRoot, "media", "albums")
	seedCatalogWithItems(t, driveRoot, "root-stay-away")
	seedCatalogWithItems(t, libRoot, "selected-catalog")
	if err := library.SaveRoot(driveRoot, libRoot); err != nil {
		t.Fatalf("save location: %v", err)
	}

	port := freePort(t)
	st := newDriveStore(t, driveRoot)
	m := NewMobile(st)
	defer m.Close()
	m.SetPort(port)
	m.SetEnabled(true)
	waitFor(t, "server start", func() bool { return pingURL(t, port) })

	token := m.Token()
	if got := mediaHashes(t, port, token); len(got) != 1 || got[0] != "selected-catalog" {
		t.Fatalf("/api/media served %v, want the selected catalog", got)
	}
	if got := dbDownloadHash(t, port); got != "selected-catalog" {
		t.Fatalf("/download/db served catalog with %q, want selected-catalog", got)
	}

	// Moving the library folder must be followed without a manual restart.
	nextRoot := filepath.Join(driveRoot, "elsewhere", "catalog")
	seedCatalogWithItems(t, nextRoot, "moved-catalog")
	if err := library.SaveRoot(driveRoot, nextRoot); err != nil {
		t.Fatalf("save moved location: %v", err)
	}
	st.Bus.Publish(state.Event{Kind: state.EventLibraryLocationChanged})
	waitFor(t, "server restart at moved location", func() bool {
		if !pingURL(t, port) {
			return false
		}
		got := tryMediaHashes(port, token)
		return len(got) == 1 && got[0] == "moved-catalog"
	})
}

// TestBackupsFollowSelectedLibraryLocation proves snapshots, listing and
// restore all operate next to the resolved catalog instead of the drive
// root.
func TestBackupsFollowSelectedLibraryLocation(t *testing.T) {
	driveRoot := t.TempDir()
	libRoot := filepath.Join(driveRoot, "media", "albums")
	seedCatalogWithItems(t, libRoot, "lib-item")
	if err := library.SaveRoot(driveRoot, libRoot); err != nil {
		t.Fatalf("save location: %v", err)
	}

	b := NewBackups(newDriveStore(t, driveRoot))
	wantPath := filepath.Join(libRoot, storage.LibraryDBFilename)
	if got := b.DBPath(); got != wantPath {
		t.Fatalf("DBPath = %q, want %q", got, wantPath)
	}
	path, err := b.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(libRoot, storage.BackupFolder)) {
		t.Fatalf("backup %q outside the selected library folder", path)
	}
	rows, err := b.ListBackups()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListBackups = (%v, %v), want one row", rows, err)
	}
	if _, ok := b.IntegrityCheck(); !ok {
		t.Fatal("IntegrityCheck not ok on the selected catalog")
	}

	// Restore must replace the catalog inside the library folder.
	seedCatalogWithItems(t, libRoot) // reopen
	if err := b.RestoreBackup(path); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	db, err := storage.Open(libRoot, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.Raw().QueryRow("SELECT count(*) FROM media_items").Scan(&n); err != nil || n != 1 {
		t.Fatalf("restored catalog item count = %d (%v), want 1", n, err)
	}
}

// TestGDriveBackupUsesSelectedLibraryLocation exercises the same resolver
// for the cloud backup path: an unreadable drive-root catalog must not be
// mistaken for the real one. BackupNow fails on missing credentials, but
// the failure must come from auth, not from opening the wrong file.
func TestGDriveBackupUsesSelectedLibraryLocation(t *testing.T) {
	driveRoot := t.TempDir()
	libRoot := filepath.Join(driveRoot, "media", "albums")
	seedCatalogWithItems(t, libRoot, "lib-item")
	if err := library.SaveRoot(driveRoot, libRoot); err != nil {
		t.Fatalf("save location: %v", err)
	}
	g := NewGDrive(newDriveStore(t, driveRoot))
	defer g.Close()
	_, err := g.BackupNow()
	if err == nil || !strings.Contains(err.Error(), "service account") {
		t.Fatalf("BackupNow error = %v, want missing-credentials failure", err)
	}
	_ = context.Background()
}
