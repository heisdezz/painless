package shell

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"painless/internal/domain"
	"painless/internal/state"
)

func newTestShell() (*Shell, *state.Store) {
	store := state.NewStore()
	return New(store), store
}

func TestShellNavigation(t *testing.T) {
	shell, store := newTestShell()
	tt := mui.NewTester(shell.View, 1200, 800)

	if err := tt.Click("Albums"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenAlbums {
		t.Fatalf("screen after clicking Albums = %q", store.State.Screen())
	}
	if !tt.HasText("No albums yet") || !tt.HasText("New Album") {
		t.Fatalf("texts after navigation = %q", tt.Texts())
	}

	if err := tt.Click("Library"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenCatalog {
		t.Fatalf("screen after clicking Library = %q", store.State.Screen())
	}
}

func TestShellDriveGaugeFromWorker(t *testing.T) {
	shell, store := newTestShell()
	tt := mui.NewTester(shell.View, 1200, 800)
	if !tt.HasText("No drive connected") {
		t.Fatalf("texts without a drive = %q", tt.Texts())
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		store.ApplyDrives([]domain.DriveInfo{{
			ID:             "t7",
			Name:           "Samsung T7",
			Type:           domain.DriveTypeExternal,
			Status:         domain.DriveStatusMounted,
			TotalBytes:     2 << 40,
			FreeBytes:      1 << 40,
			UsedPercentage: 50,
		}})
	}()
	<-done

	tt.Frame()
	if !tt.HasText("Samsung T7") || !tt.HasText("free of") {
		t.Fatalf("texts after drive publication = %q", tt.Texts())
	}
}

func TestShellToastEvent(t *testing.T) {
	shell, store := newTestShell()
	tt := mui.NewTester(shell.View, 1200, 800)

	event := state.Event{
		Kind:  state.EventToast,
		Toast: &state.Toast{Level: domain.LogLevelInfo, Title: "Scan complete", Body: "420 media files indexed"},
	}
	shell.Toaster().HandleEvent(event)
	tt.Frame()
	if !tt.HasText("Scan complete") || !tt.HasText("420 media files indexed") {
		t.Fatalf("texts after toast event = %q", tt.Texts())
	}

	// A worker only publishes plain data; the bridge delivers it as an event.
	sub := store.Bus.Subscribe(4)
	shell.Toaster().Publish(ToastMessage{
		Title: "Backup done", Type: feedback.ToastSuccess,
	})
	received := <-sub.C
	if received.Kind != state.EventToast || received.Toast == nil ||
		received.Toast.Title != "Backup done" {
		t.Fatalf("published event = %+v", received)
	}
	sub.Cancel()
}

func TestFormatBytes(t *testing.T) {
	cases := map[uint64]string{
		512:     "512 B",
		2048:    "2.0 KB",
		1 << 30: "1.0 GB",
		2 << 40: "2.0 TB",
	}
	for bytes, want := range cases {
		if got := formatBytes(bytes); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", bytes, got, want)
		}
	}
}
