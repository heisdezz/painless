package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"painless/internal/domain"
)

// fakeResolver answers lookups from a static map.
type fakeResolver struct {
	byPath map[string]*domain.MediaItem
	byID   map[int64]*domain.MediaItem
}

func (f *fakeResolver) GetByPath(_ context.Context, rel string) (*domain.MediaItem, error) {
	if item, ok := f.byPath[rel]; ok {
		return item, nil
	}
	return nil, nil
}

func (f *fakeResolver) GetByID(_ context.Context, id int64) (*domain.MediaItem, error) {
	if item, ok := f.byID[id]; ok {
		return item, nil
	}
	return nil, nil
}

// fakeThumbs returns deterministic WebP-ish bytes.
type fakeThumbs struct{ fail error }

func (f fakeThumbs) Get(_ context.Context, item domain.MediaItem) ([]byte, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	return []byte("RIFFxxxxWEBP:" + item.FileHash), nil
}

func testEnv(t *testing.T) (root string, res *fakeResolver, srv *Routes) {
	t.Helper()
	root = t.TempDir()

	body := make([]byte, 1024)
	for i := range body {
		body[i] = byte('A' + i%26)
	}
	if err := os.WriteFile(filepath.Join(root, "vacation.mp4"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "song.mp3"), []byte("audio-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A secret outside the root that traversal attempts must not reach.
	if err := os.WriteFile(filepath.Join(filepath.Dir(root), "secret.txt"), []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	res = &fakeResolver{
		byPath: map[string]*domain.MediaItem{
			"vacation.mp4": {ID: 1, FileHash: "h1", CurrentRelativePath: "vacation.mp4", MimeType: "video/mp4", FileSize: int64(len(body))},
			"song.mp3":     {ID: 2, FileHash: "h2", CurrentRelativePath: "song.mp3", MimeType: "audio/mpeg", FileSize: 11},
			"ghost.mp4":    {ID: 3, FileHash: "h3", CurrentRelativePath: "ghost.mp4", MimeType: "video/mp4"},
		},
		byID: map[int64]*domain.MediaItem{
			1: {ID: 1, FileHash: "h1", CurrentRelativePath: "vacation.mp4", MimeType: "video/mp4"},
		},
	}
	srv = &Routes{
		DriveRoot:  func() string { return root },
		Resolver:   res,
		Thumbnails: fakeThumbs{},
	}
	return root, res, srv
}

func serve(t *testing.T, srv *Routes) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestStreamFullBody(t *testing.T) {
	_, _, srv := testEnv(t)
	ts := serve(t, srv)

	resp, err := http.Get(ts.URL + "/stream?path=vacation.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("Content-Type = %q, want video/mp4", ct)
	}
	if acr := resp.Header.Get("Accept-Ranges"); acr != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes", acr)
	}
	data, _ := io.ReadAll(resp.Body)
	if len(data) != 1024 {
		t.Errorf("body length = %d, want 1024", len(data))
	}
}

func TestStreamRangeRequests(t *testing.T) {
	_, _, srv := testEnv(t)
	ts := serve(t, srv)

	req, _ := http.NewRequest("GET", ts.URL+"/stream?path=vacation.mp4", nil)
	req.Header.Set("Range", "bytes=0-99")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "bytes 0-99/1024" {
		t.Errorf("Content-Range = %q", cr)
	}
	data, _ := io.ReadAll(resp.Body)
	if len(data) != 100 {
		t.Errorf("partial body = %d bytes, want 100", len(data))
	}

	// Open-ended suffix range.
	req.Header.Set("Range", "bytes=1000-")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusPartialContent {
		t.Fatalf("suffix status = %d, want 206", resp2.StatusCode)
	}
	data2, _ := io.ReadAll(resp2.Body)
	if len(data2) != 24 {
		t.Errorf("suffix body = %d bytes, want 24", len(data2))
	}

	// HEAD carries headers without a body.
	reqHead, _ := http.NewRequest("HEAD", ts.URL+"/stream?path=song.mp3", nil)
	resp3, err := http.DefaultClient.Do(reqHead)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK || resp3.Header.Get("Content-Type") != "audio/mpeg" {
		t.Errorf("HEAD = %d %q", resp3.StatusCode, resp3.Header.Get("Content-Type"))
	}
}

func TestStreamSeekLatency(t *testing.T) {
	_, _, srv := testEnv(t)
	ts := serve(t, srv)
	client := http.Client{Timeout: 5 * time.Second}

	const seeks = 20
	start := time.Now()
	for i := 0; i < seeks; i++ {
		offset := i * 25
		req, _ := http.NewRequest("GET", ts.URL+"/stream?path=vacation.mp4", nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+9))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusPartialContent {
			t.Fatalf("seek status = %d, want 206", resp.StatusCode)
		}
	}
	avg := time.Since(start) / seeks
	if avg > 5*time.Millisecond {
		t.Errorf("average seek latency = %v, want under 5ms", avg)
	}
}

func TestStreamSecurity(t *testing.T) {
	_, _, srv := testEnv(t)
	ts := serve(t, srv)

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"traversal parent", "path=..%2Fsecret.txt", http.StatusForbidden},
		{"deep traversal", "path=..%2F..%2F..%2Fetc%2Fpasswd", http.StatusForbidden},
		{"absolute path", "path=%2Fetc%2Fpasswd", http.StatusForbidden},
		{"not in library", "path=unknown.mp4", http.StatusNotFound},
		{"cataloged but missing on disk", "path=ghost.mp4", http.StatusNotFound},
		{"empty path", "path=", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + "/stream?" + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}

	// The secret must never leak through any accepted path form.
	for _, q := range []string{"path=../secret.txt", "path=%2e%2e/secret.txt"} {
		resp, err := http.Get(ts.URL + "/stream?" + q)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(data) > 0 && string(data) == "top secret" {
			t.Errorf("secret leaked via %q", q)
		}
	}
}

