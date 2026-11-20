package mobile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"painless/internal/domain"
)

// fakeThumbs satisfies ThumbnailWarmer with fixed WebP-ish bytes.
type fakeThumbs struct{}

func (fakeThumbs) Get(_ context.Context, item domain.MediaItem) ([]byte, error) {
	return []byte("webp:" + item.FileHash), nil
}

// newCompatEnv is a test env whose thumbnail warmer is wired, matching
// production wiring where thumbnails are always available.
func newCompatEnv(t *testing.T) (root string, srv *Server, media *mediaDeps) {
	t.Helper()
	r, s, m, a := newTestEnv(t)
	s.opts.Thumbnails = fakeThumbs{}
	return r, s, &mediaDeps{media: m, albums: a}
}

type mediaDeps struct {
	media interface {
		List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error)
		InsertBatch(ctx context.Context, items []domain.MediaItem) (int, error)
		GetByID(ctx context.Context, id int64) (*domain.MediaItem, error)
		GetByPath(ctx context.Context, relPath string) (*domain.MediaItem, error)
		GetByHash(ctx context.Context, hash string) (*domain.MediaItem, error)
	}
	albums interface {
		Create(ctx context.Context, input domain.CreateAlbumInput) (*domain.Album, error)
		GetByID(ctx context.Context, id int64) (*domain.Album, error)
	}
}

