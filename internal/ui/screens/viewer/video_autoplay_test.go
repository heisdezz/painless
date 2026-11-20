package viewer

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func autoplayTestScreen(t *testing.T, autoplay func() bool) (*Screen, *mui.VideoPlayer) {
	t.Helper()
	items := testMediaItems()
	player, err := mui.NewVideoPlayer(fakeVideoDecoder{})
	if err != nil {
		t.Fatal(err)
	}
	screen := New(Options{
		Autoplay: autoplay,
		VideoOpener: func(string) (*mui.VideoPlayer, error) {
			return player, nil
		},
	})
	screen.OpenItem(items[1], items)
	return screen, player
}

func TestViewerAutoplayOffOpensPaused(t *testing.T) {
	screen, player := autoplayTestScreen(t, func() bool { return false })
	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()

	if player.Playing() {
		t.Fatal("video autoplayed with autoplay disabled")
	}
	screen.togglePlayPause()
	if !player.Playing() {
		t.Fatal("Play did not start the paused video")
	}
}

func TestViewerAutoplayOnByDefault(t *testing.T) {
	// No Autoplay predicate is the default: a video starts right away.
	screen, player := autoplayTestScreen(t, nil)
	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()

	if !player.Playing() {
		t.Fatal("video did not autoplay by default")
	}
	// Rebuilding the same screen does not pause it again.
	tt.Frame()
	if !player.Playing() {
		t.Fatal("rebuild paused an autoplaying video")
	}
}
