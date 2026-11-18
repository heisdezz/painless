package viewer

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
)

// TestRenderSnapshots writes rendered frames of the redesigned viewer to
// /tmp for visual inspection. Run with:
//
//	go test ./internal/ui/screens/viewer -run TestRenderSnapshots -v
func TestRenderSnapshots(t *testing.T) {
	items := testMediaItems()

	screen := New(Options{FileOpener: &mockFileOpener{}})
	screen.OpenItem(items[0], items) // image with EXIF + GPS
	tt := mui.NewTester(screen.View, 1100, 700)
	tt.SetDark(true)
	tt.Advance(1 * time.Second)
	tt.Frame()
	writePNG(t, "/tmp/viewer_image.png", tt.Image())

	screen.OpenItem(items[1], items) // video
	tt.Frame()
	writePNG(t, "/tmp/viewer_video.png", tt.Image())

	screen.ToggleDrawer() // rail closed, full-bleed stage
	tt.Frame()
	writePNG(t, "/tmp/viewer_video_norail.png", tt.Image())

	screen2 := New(Options{FileOpener: &mockFileOpener{}})
	screen2.OpenItem(items[0], items)
	tt2 := mui.NewTester(screen2.View, 800, 600)
	tt2.SetDark(true)
	tt2.Frame()
	writePNG(t, "/tmp/viewer_narrow.png", tt2.Image())
}

func writePNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	if img == nil {
		t.Fatal("no image rendered")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
