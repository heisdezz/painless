package catalog

import (
	"context"
	"fmt"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

// pagedSource is a fake source that honors Limit and Offset like the real
// MediaRepo, so paging assertions see the same windowing.
type pagedSource struct {
	items []domain.MediaItem
	last  domain.MediaListParams
}

func newPagedSource(n int) *pagedSource {
	return &pagedSource{items: makePagedItems(n)}
}

func (p *pagedSource) List(_ context.Context, params domain.MediaListParams) (*domain.MediaListPage, error) {
	p.last = params
	items := p.items
	if params.Offset > len(items) {
		items = nil
	} else {
		items = items[params.Offset:]
	}
	if params.Limit > 0 && params.Limit < len(items) {
		items = items[:params.Limit]
	}
	return &domain.MediaListPage{Items: items, Total: len(p.items), Limit: params.Limit, Offset: params.Offset}, nil
}

func makePagedItems(n int) []domain.MediaItem {
	items := make([]domain.MediaItem, 0, n)
	for i := 1; i <= n; i++ {
		items = append(items, domain.MediaItem{
			ID:                  int64(i),
			FileHash:            fmt.Sprintf("hash-%d", i),
			CurrentRelativePath: fmt.Sprintf("photos/%03d.jpg", i),
			MimeType:            "image/jpeg",
			FileSize:            1024,
		})
	}
	return items
}

func TestCatalogPaginationPagesThroughResults(t *testing.T) {
	source := newPagedSource(130)
	screen := New(Options{Source: source})

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	waitFor(t, tt, func() bool { return len(screen.page.Items) > 0 })
	if !tt.HasText("001.jpg") || tt.HasText("061.jpg") {
		t.Fatalf("page 1 should show items 1-60: %q", tt.Texts())
	}
	if source.last.Limit != catalogPageSize || source.last.Offset != 0 {
		t.Fatalf("first page params = limit %d offset %d", source.last.Limit, source.last.Offset)
	}

	if err := tt.Click("2"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, func() bool { return source.last.Offset == catalogPageSize })
	if !tt.HasText("061.jpg") || tt.HasText("121.jpg") {
		t.Fatalf("page 2 should show items 61-120: pageNum=%d fetchKey=%q loadedFor=%q first=%d n=%d texts=%q",
			screen.pageNum, screen.fetchKey, screen.loadedFor, screen.page.Items[0].ID, len(screen.page.Items), firstTexts(tt, 40))
	}
}

func TestCatalogPaginationResetsOnSearch(t *testing.T) {
	source := newPagedSource(130)
	screen := New(Options{Source: source})

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	waitFor(t, tt, func() bool { return len(screen.page.Items) > 0 })
	if err := tt.Click("2"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, func() bool { return source.last.Offset == catalogPageSize })

	// A new committed query restarts at the first page.
	screen.query = "061"
	waitFor(t, tt, func() bool { return source.last.Search == "061" && source.last.Offset == 0 })
	if screen.pageNum != 1 {
		t.Fatalf("page after new search = %d, want 1", screen.pageNum)
	}
}

func TestCatalogPaginationHiddenOnSinglePage(t *testing.T) {
	screen := New(Options{Source: newPagedSource(10)})
	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	waitFor(t, tt, func() bool { return len(screen.page.Items) > 0 })
	if tt.HasText("1-10 of 10") {
		t.Fatalf("pager should be hidden on a single page: %q", tt.Texts())
	}
}

// firstTexts returns the first n texts for compact failure messages.
func firstTexts(tt *mui.Tester, n int) []string {
	texts := tt.Texts()
	if len(texts) > n {
		return texts[:n]
	}
	return texts
}

func waitFor(t *testing.T, tt *mui.Tester, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if done() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met within 2s")
}
