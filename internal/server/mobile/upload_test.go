package mobile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"painless/internal/domain"
)

// jpegBytes returns bytes with a real JPEG magic header so MIME sniffing
// resolves to image/jpeg.
func jpegBytes() []byte {
	return append([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}, []byte("JFIF\x00\x01fake-image-payload")...)
}

// multipartBody builds a multipart body with a single file field.
func multipartBody(t *testing.T, fieldName, filename string, content []byte) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), &buf
}

func postUpload(t *testing.T, url, token string, headers map[string]string, contentType string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestUploadMultipartToAlbum(t *testing.T) {
	root, srv, _, albums := newTestEnv(t)
	ts := serve(t, srv)

	album, err := albums.Create(t.Context(), domain.CreateAlbumInput{Name: "Beach Trip"})
	if err != nil {
		t.Fatal(err)
	}

	content := jpegBytes()
	ct, body := multipartBody(t, "file", "beach.jpg", content)
	resp := postUpload(t, ts.URL+"/api/upload", testToken, map[string]string{"X-Album-ID": fmt.Sprint(album.ID)}, ct, body)
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("upload status = %d, body = %s", resp.StatusCode, respBody)
	}
	result := decode[uploadResponse](t, resp)
	if result.Duplicate {
		t.Error("first upload reported duplicate")
	}
	if result.Item == nil || result.Item.ID == 0 || result.Item.AlbumID == nil || *result.Item.AlbumID != album.ID {
		t.Fatalf("uploaded item = %+v", result.Item)
	}
	if result.Item.MimeType != "image/jpeg" {
		t.Errorf("mime = %q, want image/jpeg", result.Item.MimeType)
	}
	if result.Item.FileHash == "" || result.Item.FileSize != int64(len(content)) {
		t.Errorf("hash/size = %q/%d", result.Item.FileHash, result.Item.FileSize)
	}

	// File must physically exist under albums/Beach Trip/.
	dest := filepath.Join(root, "albums", "Beach Trip", "beach.jpg")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("uploaded file missing at %s: %v", dest, err)
	}
	// No temp residue left on the drive.
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload_") {
			t.Errorf("temp residue: %s", e.Name())
		}
	}
}

func TestUploadDuplicateContent(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	ts := serve(t, srv)

	content := jpegBytes()
	ct, body := multipartBody(t, "file", "photo.jpg", content)
	first := postUpload(t, ts.URL+"/api/upload", testToken, nil, ct, body)
	first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first upload = %d", first.StatusCode)
	}

	ct, body = multipartBody(t, "file", "renamed.jpg", content)
	resp := postUpload(t, ts.URL+"/api/upload", testToken, nil, ct, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("duplicate upload = %d, want 200", resp.StatusCode)
	}
	result := decode[uploadResponse](t, resp)
	if !result.Duplicate || result.Item == nil {
		t.Errorf("duplicate response = %+v", result)
	}
}

func TestUploadRawBodyToUnknown(t *testing.T) {
	root, srv, _, _ := newTestEnv(t)
	ts := serve(t, srv)

	content := []byte("ID3audio-tagging-bytes")
	resp := postUpload(t, ts.URL+"/api/upload", testToken,
		map[string]string{"X-File-Name": "song.mp3"},
		"application/octet-stream", bytes.NewReader(content))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("raw upload = %d", resp.StatusCode)
	}
	result := decode[uploadResponse](t, resp)
	if result.Item == nil || result.Item.MimeType != "audio/mpeg" {
		t.Fatalf("raw item = %+v", result.Item)
	}
	dest := filepath.Join(root, "unknown", "song.mp3")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("raw upload missing at %s: %v", dest, err)
	}
}

func TestUploadValidationErrors(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	ts := serve(t, srv)

	// No token.
	resp := postUpload(t, ts.URL+"/api/upload", "", nil,
		"application/octet-stream", strings.NewReader("x"))
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token upload = %d, want 401", resp.StatusCode)
	}

	// Raw upload without X-File-Name.
	resp = postUpload(t, ts.URL+"/api/upload", testToken, nil,
		"application/octet-stream", strings.NewReader("x"))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("raw upload without name = %d, want 400", resp.StatusCode)
	}

	// Unknown album.
	ct, body := multipartBody(t, "file", "a.jpg", jpegBytes())
	resp = postUpload(t, ts.URL+"/api/upload", testToken, map[string]string{"X-Album-ID": "42"}, ct, body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown album upload = %d, want 404", resp.StatusCode)
	}

	// Multipart with no file part.
	var empty bytes.Buffer
	mw := multipart.NewWriter(&empty)
	_, _ = mw.CreateFormField("not-a-file")
	_ = mw.Close()
	resp = postUpload(t, ts.URL+"/api/upload", testToken, nil, mw.FormDataContentType(), &empty)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("no file part = %d, want 400", resp.StatusCode)
	}

	// Hostile filename characters are sanitized, not rejected.
	ct, body = multipartBody(t, "file", "be<>ach?.jpg", jpegBytes())
	resp = postUpload(t, ts.URL+"/api/upload", testToken, nil, ct, body)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("hostile filename upload = %d, want 201", resp.StatusCode)
	} else {
		result := decode[uploadResponse](t, resp)
		if strings.ContainsAny(filepath.Base(result.Item.CurrentRelativePath), "<>?") {
			t.Errorf("filename not sanitized: %q", result.Item.CurrentRelativePath)
		}
	}
}

func TestUploadTooLarge(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.opts.MaxUploadBytes = 8
	ts := serve(t, srv)

	resp := postUpload(t, ts.URL+"/api/upload", testToken,
		map[string]string{"X-File-Name": "big.jpg"},
		"application/octet-stream", strings.NewReader("0123456789ABCDEF"))
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize upload = %d, want 413", resp.StatusCode)
	}
}

func TestUploadEmptyBody(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	ts := serve(t, srv)

	resp := postUpload(t, ts.URL+"/api/upload", testToken,
		map[string]string{"X-File-Name": "empty.jpg"},
		"application/octet-stream", strings.NewReader(""))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty upload = %d, want 400", resp.StatusCode)
	}
}

func TestUploadWithoutDrive(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	srv.SetDriveRoot("")
	ts := serve(t, srv)

	resp := postUpload(t, ts.URL+"/api/upload", testToken,
		map[string]string{"X-File-Name": "a.jpg"},
		"application/octet-stream", strings.NewReader("x"))
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("upload without drive = %d, want 503", resp.StatusCode)
	}
}

func TestUploadWarmsThumbnail(t *testing.T) {
	_, srv, _, _ := newTestEnv(t)
	warmed := make(chan domain.MediaItem, 1)
	srv.opts.Thumbnails = warmRecorder{warmed}
	ts := serve(t, srv)

	ct, body := multipartBody(t, "file", "warm.jpg", jpegBytes())
	resp := postUpload(t, ts.URL+"/api/upload", testToken, nil, ct, body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload = %d", resp.StatusCode)
	}
	select {
	case item := <-warmed:
		if item.FileHash == "" {
			t.Error("warmed item has empty hash")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("thumbnail was not warmed")
	}
}

type warmRecorder struct{ ch chan domain.MediaItem }

func (w warmRecorder) Get(_ context.Context, item domain.MediaItem) ([]byte, error) {
	w.ch <- item
	return []byte("webp"), nil
}