func TestThumbnailEndpoint(t *testing.T) {
	_, res, srv := testEnv(t)
	ts := serve(t, srv)

	resp, err := http.Get(ts.URL + "/thumbnail?mediaId=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/webp" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != thumbnailCache {
		t.Errorf("Cache-Control = %q", cc)
	}

	// Unknown id and malformed id.
	if r, _ := http.Get(ts.URL + "/thumbnail?mediaId=999"); r.StatusCode != http.StatusNotFound {
		t.Errorf("unknown id status = %d", r.StatusCode)
	} else {
		r.Body.Close()
	}
	if r, _ := http.Get(ts.URL + "/thumbnail?mediaId=abc"); r.StatusCode != http.StatusBadRequest {
		t.Errorf("bad id status = %d", r.StatusCode)
	} else {
		r.Body.Close()
	}

	// Thumbnailer failure surfaces as 500.
	res.byID[7] = &domain.MediaItem{ID: 7, FileHash: "h7"}
	srvFail := &Routes{DriveRoot: func() string { return "x" }, Resolver: res, Thumbnails: fakeThumbs{fail: errors.New("boom")}}
	tsFail := serve(t, srvFail)
	if r, _ := http.Get(tsFail.URL + "/thumbnail?mediaId=7"); r.StatusCode != http.StatusInternalServerError {
		t.Errorf("failing thumb status = %d", r.StatusCode)
	} else {
		r.Body.Close()
	}
}

func TestResolvePathUnit(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "albums", "trip")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "a.jpg")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolvePath(root, "albums/trip/a.jpg")
	if err != nil || got != file {
		t.Errorf("ResolvePath = %q, %v; want %q", got, err, file)
	}

	// Windows-style separators are cleaned by filepath on Linux into a
	// single component name; both must stay inside the root.
	if _, err := ResolvePath(root, "albums/trip/../trip/a.jpg"); err != nil {
		t.Errorf("inner dot path rejected: %v", err)
	}
	for _, bad := range []string{"", "/etc/passwd", "..", "a/../../.."} {
		if _, err := ResolvePath(root, bad); !errors.Is(err, ErrOutsideRoot) && !errors.Is(err, ErrMissing) {
			t.Errorf("ResolvePath(%q) err = %v, want outside-root or missing", bad, err)
		}
	}
	if _, err := ResolvePath(root, "nope/missing.jpg"); !errors.Is(err, ErrMissing) {
		t.Errorf("missing file err = %v, want ErrMissing", err)
	}
	if _, err := ResolvePath(t.TempDir()+"/nonexistent-root", "a"); err == nil {
		t.Error("missing root should error")
	}
}
