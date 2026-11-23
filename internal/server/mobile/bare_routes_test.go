package mobile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"painless/internal/domain"
)

// TestBareMediaAndThumbnailRoutes verifies the unprefixed /media/{id} and
// /thumbnail/{id} routes that the D-Stream mobile app actually calls.
func TestBareMediaAndThumbnailRoutes(t *testing.T) {
	root, srv, deps := newCompatEnv(t)
	ts := serve(t, srv)

	putFile(t, root, "unknown/bare.txt", "bare-payload-0123456789")
	item := domain.MediaItem{
		FileHash: "h-bare", OriginalRelativePath: "unknown/bare.txt",
		CurrentRelativePath: "unknown/bare.txt", FileSize: int64(len("bare-payload-0123456789")),
		MimeType: "text/plain",
	}
	inserted, err := deps.media.InsertBatch(context.Background(), []domain.MediaItem{item})
	if err != nil || inserted != 1 {
		t.Fatalf("insert batch = %d, %v", inserted, err)
	}
	stored, _ := deps.media.GetByHash(context.Background(), "h-bare")

	// Bare /media/{id} full body stream.
	bareStreamed, err := http.Get(fmt.Sprintf("%s/media/%d", ts.URL, stored.ID))
	if err != nil {
		t.Fatal(err)
	}
	bareBody, _ := io.ReadAll(bareStreamed.Body)
	bareStreamed.Body.Close()
	if bareStreamed.StatusCode != http.StatusOK || string(bareBody) != "bare-payload-0123456789" {
		t.Errorf("bare /media stream = %d %q", bareStreamed.StatusCode, bareBody)
	}

	// Bare /media/{id} byte range.
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/media/%d", ts.URL, stored.ID), nil)
	req.Header.Set("Range", "bytes=0-8")
	bareRanged, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	bareRbody, _ := io.ReadAll(bareRanged.Body)
	bareRanged.Body.Close()
	if bareRanged.StatusCode != http.StatusPartialContent || string(bareRbody) != "bare-payl" {
		t.Errorf("bare /media range = %d %q", bareRanged.StatusCode, bareRbody)
	}

	// Bare /thumbnail/{id}.
	bareThumb, err := http.Get(fmt.Sprintf("%s/thumbnail/%d", ts.URL, stored.ID))
	if err != nil {
		t.Fatal(err)
	}
	bareTbody, _ := io.ReadAll(bareThumb.Body)
	bareThumb.Body.Close()
	if bareThumb.StatusCode != http.StatusOK || string(bareTbody) != "webp:h-bare" {
		t.Errorf("bare /thumbnail = %d %q", bareThumb.StatusCode, bareTbody)
	}
}
