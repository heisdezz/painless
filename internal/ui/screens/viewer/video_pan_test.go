package viewer

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func TestVideoDragPansWhenZoomed(t *testing.T) {
	screen := New(Options{})
	screen.zoomVideoAt(200, 150, 2) // pan starts at (-200, -150)

	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerDown, Button: 0, X: 100, Y: 100})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 150, Y: 130})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 170, Y: 120})
	if !near(screen.videoPanX, -60) || !near(screen.videoPanY, -110) {
		t.Fatalf("pan = (%v, %v), want (-60, -110)", screen.videoPanX, screen.videoPanY)
	}
	if !screen.videoPanning {
		t.Fatal("drag ended early")
	}

	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerUp, Button: 0, X: 170, Y: 120})
	if screen.videoPanning {
		t.Fatal("release did not end the drag")
	}
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: -1, X: 300, Y: 300})
	if !near(screen.videoPanX, -60) || !near(screen.videoPanY, -110) {
		t.Fatalf("pan changed after release: (%v, %v)", screen.videoPanX, screen.videoPanY)
	}
}

func TestVideoDragAtNaturalSizeDoesNotPan(t *testing.T) {
	screen := New(Options{})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerDown, Button: 0, X: 100, Y: 100})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 200, Y: 200})
	if screen.videoPanning || screen.videoPanX != 0 || screen.videoPanY != 0 {
		t.Fatalf("dragging at 1x panned: (%v, %v)", screen.videoPanX, screen.videoPanY)
	}
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerUp, Button: 0, X: 200, Y: 200})
}

func TestVideoPanRollsBackOnCanceledPress(t *testing.T) {
	screen := New(Options{})
	screen.zoomVideoAt(200, 150, 2)
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerDown, Button: 0, X: 100, Y: 100})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 180, Y: 160})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerUp, Button: 0, X: 180, Y: 160, Canceled: true})
	if !near(screen.videoPanX, -200) || !near(screen.videoPanY, -150) || screen.videoPanning {
		t.Fatalf("canceled drag left pan=(%v,%v) panning=%v", screen.videoPanX, screen.videoPanY, screen.videoPanning)
	}
}

// Zooming in during a drag enables panning from that point on; no modifier
// is involved, and the pre-zoom movement is not applied retroactively.
func TestVideoDragStartsAfterZooming(t *testing.T) {
	screen := New(Options{})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerDown, Button: 0, X: 100, Y: 100})
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 130, Y: 120})
	if screen.videoPanning || screen.videoPanX != 0 {
		t.Fatal("drag began panning before zooming in")
	}
	screen.zoomVideoAt(100, 100, 2)
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 150, Y: 140})
	if !screen.videoPanning || !near(screen.videoPanX, -60) || !near(screen.videoPanY, -60) {
		t.Fatalf("drag after zoom did not pan: pan=(%v,%v) panning=%v", screen.videoPanX, screen.videoPanY, screen.videoPanning)
	}
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerUp, Button: 0, X: 150, Y: 140})
}

func TestVideoPanStopsWhenZoomedBackOut(t *testing.T) {
	screen := New(Options{})
	screen.zoomVideoAt(200, 150, 2)
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerDown, Button: 0, X: 100, Y: 100})
	screen.zoomVideoAt(200, 150, 0.5) // clamp back to 1x and clear pan
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerMove, Button: 0, X: 180, Y: 160})
	if screen.videoPanning || screen.videoPanX != 0 || screen.videoPanY != 0 {
		t.Fatalf("panning continued at 1x: pan=(%v,%v) panning=%v", screen.videoPanX, screen.videoPanY, screen.videoPanning)
	}
	screen.handleVideoInput(mui.InputEvent{Kind: mui.InputPointerUp, Button: 0, X: 180, Y: 160})
}
