package viewer

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// newPlayingViewer opens the video item with a live player, as the pause
// button test does: the player is injected so no system decoder is needed.
// OpenItem stops whatever played before, so it resumes the throttler; the
// returned baseline counts those setup resumes, to measure deltas against.
func newPlayingViewer(t *testing.T, throttler *mockThrottler) (*Screen, *mui.VideoPlayer, int) {
	t.Helper()
	items := testMediaItems()
	screen := New(Options{Throttler: throttler})
	screen.OpenItem(items[1], items)

	player, err := mui.NewVideoPlayer(fakeVideoDecoder{})
	if err != nil {
		t.Fatal(err)
	}
	screen.mu.Lock()
	screen.videoPlayer = player
	screen.videoHash = items[1].FileHash
	screen.mu.Unlock()
	return screen, player, throttler.resumes()
}

// resumes reads the mock's resume count under its lock.
func (m *mockThrottler) resumes() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resumeCount
}

// Leaving the screen — a rail item, a shortcut, the window closing —
// unmounts the stage, so a playing video stops with it instead of running
// on behind the next screen.
func TestViewer_StopsPlaybackWhenTheScreenLeaves(t *testing.T) {
	throttler := &mockThrottler{}
	screen, player, before := newPlayingViewer(t, throttler)

	// The host builds the viewer only while it shows it, as the shell's
	// content does for the active screen.
	showing := true
	tt := mui.NewTester(func(c *mui.Context) {
		if showing {
			screen.View(c)
		}
	}, 1000, 700)
	tt.Frame()
	if !player.Playing() {
		t.Fatal("video should autoplay")
	}

	showing = false
	tt.Frame()
	tt.Frame()

	if player.Playing() {
		t.Fatal("playback kept running after the screen left")
	}
	screen.mu.RLock()
	open, hash := screen.videoPlayer, screen.videoHash
	screen.mu.RUnlock()
	if open != nil || hash != "" {
		t.Fatalf("player left open after the screen left: player=%v hash=%q", open, hash)
	}
	if throttler.resumes() == before {
		t.Fatal("thumbnails stay paused after the screen left")
	}
}

// Rebuilding the same screen must not stop playback: the stage keeps its
// identity from frame to frame.
func TestViewer_KeepsPlaybackWhileTheScreenStays(t *testing.T) {
	throttler := &mockThrottler{}
	screen, player, before := newPlayingViewer(t, throttler)

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	tt.Frame()
	tt.Frame()

	if !player.Playing() {
		t.Fatal("rebuilding the screen stopped playback")
	}
	if got := throttler.resumes(); got != before {
		t.Fatalf("throttler resumed %d times while the screen stayed up", got-before)
	}
}
