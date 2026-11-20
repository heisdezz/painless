package discover

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/fileops"
)

// TestExecutePanelAlertFullyVisible guards the "Staging Ready" banner against
// clipping. The upper half of the discover screen is a stack of fixed-height
// blocks; before they were wrapped in a capped scroll region, HiDPI zoom or
// the minimum window size pushed the banner below the viewport, where the
// tester still reported it — with a zero-sized rect.
func TestExecutePanelAlertFullyVisible(t *testing.T) {
	stager := fileops.NewStagingEngine()
	screen := New(Options{Stager: stager})
	screen.SetTargetPath("/media/usb")
	screen.StageItem(domain.MediaItem{
		ID:                  101,
		FileHash:            "hash101",
		CurrentRelativePath: "photos/sample.jpg",
	}, "Vacation")

	// Default size and the app's minimum, at the zoom factors HiDPI Linux
	// setups commonly pick.
	cases := []struct {
		name          string
		width, height int
		zoom          float32
	}{
		{"default", 1000, 800, 1.0},
		{"minimum", 800, 600, 1.0},
		{"minimum-zoom-125", 800, 600, 1.25},
		{"minimum-zoom-150", 800, 600, 1.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tt := mui.NewTester(screen.View, tc.width, tc.height)
			tt.SetZoom(tc.zoom)
			tt.Frame()

			w, h := float32(tc.width), float32(tc.height)

			alert, ok := tt.Find("Staging Ready")
			if !ok {
				t.Fatalf("'Staging Ready' alert not rendered; texts = %q", tt.Texts())
			}
			if alert.W <= 0 || alert.H <= 0 {
				t.Fatalf("alert has zero extent (clipped out of the viewport): rect = %+v", alert)
			}
			if alert.X < 0 || alert.Y < 0 || alert.X+alert.W > w || alert.Y+alert.H > h {
				t.Fatalf("alert clipped: rect = %+v  (window %.0fx%.0f, zoom %.2f)", alert, w, h, tc.zoom)
			}

			button, ok := tt.Find("Consolidate & Execute")
			if !ok {
				t.Fatalf("'Consolidate & Execute' button not rendered; texts = %q", tt.Texts())
			}
			if button.W <= 0 || button.H <= 0 {
				t.Fatalf("button has zero extent (clipped out of the viewport): rect = %+v", button)
			}
			if button.X < 0 || button.Y < 0 || button.X+button.W > w || button.Y+button.H > h {
				t.Fatalf("button clipped: rect = %+v  (window %.0fx%.0f, zoom %.2f)", button, w, h, tc.zoom)
			}
			if button.X < alert.X+alert.W && button.Y < alert.Y+alert.H && button.Y+button.H > alert.Y {
				t.Fatalf("button overlaps alert: alert %+v, button %+v", alert, button)
			}
		})
	}
}