// putFile writes a file under the drive root.
func putFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompatAlbumGetUpdateDelete(t *testing.T) {
	root, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	album := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken,
		strings.NewReader(`{"name":"Trip","description":"summer"}`)))

	// GET one album.
	got := decode[domain.Album](t, mustGet(t, fmt.Sprintf("%s/api/albums/%d", ts.URL, album.ID)))
	if got.ID != album.ID || got.Name != "Trip" {
		t.Errorf("got album = %+v", got)
	}
	resp := mustGet(t, ts.URL+"/api/albums/999")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown album status = %d, want 404", resp.StatusCode)
	}

	// PATCH name and description.
	updated := decode[domain.Album](t, do(t, "PATCH", fmt.Sprintf("%s/api/albums/%d", ts.URL, album.ID), testToken,
		strings.NewReader(`{"name":"Alps","description":"winter"}`)))
	if updated.Name != "Alps" || updated.Description == nil || *updated.Description != "winter" {
		t.Errorf("updated album = %+v", updated)
	}

	// PATCH description only keeps the name.
	updated = decode[domain.Album](t, do(t, "PATCH", fmt.Sprintf("%s/api/albums/%d", ts.URL, album.ID), testToken,
		strings.NewReader(`{"description":"spring"}`)))
	if updated.Name != "Alps" || updated.Description == nil || *updated.Description != "spring" {
		t.Errorf("description-only update = %+v", updated)
	}

	// Invalid rename is rejected.
	resp = do(t, "PATCH", fmt.Sprintf("%s/api/albums/%d", ts.URL, album.ID), testToken,
		strings.NewReader(`{"name":"bad/name"}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid rename status = %d, want 400", resp.StatusCode)
	}

	// DELETE (non-permanent) unassigns media back to Unsorted.
	item := compatUpload(t, ts, "clip.mp4", "video-clip", &album.ID)
	deleted := decode[map[string]any](t, do(t, "DELETE", fmt.Sprintf("%s/api/albums/%d", ts.URL, album.ID), testToken, nil))
	if deleted["permanent"] != false {
		t.Errorf("delete result = %+v", deleted)
	}
	after, _ := deps.media.GetByID(context.Background(), item.ID)
	if after == nil || after.AlbumID != nil {
		t.Errorf("item after album delete = %+v, want unassigned", after)
	}

	// DELETE permanent removes records and physical files.
	album2 := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken, strings.NewReader(`{"name":"Gone"}`)))
	item2 := compatUpload(t, ts, "gone.jpg", "gone-payload", &album2.ID)
	path := filepath.Join(root, item2.CurrentRelativePath)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("uploaded file missing: %v", err)
	}
	deleted = decode[map[string]any](t, do(t, "DELETE", fmt.Sprintf("%s/api/albums/%d?permanent=true", ts.URL, album2.ID), testToken, nil))
	if deleted["permanent"] != true {
		t.Errorf("permanent delete result = %+v", deleted)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("permanent delete left file %s (stat err %v)", path, err)
	}
	if after2, _ := deps.media.GetByID(context.Background(), item2.ID); after2 != nil {
		t.Errorf("record survived permanent delete: %+v", after2)
	}
	if got2, _ := deps.albums.GetByID(context.Background(), album2.ID); got2 != nil {
		t.Errorf("album survived delete: %+v", got2)
	}
}

func TestCompatMediaStreamThumbnailDetails(t *testing.T) {
	root, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	putFile(t, root, "unknown/stream.txt", "streamable-payload-0123456789")
	item := domain.MediaItem{
		FileHash: "h-stream", OriginalRelativePath: "unknown/stream.txt",
		CurrentRelativePath: "unknown/stream.txt", FileSize: int64(len("streamable-payload-0123456789")),
		MimeType: "text/plain",
	}
	inserted, err := deps.media.InsertBatch(context.Background(), []domain.MediaItem{item})
	if err != nil || inserted != 1 {
		t.Fatalf("insert batch = %d, %v", inserted, err)
	}
	stored, _ := deps.media.GetByHash(context.Background(), "h-stream")

	// Full-body stream.
	streamed, err := http.Get(fmt.Sprintf("%s/api/media/%d", ts.URL, stored.ID))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(streamed.Body)
	streamed.Body.Close()
	if streamed.StatusCode != http.StatusOK || string(body) != "streamable-payload-0123456789" {
		t.Errorf("stream = %d %q", streamed.StatusCode, body)
	}

	// Byte range.
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/media/%d", ts.URL, stored.ID), nil)
	req.Header.Set("Range", "bytes=0-8")
	ranged, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rbody, _ := io.ReadAll(ranged.Body)
	ranged.Body.Close()
	if ranged.StatusCode != http.StatusPartialContent || string(rbody) != "streamabl" {
		t.Errorf("range = %d %q", ranged.StatusCode, rbody)
	}

	// Thumbnail by ID.
	thumb, err := http.Get(fmt.Sprintf("%s/api/thumbnail/%d", ts.URL, stored.ID))
	if err != nil {
		t.Fatal(err)
	}
	tbody, _ := io.ReadAll(thumb.Body)
	thumb.Body.Close()
	if thumb.StatusCode != http.StatusOK || string(tbody) != "webp:h-stream" {
		t.Errorf("thumbnail = %d %q", thumb.StatusCode, tbody)
	}

	// Details JSON.
	details := decode[domain.MediaItem](t, mustGet(t, fmt.Sprintf("%s/api/media/%d/details", ts.URL, stored.ID)))
	if details.FileHash != "h-stream" {
		t.Errorf("details = %+v", details)
	}
}

func TestCompatUploadMultipartFormFields(t *testing.T) {
	_, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	album := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken,
		strings.NewReader(`{"name":"Uploads"}`)))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// D-Stream sends albumId as a regular form field before/after the file.
	if err := mw.WriteField("albumId", fmt.Sprintf("%d", album.ID)); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(jpegBytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", ts.URL+"/api/media", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("multipart upload status = %d: %s", resp.StatusCode, readBody(resp))
	}
	result := decode[uploadResponse](t, resp)
	if result.Item == nil || result.Item.AlbumID == nil || *result.Item.AlbumID != album.ID {
		t.Errorf("upload result = %+v", result)
	}
	onDisk, _ := deps.media.GetByID(context.Background(), result.Item.ID)
	if onDisk == nil {
		t.Errorf("upload not cataloged")
	}
}

func TestCompatUploadRawQuery(t *testing.T) {
	_, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	album := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken,
		strings.NewReader(`{"name":"Raw"}`)))

	url := fmt.Sprintf("%s/api/media?albumId=%d&filename=clip.mp4", ts.URL, album.ID)
	resp := do(t, "POST", url, testToken, strings.NewReader("raw-video-bytes"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("raw upload status = %d: %s", resp.StatusCode, readBody(resp))
	}
	result := decode[uploadResponse](t, resp)
	if result.Item == nil || result.Item.AlbumID == nil || *result.Item.AlbumID != album.ID {
		t.Errorf("raw upload result = %+v", result)
	}
	if !strings.HasSuffix(result.Item.CurrentRelativePath, "clip.mp4") {
		t.Errorf("raw upload path = %q", result.Item.CurrentRelativePath)
	}
	_ = deps
}

func TestCompatMediaMoveAndDelete(t *testing.T) {
	root, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	target := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken,
		strings.NewReader(`{"name":"Target"}`)))
	item := compatUpload(t, ts, "move-me.jpg", "move-payload", nil)

	// PATCH move into album.
	moved := decode[domain.MediaItem](t, do(t, "PATCH", fmt.Sprintf("%s/api/media/%d", ts.URL, item.ID), testToken,
		strings.NewReader(fmt.Sprintf(`{"albumId":%d}`, target.ID))))
	if moved.AlbumID == nil || *moved.AlbumID != target.ID {
		t.Errorf("moved item = %+v", moved)
	}

	// PATCH albumId null unassigns.
	moved = decode[domain.MediaItem](t, do(t, "PATCH", fmt.Sprintf("%s/api/media/%d", ts.URL, item.ID), testToken,
		strings.NewReader(`{"albumId":null}`)))
	if moved.AlbumID != nil {
		t.Errorf("unassigned item = %+v", moved)
	}

	// Unknown album target 404s.
	resp := do(t, "PATCH", fmt.Sprintf("%s/api/media/%d", ts.URL, item.ID), testToken,
		strings.NewReader(`{"albumId":424242}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown album move status = %d, want 404", resp.StatusCode)
	}

	// DELETE removes record and file.
	path := filepath.Join(root, item.CurrentRelativePath)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file missing before delete: %v", err)
	}
	deleted := decode[map[string]any](t, do(t, "DELETE", fmt.Sprintf("%s/api/media/%d", ts.URL, item.ID), testToken, nil))
	if deleted["deleted"] == nil {
		t.Errorf("delete result = %+v", deleted)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file survived delete (stat err %v)", err)
	}
	if after, _ := deps.media.GetByID(context.Background(), item.ID); after != nil {
		t.Errorf("record survived delete: %+v", after)
	}
}

