package state

import (
	"sync"
	"testing"
	"time"

	"painless/internal/domain"
)

func TestScreenValid(t *testing.T) {
	for _, s := range AllScreens {
		if !s.Valid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if Screen("bogus").Valid() {
		t.Error("bogus screen should not be valid")
	}
}

func TestDefaultScreen(t *testing.T) {
	s := New()
	if s.Screen() != ScreenDashboard {
		t.Errorf("default screen = %q, want dashboard", s.Screen())
	}
	if !s.SetScreen(ScreenAlbums) {
		t.Error("SetScreen to a new screen should report change")
	}
	if s.SetScreen(ScreenAlbums) {
		t.Error("SetScreen to the same screen should not report change")
	}
	if s.SetScreen("bogus") {
		t.Error("invalid screen should be rejected")
	}
	if s.Screen() != ScreenAlbums {
		t.Errorf("screen = %q after invalid set, want albums", s.Screen())
	}
}

func TestSetDrivesFallback(t *testing.T) {
	s := New()
	d1 := domain.DriveInfo{ID: "d1", Name: "One", HasLibrary: true}
	d2 := domain.DriveInfo{ID: "d2", Name: "Two"}

	prev, present := s.SetDrives([]domain.DriveInfo{d1, d2})
	if prev != "" || present {
		t.Fatalf("first SetDrives: prev=%q present=%v, want empty/true-false", prev, present)
	}
	if s.ActiveDriveID() != "d1" {
		t.Errorf("active = %q, want d1 (prefers HasLibrary)", s.ActiveDriveID())
	}

	// Select d2 explicitly, then remove it: fallback returns to d1.
	s.SetActiveDrive("d2")
	prev, _ = s.SetDrives([]domain.DriveInfo{d1})
	if prev != "d2" {
		t.Errorf("prev = %q, want d2", prev)
	}
	if s.ActiveDriveID() != "d1" {
		t.Errorf("active after fallback = %q, want d1", s.ActiveDriveID())
	}

	// Remove everything: no active drive.
	prev, present = s.SetDrives(nil)
	if prev != "d1" || present {
		t.Errorf("empty SetDrives: prev=%q present=%v, want d1/false", prev, present)
	}
	if s.ActiveDriveID() != "" {
		t.Errorf("active = %q, want empty", s.ActiveDriveID())
	}
	if _, ok := s.ActiveDrive(); ok {
		t.Error("ActiveDrive should report absent with no drives")
	}
}

func TestSetActiveDrive(t *testing.T) {
	s := New()
	s.SetDrives([]domain.DriveInfo{{ID: "a"}, {ID: "b"}})
	if !s.SetActiveDrive("b") {
		t.Error("changing active drive should report change")
	}
	if s.SetActiveDrive("b") {
		t.Error("re-selecting the same drive should not report change")
	}
	if s.SetActiveDrive("missing") {
		t.Error("unknown drive should not be selected")
	}
	if s.ActiveDriveID() != "b" {
		t.Errorf("active = %q, want b", s.ActiveDriveID())
	}
}

func TestSystemBootDriveCannotBeSelected(t *testing.T) {
	s := New()
	efi := domain.DriveInfo{ID: "/boot/efi", Name: "efi", Path: "/boot/efi", HasLibrary: true}
	usb := domain.DriveInfo{ID: "/run/media/user/USB", Name: "USB", Path: "/run/media/user/USB"}

	// Auto-selection skips system boot drives even when HasLibrary is true.
	s.SetDrives([]domain.DriveInfo{efi, usb})
	if got := s.ActiveDriveID(); got != usb.ID {
		t.Fatalf("auto-selected active = %q, want %q", got, usb.ID)
	}

	// Explicit selection refuses system boot drives.
	if s.SetActiveDrive(efi.ID) {
		t.Fatal("SetActiveDrive should reject system boot drive")
	}
	if got := s.ActiveDriveID(); got != usb.ID {
		t.Fatalf("active = %q, want %q", got, usb.ID)
	}

	// When only a system boot drive is present, nothing is selected.
	s.SetDrives([]domain.DriveInfo{efi})
	if got := s.ActiveDriveID(); got != "" {
		t.Fatalf("active = %q, want empty", got)
	}
}

func TestDrivesCopy(t *testing.T) {
	s := New()
	s.SetDrives([]domain.DriveInfo{{ID: "a"}})
	drives := s.Drives()
	drives[0].ID = "mutated"
	if s.Drives()[0].ID != "a" {
		t.Error("Drives should return a copy")
	}
}

func TestSelectionOrderAndMembership(t *testing.T) {
	sel := NewSelection()
	sel.Add(5)
	sel.Add(3)
	sel.Add(5) // duplicate ignored
	if got := sel.List(); len(got) != 2 || got[0] != 5 || got[1] != 3 {
		t.Errorf("List = %v, want [5 3]", got)
	}
	if !sel.Has(5) || sel.Has(4) {
		t.Error("membership broken")
	}
	if sel.Count() != 2 {
		t.Errorf("Count = %d, want 2", sel.Count())
	}
	if sel.Toggle(5) {
		t.Error("Toggle of selected id should return false (now deselected)")
	}
	if sel.Has(5) {
		t.Error("5 should be removed after toggle")
	}
	if got := sel.List(); len(got) != 1 || got[0] != 3 {
		t.Errorf("List after toggle = %v, want [3]", got)
	}
	if !sel.Toggle(7) {
		t.Error("Toggle of unselected id should return true (now selected)")
	}
	if sel.Anchor() != 7 {
		t.Errorf("Anchor = %d, want 7", sel.Anchor())
	}
}

func TestSelectionSetAndRange(t *testing.T) {
	sel := NewSelection()
	sel.Set([]int64{9, 2, 4})
	if got := sel.List(); len(got) != 3 || got[0] != 9 {
		t.Errorf("List = %v, want [9 2 4]", got)
	}
	if sel.Anchor() != 4 {
		t.Errorf("Anchor after Set = %d, want 4 (last)", sel.Anchor())
	}
	sel.AddRange(2, 6)
	if got := sel.List(); len(got) != 6 {
		t.Errorf("List after AddRange = %v, want 6 items", got)
	}
	sel.Clear()
	if sel.Count() != 0 || sel.Anchor() != 0 {
		t.Error("Clear should empty selection and reset anchor")
	}
}

func TestSelectionRetainAll(t *testing.T) {
	sel := NewSelection()
	sel.Set([]int64{1, 2, 3, 4})
	sel.RetainAll([]int64{2, 4, 99})
	got := sel.List()
	if len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Errorf("List after RetainAll = %v, want [2 4]", got)
	}
}

