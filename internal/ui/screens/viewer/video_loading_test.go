package viewer

import (
	"errors"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func TestVideoStageShowsLoadingWhileMounting(t *testing.T) {
	player, err := mui.NewVideoPlayer(fakeVideoDecoder{})
	if err != nil {
		t.Fatal(err)
	}
	items := testMediaItems()
	screen := New(Options{VideoOpener: func(string) (*mui.VideoPlayer, error) {
		return player, nil
	}})
	screen.OpenItem(items[1], items)
	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()

	if tt.HasText("Loading video…") {
		t.Fatal("spinner shown for a ready player")
	}

	screen.mu.Lock()
	screen.videoLoading = true
	screen.mu.Unlock()
	tt.Frame()
	if !tt.HasText("Loading video…") {
		t.Fatalf("spinner missing while mounting: %q", tt.Texts())
	}
	if tt.HasText("No video preview available") {
		t.Fatal("stage claimed there was no video while it was still loading")
	}

	screen.mu.Lock()
	screen.videoLoading = false
	screen.mu.Unlock()
	tt.Frame()
	if tt.HasText("Loading video…") {
		t.Fatal("spinner stayed up after mounting")
	}
}

func TestVideoStageStillReportsFailedOpen(t *testing.T) {
	items := testMediaItems()
	screen := New(Options{VideoOpener: func(string) (*mui.VideoPlayer, error) {
		return nil, errors.New("cannot open")
	}})
	screen.OpenItem(items[1], items)
	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()

	if !tt.HasText("No video preview available") {
		t.Fatalf("failed open did not report the missing preview: %q", tt.Texts())
	}
	if tt.HasText("Loading video…") {
		t.Fatal("spinner kept spinning after a failed open")
	}
}
