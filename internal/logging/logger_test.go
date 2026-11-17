package logging

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"painless/internal/domain"
)

// newTestLogger returns a logger whose timestamps increment once per
// entry so ring ordering is observable.
func newTestLogger(capacity int) *Logger {
	l := New(Options{Capacity: capacity})
	base := time.Unix(0, 0).UTC()
	seq := 0
	l.nowFunc = func() time.Time {
		seq++
		return base.Add(time.Duration(seq) * time.Second)
	}
	return l
}

func TestRingBufferEvictsOldest(t *testing.T) {
	l := newTestLogger(3)
	l.Info("scanner", "one")
	l.Info("scanner", "two")
	l.Info("scanner", "three")
	l.Info("scanner", "four")

	entries := l.Entries()
	if len(entries) != 3 {
		t.Fatalf("count = %d, want 3", len(entries))
	}
	if entries[0].Message != "two" || entries[2].Message != "four" {
		t.Errorf("ring order = %q", []string{entries[0].Message, entries[1].Message, entries[2].Message})
	}
	if l.Count() != 3 {
		t.Errorf("Count = %d", l.Count())
	}
}

func TestLevelFiltering(t *testing.T) {
	l := New(Options{Capacity: 10, MinLevel: domain.LogLevelWarn})
	l.Info("a", "kept out")
	l.Warn("b", "kept %d", 1)
	l.Error("c", "kept 2")

	if got := l.Count(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if l.Dropped() != 1 {
		t.Errorf("dropped = %d, want 1", l.Dropped())
	}
	if l.MinLevel() != domain.LogLevelWarn {
		t.Errorf("min level = %q", l.MinLevel())
	}

	l.SetMinLevel(domain.LogLevelInfo)
	l.Info("a", "now kept")
	if got := l.Count(); got != 3 {
		t.Errorf("count after level change = %d, want 3", got)
	}
}

func TestQuerySearchAndLevel(t *testing.T) {
	l := newTestLogger(20)
	l.Info("scanner", "walked /photos")
	l.Warn("thumbnail", "slow encode for /photos/x.raw")
	l.Error("gdrive", "upload failed")
	l.Error("scanner", "read error on /music")

	got := l.Query(Filter{Search: "photos"})
	if len(got) != 2 {
		t.Errorf("search 'photos' matches = %d, want 2", len(got))
	}
	got = l.Query(Filter{Level: domain.LogLevelError})
	if len(got) != 2 || got[0].Context != "gdrive" {
		t.Errorf("error query = %+v", got)
	}
	got = l.Query(Filter{Search: "SCANNER", Level: domain.LogLevelError})
	if len(got) != 1 || got[0].Message != "read error on /music" {
		t.Errorf("combined query = %+v", got)
	}
	// Context participates in search.
	got = l.Query(Filter{Search: "gdrive"})
	if len(got) != 1 {
		t.Errorf("context search = %d, want 1", len(got))
	}
}

func TestQueryLimitKeepsNewest(t *testing.T) {
	l := newTestLogger(50)
	for i := 0; i < 10; i++ {
		l.Info("gen", "line %d", i)
	}
	got := l.Query(Filter{Limit: 3})
	if len(got) != 3 || got[0].Message != "line 7" || got[2].Message != "line 9" {
		t.Errorf("limited query = %+v", got)
	}
}

func TestNotifyCallback(t *testing.T) {
	calls := 0
	l := New(Options{Capacity: 4, Notify: func() { calls++ }})
	l.Info("a", "one")
	l.Warn("a", "two")
	if calls != 2 {
		t.Errorf("notify calls = %d, want 2", calls)
	}
	l.SetNotify(nil)
	l.Info("a", "three")
	if calls != 2 {
		t.Errorf("notify after unset = %d, want 2", calls)
	}
}

func TestExportText(t *testing.T) {
	l := newTestLogger(10)
	l.Info("scanner", "walked %d dirs", 12)
	l.Error("gdrive", "quota exceeded")
	var buf bytes.Buffer
	if err := l.Export(&buf, Filter{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("export lines = %d: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "INFO: walked 12 dirs") ||
		!strings.Contains(lines[0], "[scanner]") {
		t.Errorf("first line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "ERROR: quota exceeded") {
		t.Errorf("second line = %q", lines[1])
	}
}

func TestConcurrentAppends(t *testing.T) {
	l := New(Options{Capacity: 100})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				l.Info("worker", "g%d i%d", g, i)
			}
		}(g)
	}
	wg.Wait()
	if got := l.Count(); got != 100 {
		t.Errorf("count = %d, want 100 (capacity)", got)
	}
}

func TestClear(t *testing.T) {
	l := newTestLogger(5)
	l.Info("a", "x")
	l.Clear()
	if l.Count() != 0 || len(l.Entries()) != 0 {
		t.Errorf("after clear count = %d", l.Count())
	}
}