func TestJobLifecycle(t *testing.T) {
	s := New()
	s.StartJob("scan1", "scan", "Scanning drive")
	j, ok := s.Job("scan1")
	if !ok || !j.Active() || !j.Indeterminate {
		t.Fatalf("fresh job = %+v ok=%v, want running+indeterminate", j, ok)
	}

	s.UpdateJob("scan1", 0.5, false, "420 files")
	j, _ = s.Job("scan1")
	if j.Progress != 0.5 || j.Indeterminate || j.Detail != "420 files" {
		t.Errorf("updated job = %+v", j)
	}

	// Progress clamping.
	s.UpdateJob("scan1", 1.7, false, "")
	j, _ = s.Job("scan1")
	if j.Progress != 1 {
		t.Errorf("progress not clamped: %v", j.Progress)
	}

	s.FinishJob("scan1", JobDone, "complete")
	j, _ = s.Job("scan1")
	if j.Active() || j.FinishedAt == nil {
		t.Errorf("finished job = %+v", j)
	}

	// Updates after finish are no-ops.
	s.UpdateJob("scan1", 0, false, "late")
	s.FinishJob("scan1", JobFailed, "late")
	j, _ = s.Job("scan1")
	if j.Status != JobDone || j.Detail != "complete" {
		t.Errorf("job mutated after finish: %+v", j)
	}

	// Unknown job updates are no-ops.
	s.UpdateJob("nope", 1, false, "x")
	s.FinishJob("nope", JobDone, "x")
	if _, ok := s.Job("nope"); ok {
		t.Error("unknown job should not exist")
	}
}

func TestJobsOrderAndHistory(t *testing.T) {
	s := New()
	s.StartJob("j1", "scan", "one")
	time.Sleep(2 * time.Millisecond)
	s.StartJob("j2", "thumb", "two")
	s.FinishJob("j1", JobDone, "")

	// Oldest-first active jobs, finished history trimmed.
	jobs := s.Jobs(0)
	if len(jobs) != 1 || jobs[0].ID != "j2" {
		t.Errorf("Jobs(0) = %+v, want only running j2", jobs)
	}
	jobs = s.Jobs(5)
	if len(jobs) != 2 || jobs[0].ID != "j2" || jobs[1].ID != "j1" {
		t.Errorf("Jobs(5) = %+v, want [j2 j1]", jobs)
	}
	if !s.AnyActive() {
		t.Error("AnyActive should be true with j2 running")
	}
	s.FinishJob("j2", JobDone, "")
	if s.AnyActive() {
		t.Error("AnyActive should be false with only finished jobs")
	}
	s.StartJob("j3", "scan", "three")
	if !s.AnyActive() {
		t.Error("AnyActive should be true with running job")
	}
}

