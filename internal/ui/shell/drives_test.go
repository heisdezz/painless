package shell

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"painless/internal/domain"
	"painless/internal/state"
)

type fakeDriveLister struct {
	mu     sync.Mutex
	drives []domain.DriveInfo
	err    error
	calls  atomic.Int64
}

func (f *fakeDriveLister) ListDrives(context.Context) ([]domain.DriveInfo, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.DriveInfo(nil), f.drives...), nil
}

func (f *fakeDriveLister) set(drives ...domain.DriveInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drives = drives
}

func (f *fakeDriveLister) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func testDrive(name string) domain.DriveInfo {
	return domain.DriveInfo{
		ID:             "drive:" + name,
		Name:           name,
		Type:           domain.DriveTypeExternal,
		Status:         domain.DriveStatusMounted,
		TotalBytes:     2 << 40,
		FreeBytes:      1 << 40,
		UsedPercentage: 50,
	}
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDriveControllerSeedsStoreAndShell(t *testing.T) {
	store := state.NewStore()
	shell := New(store)
	lister := &fakeDriveLister{drives: []domain.DriveInfo{testDrive("Samsung T7")}}
	controller := newDriveController(store, lister, shell.toaster, shell.logger, nil)
	shell.attachDrives(controller)
	controller.Start(context.Background())

	tt := mui.NewTester(shell.View, 1200, 800)
	if _, ok := store.State.ActiveDrive(); !ok {
		t.Fatal("startup scan did not put a drive in the store")
	}
	if !tt.HasText("Samsung T7") || !tt.HasText("free of") {
		t.Fatalf("shell texts after startup scan = %q", tt.Texts())
	}
}

func TestDriveControllerAppliesMonitorEvents(t *testing.T) {
	store := state.NewStore()
	shell := New(store)
	lister := &fakeDriveLister{}
	events := make(chan domain.DriveEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller := newDriveController(store, lister, shell.toaster, shell.logger, events)
	shell.attachDrives(controller)
	controller.Start(ctx)

	if lister.calls.Load() != 1 {
		t.Fatalf("startup scan calls = %d, want 1", lister.calls.Load())
	}

	lister.set(testDrive("New USB"))
	events <- domain.DriveEvent{Type: domain.DriveEventMounted, Drive: testDrive("New USB")}

	tt := mui.NewTester(shell.View, 1200, 800)
	waitFor(t, "monitor event to reach the shell", func() bool {
		tt.Frame()
		return lister.calls.Load() >= 2 && tt.HasText("New USB")
	})
}

func TestRefreshButtonScansDrives(t *testing.T) {
	store := state.NewStore()
	shell := New(store)
	lister := &fakeDriveLister{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller := newDriveController(store, lister, shell.toaster, shell.logger, nil)
	shell.attachDrives(controller)
	controller.Start(ctx)

	tt := mui.NewTester(shell.View, 1200, 800)
	if !tt.HasText("No drive connected") {
		t.Fatalf("texts before refresh = %q", tt.Texts())
	}

	lister.set(testDrive("Refreshed SSD"))
	if err := tt.Click("Refresh drives"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "refresh click to rescan", func() bool {
		tt.Frame()
		return lister.calls.Load() >= 2 && tt.HasText("Refreshed SSD")
	})
}

func TestRefreshReportsFailure(t *testing.T) {
	store := state.NewStore()
	shell := New(store)
	lister := &fakeDriveLister{err: errors.New("disk offline")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller := newDriveController(store, lister, shell.toaster, shell.logger, nil)
	shell.attachDrives(controller)
	controller.Start(ctx)

	sub := store.Bus.Subscribe(4)
	defer sub.Cancel()
	lister.setError(errors.New("rescan failed"))
	controller.Refresh(ctx)
	select {
	case event := <-sub.C:
		if event.Kind != state.EventToast || event.Detail != toastHintError {
			t.Fatalf("failure event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("no failure toast published")
	}
}

func TestToastPublishPreservesType(t *testing.T) {
	store := state.NewStore()
	toaster := newToaster(store)
	sub := store.Bus.Subscribe(8)
	defer sub.Cancel()

	toaster.Publish(ToastMessage{Title: "Saved", Type: feedback.ToastSuccess})
	event := <-sub.C
	if event.Kind != state.EventToast || event.Detail != toastHintSuccess {
		t.Fatalf("published event = %+v", event)
	}
	toaster.HandleEvent(event)
	if len(toaster.pending) != 1 || toaster.pending[0].Type != feedback.ToastSuccess {
		t.Fatalf("pending toasts = %+v", toaster.pending)
	}

	toaster.pending = nil
	toaster.Publish(ToastMessage{Title: "Removed", Type: feedback.ToastError})
	toaster.HandleEvent(<-sub.C)
	if len(toaster.pending) != 1 || toaster.pending[0].Type != feedback.ToastError {
		t.Fatalf("error toast = %+v", toaster.pending)
	}
}
