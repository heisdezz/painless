package viewer

import (
	"math"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func near(got, want float32) bool {
	return math.Abs(float64(got-want)) < 1e-3
}

// Zooming keeps the picture point under the pointer in place: pan' =
// pointer - (pointer - pan) * factor.
func TestVideoZoomIsAnchoredAtThePointer(t *testing.T) {
	screen := New(Options{})

	const x, y float32 = 200, 150
	screen.zoomVideoAt(x, y, videoZoomStep)
	if !near(screen.videoZoom, 1.25) {
		t.Fatalf("zoom = %v, want 1.25", screen.videoZoom)
	}
	if !near(screen.videoPanX, -50) || !near(screen.videoPanY, -37.5) {
		t.Fatalf("pan = (%v, %v), want (-50, -37.5)", screen.videoPanX, screen.videoPanY)
	}
	if !near((x-screen.videoPanX)/screen.videoZoom, x) ||
		!near((y-screen.videoPanY)/screen.videoZoom, y) {
		t.Fatal("the point under the pointer moved on zoom")
	}

	screen.zoomVideoAt(x, y, videoZoomStep)
	if !near(screen.videoZoom, 1.25*1.25) {
		t.Fatalf("zoom = %v, want 1.5625", screen.videoZoom)
	}
	if !near((x-screen.videoPanX)/screen.videoZoom, x) ||
		!near((y-screen.videoPanY)/screen.videoZoom, y) {
		t.Fatal("the point under the pointer moved on a second zoom")
	}

	// Zooming at a different point keeps that point's position, not the
	// first anchor's. MyGo gives the pointer in element-local coords.
	const movedX float32 = 360
	before := screen.videoPanX + screen.videoZoom*movedX
	screen.zoomVideoAt(movedX, y, 1.25)
	if after := screen.videoPanX + screen.videoZoom*movedX; !near(after, before) {
		t.Fatalf("point at x=%v moved from %v to %v", movedX, before, after)
	}
}

// Zooming out retraces the zoom-in anchor, wherever the cursor is now: the
// frame does not drift, and it lands centered again at 1x.
func TestVideoZoomOutKeepsTheZoomInAnchor(t *testing.T) {
	screen := New(Options{})
	const x, y float32 = 200, 150

	screen.zoomVideoAt(x, y, 2)
	if !near(screen.videoZoom, 2) || !near(screen.videoPanX, -200) || !near(screen.videoPanY, -150) {
		t.Fatalf("zoom in = (%v, %v, %v)", screen.videoZoom, screen.videoPanX, screen.videoPanY)
	}

	// Cursor far from the original anchor — zoom out from there.
	screen.zoomVideoAt(900, 600, 0.6)
	if !near(screen.videoZoom, 1.2) || !near(screen.videoPanX, -40) || !near(screen.videoPanY, -30) {
		t.Fatalf("zoom out = (%v, %v, %v)", screen.videoZoom, screen.videoPanX, screen.videoPanY)
	}
	if !near(screen.videoPanX+screen.videoZoom*x, x) ||
		!near(screen.videoPanY+screen.videoZoom*y, y) {
		t.Fatal("zoom out displaced the picture point that was under the cursor")
	}

	screen.zoomVideoAt(700, 200, 0.5)
	if !near(screen.videoZoom, 1) || !near(screen.videoPanX, 0) || !near(screen.videoPanY, 0) {
		t.Fatalf("zoom out to 1x = (%v, %v, %v), want (1,0,0)", screen.videoZoom, screen.videoPanX, screen.videoPanY)
	}
}

// MyGo reports pointer coordinates in element-local space; pan is in screen
// space. After a drag, zooming in again must still anchor on the cursor's
// current screen position, not on where the drag began.
func TestVideoZoomInAfterPanAnchorsAtCursor(t *testing.T) {
	screen := New(Options{})
	const first, second float32 = 200, 400

	// A wheel event at screen offset 200 zooms 2x around that point.
	screen.zoomVideoAt(first, first, 2)

	// Feed a drag at screen positions; convert to the local coords MyGo
	// would report for the current zoom and pan.
	pointer := func(kind mui.InputKind, screenX, screenY float32, button int) {
		screen.mu.RLock()
		zoom, panX, panY := screen.videoZoom, screen.videoPanX, screen.videoPanY
		screen.mu.RUnlock()
		screen.handleVideoInput(mui.InputEvent{
			Kind: kind, Button: button,
			X: (screenX - panX) / zoom, Y: (screenY - panY) / zoom,
		})
	}
	pointer(mui.InputPointerDown, first, first, 0)
	pointer(mui.InputPointerMove, second, second, 0)
	pointer(mui.InputPointerUp, second, second, 0)

	// The drag moved the picture with the cursor: the original content
	// point is now under screen position 400 and pan is back to zero.
	if !near(screen.videoPanX, 0) || !near(screen.videoPanY, 0) {
		t.Fatalf("pan after drag = (%v, %v), want (0, 0)", screen.videoPanX, screen.videoPanY)
	}

	// Another wheel zoom at 400 keeps that point at 400.
	screen.mu.RLock()
	zoom, panX := screen.videoZoom, screen.videoPanX
	screen.mu.RUnlock()
	local := (second - panX) / zoom
	screen.zoomVideoAt(local, local, 1.25)
	screen.mu.RLock()
	afterZoom, afterPan := screen.videoZoom, screen.videoPanX
	screen.mu.RUnlock()
	if got := afterPan + afterZoom*local; !near(got, second) {
		t.Fatalf("zoom after pan moved the cursor point from %v to %v", second, got)
	}
}

func TestVideoZoomClampsAndReturnsToNaturalSize(t *testing.T) {
	screen := New(Options{})
	const x, y float32 = 200, 150

	screen.zoomVideoAt(x, y, 1000)
	if screen.videoZoom != maxVideoZoom {
		t.Fatalf("zoom = %v, want max %v", screen.videoZoom, maxVideoZoom)
	}

	screen.zoomVideoAt(x, y, 0.001)
	if !near(screen.videoZoom, 1) || !near(screen.videoPanX, 0) || !near(screen.videoPanY, 0) {
		t.Fatalf("zooming out past 1 left zoom=%v pan=(%v,%v)", screen.videoZoom, screen.videoPanX, screen.videoPanY)
	}
}

// Wheel notches and touchpad pinches reach the video element and zoom there;
// a precise two-finger scroll is left to the surrounding view.
func TestVideoWheelAndPinchZoom(t *testing.T) {
	screen, _, _ := newPlayingViewer(t, &mockThrottler{})
	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()

	tt.Wheel(400, 300, 0, -3)
	tt.Frame()
	screen.mu.RLock()
	afterWheel := screen.videoZoom
	screen.mu.RUnlock()
	if afterWheel <= 1 {
		t.Fatalf("wheel notch did not zoom in: zoom = %v", afterWheel)
	}

	tt.Pinch(400, 300, 2)
	tt.Frame()
	screen.mu.RLock()
	afterPinch := screen.videoZoom
	screen.mu.RUnlock()
	if afterPinch <= afterWheel {
		t.Fatalf("pinch did not zoom further: %v -> %v", afterWheel, afterPinch)
	}

	// Two-finger scroll is smooth and continuous, but still zooms.
	screen.mu.RLock()
	before := screen.videoZoom
	screen.mu.RUnlock()
	tt.Scroll(400, 300, 0, 40)
	tt.Frame()
	screen.mu.RLock()
	after := screen.videoZoom
	screen.mu.RUnlock()
	if after >= before {
		t.Fatalf("two-finger scroll did not zoom out: %v -> %v", before, after)
	}
}

// Zoom belongs to a frame, not to the next one: leaving or switching clips
// restores the natural size.
func TestVideoZoomResetsWhenPlaybackStops(t *testing.T) {
	screen := New(Options{})
	screen.zoomVideoAt(200, 150, 2)
	screen.stopVideo()

	screen.mu.RLock()
	zoom, panX, panY := screen.videoZoom, screen.videoPanX, screen.videoPanY
	screen.mu.RUnlock()
	if !near(zoom, 1) || !near(panX, 0) || !near(panY, 0) {
		t.Fatalf("zoom state after stop = (%v, %v, %v), want (1,0,0)", zoom, panX, panY)
	}
}
