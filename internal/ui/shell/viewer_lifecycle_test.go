package shell

import (
	"sync"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/state"
	"painless/internal/ui/screens/viewer"
)

// lifecycleThrottler counts the resumes playback makes: stopping a video
// resumes background thumbnail work, so a resume proves playback stopped.
type lifecycleThrottler struct {
	mu      sync.Mutex
	resumes int
}

func (t *lifecycleThrottler) Pause() {}

func (t *lifecycleThrottler) Resume() {
	t.mu.Lock()
	t.resumes++
	t.mu.Unlock()
}

func (t *lifecycleThrottler) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.resumes
}

// The viewer is built only while it is the active screen: leaving it for
// the rail's navigation unmounts its stage, which stops playback.
func TestShellStopsViewerPlaybackOnLeavingScreen(t *testing.T) {
	shell, store := newTestShell()
	throttler := &lifecycleThrottler{}
	shell.SetViewer(viewer.New(viewer.Options{Throttler: throttler}))

	store.Navigate(state.ScreenViewer)
	tt := mui.NewTester(shell.View, 1200, 800)
	tt.Frame()
	if !tt.HasText("No media selected") {
		t.Fatalf("viewer texts = %q", tt.Texts())
	}
	if got := throttler.count(); got != 0 {
		t.Fatalf("stopped playback %d times while the screen was still up", got)
	}

	// Rebuilding the same screen must leave it alone.
	tt.Frame()
	if got := throttler.count(); got != 0 {
		t.Fatalf("stopped playback %d times on a rebuild of the same screen", got)
	}

	if err := tt.Click("Library"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if store.State.Screen() != state.ScreenCatalog {
		t.Fatalf("screen after clicking Library = %q", store.State.Screen())
	}
	if got := throttler.count(); got == 0 {
		t.Fatal("leaving the viewer screen left playback running")
	}
}