func TestBusSubscribePublishCancel(t *testing.T) {
	bus := NewBus()
	if bus.SubscriberCount() != 0 {
		t.Fatal("fresh bus should have no subscribers")
	}

	subA := bus.Subscribe(8)
	subB := bus.Subscribe(8)
	if bus.SubscriberCount() != 2 {
		t.Fatalf("SubscriberCount = %d, want 2", bus.SubscriberCount())
	}

	bus.Publish(Event{Kind: EventToast, Toast: &Toast{Title: "hi"}})
	for i, sub := range []Subscription{subA, subB} {
		select {
		case ev := <-sub.C:
			if ev.Kind != EventToast || ev.Toast == nil || ev.Toast.Title != "hi" {
				t.Errorf("subscriber %d got %+v", i, ev)
			}
			if ev.At.IsZero() {
				t.Errorf("subscriber %d event missing timestamp", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out waiting for event", i)
		}
	}

	subB.Cancel()
	if bus.SubscriberCount() != 1 {
		t.Fatalf("SubscriberCount after cancel = %d, want 1", bus.SubscriberCount())
	}
	// Double cancel is safe.
	subB.Cancel()

	bus.Publish(Event{Kind: EventDrivesChanged})
	select {
	case ev := <-subA.C:
		if ev.Kind != EventDrivesChanged {
			t.Errorf("got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("subA timed out")
	}
	select {
	case ev, ok := <-subB.C:
		if ok {
			t.Errorf("cancelled subscriber received %+v", ev)
		}
	default:
	}

	subA.Cancel()
	if bus.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount = %d, want 0", bus.SubscriberCount())
	}
}

func TestBusSlowSubscriberDrops(t *testing.T) {
	bus := NewBus()
	sub := bus.Subscribe(1)
	// Buffer of 1: first event is buffered, second is dropped (nobody reads).
	bus.Publish(Event{Kind: EventDrivesChanged})
	bus.Publish(Event{Kind: EventMediaChanged})
	bus.Publish(Event{Kind: EventToast})

	ev := <-sub.C
	if ev.Kind != EventDrivesChanged {
		t.Errorf("first event = %v, want drives_changed", ev.Kind)
	}
	// No deadlock: publish returned even though the buffer stayed full.
}

func TestStorePublishers(t *testing.T) {
	st := NewStore()
	sub := st.Bus.Subscribe(64)

	st.ApplyDrives([]domain.DriveInfo{{ID: "a", HasLibrary: true}, {ID: "b"}})
	st.SetActiveDrive("b")
	st.Navigate(ScreenDiscover)
	st.ToggleSelect(3)
	st.RangeSelect(4, 6)
	st.ReplaceSelection([]int64{9})
	st.ClearSelection()
	st.StartJobTracked("job", "scan", "label")
	st.ProgressJob("job", 0.25, false, "step")
	st.CompleteJob("job", JobDone, "done")
	st.Notify(domain.LogLevelWarn, "title", "body")
	st.MediaChanged("rescan")
	st.StagingChanged("plan")

	seen := map[EventKind]int{}
	var driveChanges []string
	deadline := time.After(2 * time.Second)
	for seen[EventStagingChanged] == 0 {
		select {
		case ev := <-sub.C:
			seen[ev.Kind]++
			switch ev.Kind {
			case EventActiveDriveChange:
				driveChanges = append(driveChanges, ev.DriveID)
			case EventScreenChanged:
				if ev.Detail != string(ScreenDiscover) {
					t.Errorf("ScreenChanged Detail = %q", ev.Detail)
				}
			case EventJobFinished:
				if ev.Job == nil || ev.Job.Status != JobDone {
					t.Errorf("JobFinished payload = %+v", ev.Job)
				}
			case EventToast:
				if ev.Toast == nil || ev.Toast.Level != domain.LogLevelWarn {
					t.Errorf("Toast payload = %+v", ev.Toast)
				}
			}
		case <-deadline:
			t.Fatalf("timed out; seen = %v", seen)
		}
	}

	want := map[EventKind]int{
		EventDrivesChanged:     1,
		EventActiveDriveChange: 2, // initial fallback selection + explicit switch
		EventScreenChanged:     1,
		EventSelectionChanged:  4,
		EventJobStarted:        1,
		EventJobProgress:       1,
		EventJobFinished:       1,
		EventToast:             1,
		EventMediaChanged:      1,
		EventStagingChanged:    1,
	}
	for kind, n := range want {
		if seen[kind] != n {
			t.Errorf("%s seen %d times, want %d", kind, seen[kind], n)
		}
	}
	// First change is the initial fallback selection (drive a), second is
	// the explicit switch to b.
	if len(driveChanges) != 2 || driveChanges[0] != "a" || driveChanges[1] != "b" {
		t.Errorf("ActiveDriveChange IDs = %v, want [a b]", driveChanges)
	}

	// State side effects.
	if st.State.ActiveDriveID() != "b" {
		t.Errorf("active = %q, want b", st.State.ActiveDriveID())
	}
	if st.State.Screen() != ScreenDiscover {
		t.Errorf("screen = %q", st.State.Screen())
	}
	if st.Selections().Count() != 0 {
		t.Error("selection should be empty after ClearSelection")
	}
	if _, ok := st.State.Job("job"); !ok {
		t.Error("job record missing")
	}
}

func TestStoreSelectionsAliasing(t *testing.T) {
	st := NewStore()
	sel := st.Selections()
	sel.Add(42)
	if !st.State.selection.Has(42) {
		t.Error("Selections should alias the state selection")
	}
}

func TestConcurrentAccess(t *testing.T) {
	st := NewStore()
	sub := st.Bus.Subscribe(128)
	done := make(chan struct{})

	// Drain events so publishers never block.
	go func() {
		defer close(done)
		for range sub.C {
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				st.ApplyDrives([]domain.DriveInfo{{ID: "a"}, {ID: "b"}})
				st.SetActiveDrive("b")
				st.Navigate(ScreenLogs)
				st.ToggleSelect(int64(w*1000 + i))
				st.StartJobTracked("job-w", "scan", "w")
				st.ProgressJob("job-w", float64(i)/100, false, "x")
				st.CompleteJob("job-w", JobDone, "x")
				_ = st.State.Screen()
				_, _ = st.State.ActiveDrive()
				_ = st.State.Jobs(3)
				_ = st.Selections().List()
			}
		}(w)
	}
	wg.Wait()
	sub.Cancel()
	<-done
}

func TestBackScreen(t *testing.T) {
	st := NewStore()

	// No navigation yet: back falls back to the catalog.
	if got := st.State.BackScreen(); got != ScreenCatalog {
		t.Fatalf("initial back screen = %q, want catalog", got)
	}

	st.Navigate(ScreenAlbums)
	st.Navigate(ScreenViewer)
	if got := st.State.BackScreen(); got != ScreenAlbums {
		t.Fatalf("back screen = %q, want albums", got)
	}

	st.Navigate(st.State.BackScreen())
	if st.State.Screen() != ScreenAlbums {
		t.Fatalf("screen = %q, want albums", st.State.Screen())
	}
	if got := st.State.BackScreen(); got != ScreenViewer {
		t.Fatalf("back screen after returning = %q, want viewer", got)
	}
}

func TestAutoplayEnabledToggle(t *testing.T) {
	st := NewStore()
	if !st.AutoplayEnabled() {
		t.Error("AutoplayEnabled = false on a new store, want true")
	}
	st.SetAutoplayEnabled(false)
	if st.AutoplayEnabled() {
		t.Error("AutoplayEnabled = true after SetAutoplayEnabled(false)")
	}
	st.SetAutoplayEnabled(true)
	if !st.AutoplayEnabled() {
		t.Error("AutoplayEnabled = false after SetAutoplayEnabled(true)")
	}
}

func TestThumbnailsEnabledToggle(t *testing.T) {
	st := NewStore()
	if !st.ThumbnailsEnabled() {
		t.Error("ThumbnailsEnabled = false on a new store, want true")
	}
	st.SetThumbnailsEnabled(false)
	if st.ThumbnailsEnabled() {
		t.Error("ThumbnailsEnabled = true after SetThumbnailsEnabled(false)")
	}
	st.SetThumbnailsEnabled(true)
	if !st.ThumbnailsEnabled() {
		t.Error("ThumbnailsEnabled = false after SetThumbnailsEnabled(true)")
	}
}
