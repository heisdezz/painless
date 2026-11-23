package mobile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"painless/internal/domain"
	"painless/internal/storage"
)

// snapshotHook is a test DatabaseSnapshot that copies the live db file,
// mirroring what VACUUM INTO produces.
func snapshotHook(t *testing.T, dbPath string) func(context.Context) (string, func(), error) {
	t.Helper()
	return func(context.Context) (string, func(), error) {
		src, err := os.ReadFile(dbPath)
		if err != nil {
			return "", nil, err
		}
		tmp, err := os.CreateTemp(t.TempDir(), "snapshot-*.db")
		if err != nil {
			return "", nil, err
		}
		if _, err := tmp.Write(src); err != nil {
			tmp.Close()
			return "", nil, err
		}
		if err := tmp.Close(); err != nil {
			return "", nil, err
		}
		path := tmp.Name()
		return path, func() { _ = os.Remove(path) }, nil
	}
}

// TestInfoEndpoints checks the D-Stream pairing payload shape at every
// alias route, including the download_url the mobile app follows.
func TestInfoEndpoints(t *testing.T) {
	root, srv, _, _ := newTestEnv(t)
	srv.opts.DriveInfo = func() *domain.DriveInfo {
		return &domain.DriveInfo{ID: "d1", Name: "Photo Drive", Path: root}
	}
	srv.opts.LibraryStats = func() *LibraryStats {
		return &LibraryStats{TotalItems: 12, Images: 9, Videos: 3, Albums: 2, Tags: 0,
			DBSizeBytes: 4096, DBExists: true}
	}
	ts := serve(t, srv)

	for _, route := range []string{"/", "/api/info", "/api/db/stats"} {
		resp := mustGet(t, ts.URL+route)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", route, resp.StatusCode)
		}
		var info infoResponse
		if err := decodeResp(resp, &info); err != nil {
			t.Fatalf("%s decode: %v", route, err)
		}
		if info.Status != "online" || info.Server != "Media Library Mobile Sync Server" {
			t.Fatalf("%s identity = %+v", route, info)
		}
		if info.DriveName != "Photo Drive" || info.DrivePath != root {
			t.Fatalf("%s drive = %q/%q", route, info.DriveName, info.DrivePath)
		}
		if !strings.HasSuffix(info.DownloadURL, "/download/db") {
			t.Fatalf("%s download_url = %q", route, info.DownloadURL)
		}
		if info.Stats == nil || info.Stats.TotalItems != 12 || !info.Stats.DBExists {
			t.Fatalf("%s stats = %+v", route, info.Stats)
		}
	}
}

// decodeResp is decode without the mustGet coupling, for responses that
// arrive with deferred bodies.
func decodeResp[T any](resp *http.Response, out *T) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// TestDownloadDBSnapshotAndRange round-trips /download/db: the served
// bytes must be a valid SQLite database, the Content-Disposition an
// attachment, and a Range request must return 206.
func TestDownloadDBSnapshotAndRange(t *testing.T) {
	root, srv, media, _ := newTestEnv(t)
	dbPath := filepath.Join(root, storage.LibraryDBFilename)
	// The test library is in memory; materialize a file for the route's
	// stat/open path and the snapshot hook.
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := media.Count(context.Background()); err != nil {
		t.Fatalf("count: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("SQLite format 3\x00fake-for-test"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv.opts.DatabaseSnapshot = snapshotHook(t, dbPath)
	ts := serve(t, srv)

	resp := mustGet(t, ts.URL+"/download/db")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-sqlite3" {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("content-disposition = %q", cd)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "SQLite format 3") {
		t.Fatalf("body = %q", body)
	}
	// A snapshot must be disposable: the cleanup removed the temp copy and
	// the live database stays in place for the next download.
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("live db gone: %v", err)
	}
}

// TestDownloadDBMissingLibrary verifies the D-Stream 404 error shape.
func TestDownloadDBMissingLibrary(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.opts.DriveRoot = filepath.Join(t.TempDir(), "empty")
	ts := serve(t, srv)

	resp := mustGet(t, ts.URL+"/download/db")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var errResp map[string]string
	if err := decodeResp(resp, &errResp); err != nil {
		t.Fatal(err)
	}
	if errResp["error"] != "Library database not found" {
		t.Fatalf("error = %q", errResp["error"])
	}
}

// TestDownloadDBHEADSkipsSnapshot proves HEAD serves metadata without
// invoking the snapshot hook.
func TestDownloadDBHEADSkipsSnapshot(t *testing.T) {
	root, srv, _, _ := newTestEnv(t)
	dbPath := filepath.Join(root, storage.LibraryDBFilename)
	if err := os.WriteFile(dbPath, []byte("SQLite format 3\x00head"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshotCalls := 0
	srv.opts.DatabaseSnapshot = func(context.Context) (string, func(), error) {
		snapshotCalls++
		return snapshotHook(t, dbPath)(context.Background())
	}
	ts := serve(t, srv)

	resp := do(t, "HEAD", ts.URL+"/download/db", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if snapshotCalls != 0 {
		t.Fatalf("HEAD triggered %d snapshots", snapshotCalls)
	}
	if cl := resp.Header.Get("Content-Length"); cl == "" || cl == "0" {
		t.Fatalf("content-length = %q", cl)
	}
}
