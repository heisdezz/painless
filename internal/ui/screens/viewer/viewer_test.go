package viewer

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

type mockThrottler struct {
	mu          sync.Mutex
	pauseCount  int
	resumeCount int
}

func (m *mockThrottler) Pause() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pauseCount++
}

func (m *mockThrottler) Resume() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resumeCount++
}

type mockFileOpener struct {
	files map[string][]byte
}

func (m *mockFileOpener) ReadFile(relPath string) ([]byte, error) {
	if data, ok := m.files[relPath]; ok {
		return data, nil
	}
	// Return a valid 16x16 PNG for testing
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes(), nil
}

func testMediaItems() []domain.MediaItem {
	dur := 125
	lat := 37.7749
	lon := -122.4194
	alt := 15.5
	dateTaken := time.Date(2026, 7, 15, 14, 30, 0, 0, time.UTC)

	imgMeta := domain.MediaMetadata{
		Width:        1920,
		Height:       1080,
		CameraMake:   "Sony",
		CameraModel:  "A7 IV",
		LensModel:    "FE 24-70mm F2.8 GM II",
		Aperture:     2.8,
		ExposureTime: "1/500",
		ISO:          100,
		FocalLength:  50.0,
		DateTaken:    &dateTaken,
		GPSLatitude:  &lat,
		GPSLongitude: &lon,
		GPSAltitude:  &alt,
	}

	vidMeta := domain.MediaMetadata{
		Width:      3840,
		Height:     2160,
		VideoCodec: "h264",
		AudioCodec: "aac",
		Bitrate:    25000000,
	}

	return []domain.MediaItem{
		{
			ID:                  1,
			FileHash:            "hash1",
			CurrentRelativePath: "photos/vacation.jpg",
			FileSize:            2048576,
			MimeType:            "image/jpeg",
			ParsedMetadata:      &imgMeta,
		},
		{
			ID:                  2,
			FileHash:            "hash2",
			CurrentRelativePath: "videos/sunset.mp4",
			FileSize:            52428800,
			MimeType:            "video/mp4",
			DurationSeconds:     &dur,
			ParsedMetadata:      &vidMeta,
		},
	}
}

func TestViewer_EmptyState(t *testing.T) {
	screen := New(Options{})
	tt := mui.NewTester(screen.View, 1000, 700)

	if !tt.HasText("No media selected") {
		t.Fatalf("expected empty state text, got: %q", tt.Texts())
	}
}

func TestViewer_ImageViewer(t *testing.T) {
	items := testMediaItems()
	opener := &mockFileOpener{files: make(map[string][]byte)}
	closed := false

	screen := New(Options{
		FileOpener: opener,
		OnClose: func() {
			closed = true
		},
	})

	screen.OpenItem(items[0], items)

	if !screen.IsOpen() || screen.CurrentItem() == nil || screen.CurrentItem().ID != 1 {
		t.Fatalf("expected current item to be ID 1")
	}

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("vacation.jpg") {
		t.Fatalf("expected filename vacation.jpg, got texts: %q", tt.Texts())
	}
	if !tt.HasText("1 of 2") {
		t.Fatalf("expected counter 1 of 2, got texts: %q", tt.Texts())
	}

	// Close viewer
	screen.Close()
	if !closed {
		t.Fatalf("expected onClose callback to be called")
	}
	if screen.IsOpen() || screen.CurrentItem() != nil {
		t.Fatalf("expected current item to be nil after close")
	}
}

func TestViewer_MetadataDrawer(t *testing.T) {
	items := testMediaItems()
	screen := New(Options{
		FileOpener: &mockFileOpener{files: make(map[string][]byte)},
	})

	screen.OpenItem(items[0], items)

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("Media Details & EXIF") {
		t.Fatalf("expected drawer title, got: %q", tt.Texts())
	}
	if !tt.HasText("File Attributes") || !tt.HasText("EXIF Camera Details") {
		t.Fatalf("expected accordion section headers, got: %q", tt.Texts())
	}
	if !tt.HasText("Sony") || !tt.HasText("A7 IV") {
		t.Fatalf("expected camera metadata rendered, got: %q", tt.Texts())
	}
	if !tt.HasText("1920 × 1080") || !tt.HasText("16:9") {
		t.Fatalf("expected resolution and aspect ratio, got: %q", tt.Texts())
	}
	if !tt.HasText("37.774900") || !tt.HasText("-122.419400") {
		t.Fatalf("expected GPS coordinates, got: %q", tt.Texts())
	}
}

func TestViewer_Navigation(t *testing.T) {
	items := testMediaItems()
	screen := New(Options{})

	screen.OpenItem(items[0], items)
	if screen.CurrentItem().ID != 1 {
		t.Fatalf("expected item 1")
	}

	screen.NextItem()
	if screen.CurrentItem().ID != 2 {
		t.Fatalf("expected item 2 after next")
	}

	screen.NextItem()
	if screen.CurrentItem().ID != 1 {
		t.Fatalf("expected wrap around to item 1")
	}

	screen.PrevItem()
	if screen.CurrentItem().ID != 2 {
		t.Fatalf("expected wrap around to item 2 on prev")
	}
}

func TestViewer_VideoPlayerThrottling(t *testing.T) {
	items := testMediaItems()
	throttler := &mockThrottler{}

	screen := New(Options{
		Throttler: throttler,
	})

	screen.OpenItem(items[1], items)
	if screen.CurrentItem().Type() != domain.MediaTypeVideo {
		t.Fatalf("expected item 2 to be video")
	}

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("sunset.mp4") {
		t.Fatalf("expected video filename, got: %q", tt.Texts())
	}

	// Test play/pause throttling hooks directly
	screen.togglePlayPause()

	// Closing video stops playback and resumes throttler
	screen.Close()
	throttler.mu.Lock()
	resumeCount := throttler.resumeCount
	throttler.mu.Unlock()

	if resumeCount == 0 {
		t.Fatalf("expected throttler.Resume() to be called on close")
	}
}

func TestViewer_ConcurrentAccess(t *testing.T) {
	items := testMediaItems()
	screen := New(Options{})
	screen.OpenItem(items[0], items)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%4 == 0 {
				screen.NextItem()
			} else if idx%4 == 1 {
				screen.PrevItem()
			} else if idx%4 == 2 {
				_ = screen.CurrentItem()
			} else {
				screen.ToggleDrawer()
			}
		}(i)
	}
	wg.Wait()
}

func TestFFmpegDecoder_ConfigDefaults(t *testing.T) {
	dur := 10 * time.Second
	cfg := FFmpegDecoderConfig{
		FilePath: "nonexistent.mp4",
		Duration: dur,
	}

	// Should handle missing file safely without crashing
	d, err := NewFFmpegDecoder(cfg)
	if err == nil && d != nil {
		if d.Duration() != dur {
			t.Errorf("expected duration %v, got %v", dur, d.Duration())
		}
		_ = d.Close()
	}
}