func TestCompatQueryParamAliases(t *testing.T) {
	_, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	albumA := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken, strings.NewReader(`{"name":"A"}`)))
	albumB := decode[domain.Album](t, do(t, "POST", ts.URL+"/api/albums", testToken, strings.NewReader(`{"name":"B"}`)))

	items := []domain.MediaItem{
		{FileHash: "a1", OriginalRelativePath: "unknown/1.jpg", CurrentRelativePath: "unknown/1.jpg", FileSize: 3, MimeType: "image/jpeg", AlbumID: &albumA.ID},
		{FileHash: "b1", OriginalRelativePath: "unknown/2.jpg", CurrentRelativePath: "unknown/2.jpg", FileSize: 5, MimeType: "image/jpeg", AlbumID: &albumB.ID},
	}
	if _, err := deps.media.InsertBatch(context.Background(), items); err != nil {
		t.Fatal(err)
	}

	snake := decode[domain.MediaListPage](t, mustGet(t, fmt.Sprintf("%s/api/media?album_id=%d&sort_by=size&sort_order=desc", ts.URL, albumA.ID)))
	camel := decode[domain.MediaListPage](t, mustGet(t, fmt.Sprintf("%s/api/media?albumId=%d&sortBy=size&sortOrder=desc", ts.URL, albumA.ID)))
	if camel.Total != 1 || len(camel.Items) != 1 || camel.Items[0].FileHash != "a1" {
		t.Errorf("camelCase query result = %+v", camel)
	}
	if snake.Total != camel.Total || len(snake.Items) != len(camel.Items) {
		t.Errorf("alias mismatch: snake %+v vs camel %+v", snake, camel)
	}
}

// compatUpload performs a native /api/upload and returns the stored item.
func compatUpload(t *testing.T, ts *httptest.Server, filename, content string, albumID *int64) domain.MediaItem {
	t.Helper()
	url := ts.URL + "/api/upload"
	body := strings.NewReader(content)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-File-Name", filename)
	if albumID != nil {
		req.Header.Set("X-Album-ID", fmt.Sprintf("%d", *albumID))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload %s status = %d: %s", filename, resp.StatusCode, readBody(resp))
	}
	result := decode[uploadResponse](t, resp)
	return *result.Item
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return string(b)
}
