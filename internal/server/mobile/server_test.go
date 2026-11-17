package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"painless/internal/domain"
	"painless/internal/fileops"
	"painless/internal/storage"
)

const testToken = "pair-secret-123"

// newTestEnv builds a server over an in-memory library and a scaffolded
// temp drive root.
func newTestEnv(t *testing.T) (root string, srv *Server, media *storage.MediaRepo, albums *storage.AlbumRepo) {
	t.Helper()

	db, err := storage.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	root = t.TempDir()
	if err := fileops.ScaffoldLibrary(root); err != nil {
		t.Fatal(err)
	}
	albums = storage.NewAlbumRepo(db)
	media = storage.NewMediaRepo(db)
	srv = New(Options{
		Token:     testToken,
		DriveRoot: root,
		Albums:    albums,
		Media:     media,
	})
	return root, srv, media, albums
}

// serve wraps the handler in an httptest server.
func serve(t *testing.T, srv *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// do issues a request with an optional bearer token and returns the
// response; the caller closes the body.
func do(t *testing.T, method, url, token string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func TestPingOpen(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	ts := serve(t, srv)

	resp, err := http.Get(ts.URL + "/api/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping status = %d", resp.StatusCode)
	}
}

func TestStatusReportsDriveAndLAN(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.opts.DriveInfo = func() *domain.DriveInfo {
		return &domain.DriveInfo{ID: "a", Name: "Vacation Drive", Path: "/media/vacation"}
	}
	ts := serve(t, srv)

	resp, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decode[statusResponse](t, resp)
	if got.Drive == nil || got.Drive.Name != "Vacation Drive" {
		t.Errorf("status drive = %+v", got.Drive)
	}
	if got.MaxUploadBytes != DefaultMaxUploadBytes {
		t.Errorf("max upload = %d", got.MaxUploadBytes)
	}
	if got.Status != "ok" {
		t.Errorf("status = %q", got.Status)
	}
}

func TestAlbumAuthAndCRUD(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	ts := serve(t, srv)

	// Writes without (or with a wrong) token are rejected.
	resp := do(t, "POST", ts.URL+"/api/albums", "", strings.NewReader(`{"name":"Trip"}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token create status = %d, want 401", resp.StatusCode)
	}
	resp = do(t, "POST", ts.URL+"/api/albums", "wrong-token", strings.NewReader(`{"name":"Trip"}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad-token create status = %d, want 401", resp.StatusCode)
	}

	// Reads are open by default.
	resp, err := http.Get(ts.URL + "/api/albums")
	if err != nil {
		t.Fatal(err)
	}
	albums := decode[[]domain.AlbumSummary](t, resp)
	if len(albums) != 0 {
		t.Errorf("initial albums = %d, want 0", len(albums))
	}

	// Create with token succeeds and shows up in the list.
	album := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken, strings.NewReader(`{"name":"Beach Trip"}`)))
	if album.ID == 0 || album.Name != "Beach Trip" {
		t.Errorf("created album = %+v", album)
	}

	albums = decode[[]domain.AlbumSummary](t, mustGet(t, ts.URL+"/api/albums"))
	if len(albums) != 1 || albums[0].Name != "Beach Trip" {
		t.Errorf("albums after create = %+v", albums)
	}

	// Invalid names are rejected.
	resp = do(t, "POST", ts.URL+"/api/albums", testToken, strings.NewReader(`{"name":"bad/name"}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid name status = %d, want 400", resp.StatusCode)
	}
}

func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestMediaListAndGet(t *testing.T) {
	_, srv, media, _ := newTestEnv(t)
	ts := serve(t, srv)

	items := []domain.MediaItem{
		{FileHash: "h1", OriginalRelativePath: "unknown/a.jpg", CurrentRelativePath: "unknown/a.jpg", FileSize: 10, MimeType: "image/jpeg"},
		{FileHash: "h2", OriginalRelativePath: "unknown/b.mp4", CurrentRelativePath: "unknown/b.mp4", FileSize: 20, MimeType: "video/mp4"},
	}
	if _, err := media.InsertBatch(context.Background(), items); err != nil {
		t.Fatal(err)
	}

	page := decode[domain.MediaListPage](t, mustGet(t, ts.URL+"/api/media?limit=1&offset=0&filter=images"))
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].FileHash != "h1" {
		t.Errorf("filtered page = %+v", page)
	}

	all := decode[domain.MediaListPage](t, mustGet(t, ts.URL+"/api/media"))
	if all.Total != 2 {
		t.Errorf("total = %d, want 2", all.Total)
	}

	item := decode[domain.MediaItem](t, mustGet(t, fmt.Sprintf("%s/api/media/%d", ts.URL, page.Items[0].ID)))
	if item.FileHash != "h1" {
		t.Errorf("fetched item = %+v", item)
	}

	resp := mustGet(t, ts.URL+"/api/media/999")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown id status = %d, want 404", resp.StatusCode)
	}
	resp = mustGet(t, ts.URL+"/api/media?filter=bogus")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad filter status = %d, want 400", resp.StatusCode)
	}
}

func TestProtectReads(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.opts.ProtectReads = true
	ts := serve(t, srv)

	resp, err := http.Get(ts.URL + "/api/albums")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("protected read without token = %d, want 401", resp.StatusCode)
	}

	// Ping stays reachable for pre-pairing checks.
	resp, err = http.Get(ts.URL + "/api/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ping under ProtectReads = %d, want 200", resp.StatusCode)
	}

	resp = do(t, "GET", ts.URL+"/api/albums", testToken, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("protected read with token = %d, want 200", resp.StatusCode)
	}
}

func TestNoTokenConfiguredFailsClosed(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.opts.Token = ""
	ts := serve(t, srv)

	resp := do(t, "POST", ts.URL+"/api/albums", "anything", strings.NewReader(`{"name":"X"}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("write with unconfigured token = %d, want 503", resp.StatusCode)
	}
}

func TestStartShutdown(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.opts.Port = 52791
	srv.opts.Bind = "127.0.0.1"
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	addr := srv.Addr()
	if addr == "" {
		t.Fatal("Addr empty after Start")
	}
	resp, err := http.Get("http://" + addr + "/api/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("second Shutdown: %v", err)
	}
	if srv.Addr() != "" {
		t.Errorf("Addr after shutdown = %q", srv.Addr())
	}
}

type playbackThumbnails struct{}

func (playbackThumbnails) Get(context.Context, domain.MediaItem) ([]byte, error) {
	return []byte("webp thumbnail"), nil
}

func TestMobilePlayback(t *testing.T) {
	root, srv, media, _ := newTestEnv(t)
	srv.opts.ProtectReads = true
	srv.opts.Thumbnails = playbackThumbnails{}
	const rel = "unknown/clip.mp4"
	if err := os.WriteFile(filepath.Join(root, rel), []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := media.InsertBatch(ctx, []domain.MediaItem{{
		FileHash: "clip-hash", OriginalRelativePath: rel, CurrentRelativePath: rel,
		FileSize: 6, MimeType: "video/mp4",
	}}); err != nil {
		t.Fatal(err)
	}
	item, err := media.GetByHash(ctx, "clip-hash")
	if err != nil || item == nil {
		t.Fatalf("lookup catalog item: %v, %v", item, err)
	}
	thumb := fmt.Sprintf("/thumbnail?mediaId=%d", item.ID)
	play := "/stream?path=" + rel
	handler := srv.Handler()
	request := func(method, target, token, byteRange string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{play, thumb} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			if got := request(method, path, "", ""); got.Code != http.StatusUnauthorized {
				t.Errorf("unauthenticated %s %s: status = %d, want 401", method, path, got.Code)
			}
		}
	}
	got := request(http.MethodGet, play, testToken, "bytes=1-3")
	if got.Code != http.StatusPartialContent || got.Body.String() != "bcd" || got.Header().Get("Content-Range") != "bytes 1-3/6" {
		t.Errorf("range response = %d %v %q", got.Code, got.Header(), got.Body.String())
	}
	got = request(http.MethodHead, play, testToken, "")
	if got.Code != http.StatusOK || got.Header().Get("Content-Length") != "6" || got.Body.Len() != 0 {
		t.Errorf("HEAD response = %d %v %q", got.Code, got.Header(), got.Body.String())
	}
	got = request(http.MethodGet, thumb, testToken, "")
	if got.Code != http.StatusOK || got.Header().Get("Content-Type") != "image/webp" || got.Body.String() != "webp thumbnail" {
		t.Errorf("thumbnail response = %d %v %q", got.Code, got.Header(), got.Body.String())
	}

	srv.SetDriveRoot(t.TempDir())
	if got := request(http.MethodGet, play, testToken, ""); got.Code != http.StatusNotFound {
		t.Errorf("after drive switch: status = %d, want 404", got.Code)
	}
	srv.SetDriveRoot("")
	if got := request(http.MethodGet, play, testToken, ""); got.Code != http.StatusServiceUnavailable {
		t.Errorf("without a drive: status = %d, want 503", got.Code)
	}
}

func TestLANAddress(t *testing.T) {
	lan, err := LANAddress(DefaultPort)
	if err != nil {
		t.Skipf("no LAN address on this host: %v", err)
	}
	host, port, err := net.SplitHostPort(lan)
	if err != nil {
		t.Fatalf("LANAddress = %q: %v", lan, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() {
		t.Errorf("LANAddress host = %q", host)
	}
	if port != fmt.Sprint(DefaultPort) {
		t.Errorf("LANAddress port = %q", port)
	}
}
