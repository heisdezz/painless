package catalog

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

type fakeSource struct {
	items []domain.MediaItem
	last  domain.MediaListParams
	calls int
}

func (f *fakeSource) List(_ context.Context, params domain.MediaListParams) (*domain.MediaListPage, error) {
	f.last = params
	f.calls++
	out := make([]domain.MediaItem, 0, len(f.items))
	for _, item := range f.items {
		if params.Filter == domain.MediaFilterImages && item.Type() != domain.MediaTypeImage {
			continue
		}
		if params.Filter == domain.MediaFilterVideos && item.Type() != domain.MediaTypeVideo {
			continue
		}
		if params.Search != "" && !strings.Contains(item.CurrentRelativePath, params.Search) {
			continue
		}
		out = append(out, item)
	}
	return &domain.MediaListPage{Items: out, Total: len(out)}, nil
}

type fakeThumbs struct{ calls atomic.Int64 }

func (f *fakeThumbs) Get(context.Context, domain.MediaItem) ([]byte, error) {
	f.calls.Add(1)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 200, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

type fakeActions struct {
	opened   domain.MediaItem
	assigned []int64
	scanned  bool
}

func (f *fakeActions) OpenInViewer(item domain.MediaItem, _ []domain.MediaItem) {
	f.opened = item
}
func (f *fakeActions) AssignToAlbum(ids []int64) { f.assigned = ids }
func (f *fakeActions) RemoveFromAlbum([]int64)   {}
func (f *fakeActions) Delete([]int64)            {}
func (f *fakeActions) Reveal(domain.MediaItem)   {}
func (f *fakeActions) Inspect(domain.MediaItem)  {}
func (f *fakeActions) StartScan()                { f.scanned = true }

func testItems() []domain.MediaItem {
	duration := 42
	return []domain.MediaItem{
		{ID: 1, FileHash: "a1", CurrentRelativePath: "photos/cat.jpg", MimeType: "image/jpeg", FileSize: 2048},
		{ID: 2, FileHash: "a2", CurrentRelativePath: "photos/dog.png", MimeType: "image/png", FileSize: 4096},
		{ID: 3, FileHash: "a3", CurrentRelativePath: "videos/cat.mp4", MimeType: "video/mp4", FileSize: 8192, DurationSeconds: &duration},
	}
}

func newTestScreen() (*Screen, *fakeSource, *fakeThumbs, *fakeActions) {
	source := &fakeSource{items: testItems()}
	thumbs := &fakeThumbs{}
	actions := &fakeActions{}
	return New(Options{Source: source, Thumbs: thumbs, Actions: actions}), source, thumbs, actions
}

// waitUntil pumps frames until cond holds. The catalog fetches its page off
// the UI thread now, so assertions must wait for the worker's result to drain.
func waitUntil(t *testing.T, tt *mui.Tester, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition never met")
}

// waitLoaded waits for the initial (or reloaded) page to arrive and render.
func waitLoaded(t *testing.T, tt *mui.Tester, s *Screen) {
	t.Helper()
	waitUntil(t, tt, func() bool { return s.fetchKey == "" && s.loadedFor != "" })
}

func TestCatalogRendersItems(t *testing.T) {
	screen, _, _, _ := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	if !tt.HasText("cat.jpg") || !tt.HasText("dog.png") || !tt.HasText("cat.mp4") {
		t.Fatalf("texts = %q", tt.Texts())
	}
	if !tt.HasText("3 items") {
		t.Fatalf("item count missing: %q", tt.Texts())
	}
	if !tt.HasText("0:42") {
		t.Fatalf("video duration badge missing: %q", tt.Texts())
	}
}

func TestCatalogFilterAndDebouncedSearch(t *testing.T) {
	screen, source, _, _ := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)

	if err := tt.Click("Photos"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, tt, func() bool {
		return source.last.Filter == domain.MediaFilterImages && !tt.HasText("cat.mp4")
	})
	if screen.filter != 1 {
		t.Fatalf("filter = %d, query params = %+v", screen.filter, source.last)
	}
	if !tt.HasText("cat.jpg") || tt.HasText("cat.mp4") {
		t.Fatalf("texts after photo filter = %q", tt.Texts())
	}

	tt.Key(mui.Cmd, mui.KeyF)
	tt.Type("dog")
	time.Sleep(debounceDelay + 60*time.Millisecond)
	waitUntil(t, tt, func() bool {
		return source.last.Search == "dog" && !tt.HasText("cat.jpg")
	})
	if !tt.HasText("dog.png") {
		t.Fatalf("texts after search = %q", tt.Texts())
	}
}

func TestCatalogEmptyState(t *testing.T) {
	source := &fakeSource{}
	actions := &fakeActions{}
	screen := New(Options{Source: source, Actions: actions})
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	if !tt.HasText("No media found") || !tt.HasText("Start Scan") {
		t.Fatalf("texts = %q", tt.Texts())
	}
	if err := tt.Click("Start Scan"); err != nil {
		t.Fatal(err)
	}
	if !actions.scanned {
		t.Fatal("Start Scan did not call the action handler")
	}
}

func TestCatalogContextMenu(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	rect, ok := tt.Find("card cat.jpg")
	if !ok {
		t.Fatalf("card not found: %q", tt.Texts())
	}
	tt.RightClickAt(rect.X+10, rect.Y+10)
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("Open in Viewer") || !tt.HasText("Inspect EXIF Metadata") {
		t.Fatalf("context menu texts = %q", tt.Texts())
	}
	if err := tt.Click("Open in Viewer"); err != nil {
		t.Fatal(err)
	}
	if actions.opened.ID != 1 {
		t.Fatalf("opened item = %+v", actions.opened)
	}
}

func TestCatalogSelectionAndBatchAction(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	rect, ok := tt.Find("card cat.jpg")
	if !ok {
		t.Fatalf("card not found: %q", tt.Texts())
	}
	tt.RightClickAt(rect.X+10, rect.Y+10)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Select"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if screen.selection.Len() == 0 {
		t.Fatal("selecting from the context menu did not select the item")
	}
	if !tt.HasText("1 selected") || !tt.HasText("Move to Album...") {
		t.Fatalf("texts after selection = %q", tt.Texts())
	}
	if err := tt.Click("Move to Album..."); err != nil {
		t.Fatal(err)
	}
	if len(actions.assigned) != 1 || actions.assigned[0] != 1 {
		t.Fatalf("assigned ids = %v", actions.assigned)
	}
}

func TestCatalogSelectModeAddsOnClick(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	rect, ok := tt.Find("card cat.jpg")
	if !ok {
		t.Fatalf("card not found: %q", tt.Texts())
	}
	tt.RightClickAt(rect.X+10, rect.Y+10)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Select"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// With a selection active, a plain click adds the item instead of opening
	// the viewer.
	if err := tt.Click("dog.png"); err != nil {
		t.Fatal(err)
	}
	if actions.opened.ID != 0 {
		t.Fatalf("select-mode click opened the viewer: %+v", actions.opened)
	}
	if screen.selection.Len() != 2 {
		t.Fatalf("selection = %v, want both items", screen.SelectedIDs())
	}
}

func TestCatalogClickOpensViewer(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	if err := tt.Click("cat.jpg"); err != nil {
		t.Fatal(err)
	}
	if actions.opened.ID != 1 {
		t.Fatalf("expected cat.jpg opened in viewer, got %+v", actions.opened)
	}
	// Opening the viewer must not leave the card looking selected when the
	// user navigates back.
	if screen.selection.Len() != 0 {
		t.Fatalf("plain click selected %v, want an empty selection", screen.SelectedIDs())
	}
}

func TestCatalogCmdClickSelectsWithoutOpening(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	if err := tt.ClickWith(mui.Cmd, "cat.jpg"); err != nil {
		t.Fatal(err)
	}
	if actions.opened.ID != 0 {
		t.Fatalf("Cmd-click should not open the viewer, got %+v", actions.opened)
	}
	if screen.selection.Len() == 0 {
		t.Fatal("Cmd-click should toggle the grid selection")
	}
}

func TestCatalogEnterOpensSelectedViewer(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	if err := tt.ClickWith(mui.Cmd, "dog.png"); err != nil {
		t.Fatal(err)
	}
	if actions.opened.ID != 0 {
		t.Fatalf("Cmd-click should not open the viewer, got %+v", actions.opened)
	}
	tt.Key(0, mui.KeyEnter)
	if actions.opened.ID != 2 {
		t.Fatalf("Enter should open the selected item, got %+v", actions.opened)
	}
}

func TestCatalogFilterBarFitsNarrowWindow(t *testing.T) {
	screen, _, _, _ := newTestScreen()
	const width = 922
	tt := mui.NewTester(screen.View, width, 700)
	waitLoaded(t, tt, screen)
	for _, name := range []string{"3 items", "Newest first"} {
		rect, ok := tt.Find(name)
		if !ok {
			t.Fatalf("%q not found: %q", name, tt.Texts())
		}
		if rect.X < 0 || rect.X+rect.W > width {
			t.Fatalf("%q rect %+v overflows window width %d", name, rect, width)
		}
	}
}

func TestCatalogLoadsThumbnails(t *testing.T) {
	screen, _, thumbs, _ := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(screen.bitmaps) == 0 {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if thumbs.calls.Load() == 0 {
		t.Fatal("thumbnail renderer was never called")
	}
	if len(screen.bitmaps) == 0 {
		t.Fatal("no thumbnail bitmap was decoded")
	}
}

// gatedSource blocks List until its release channel closes, standing in for a
// slow first read of a library on a cold USB drive.
type gatedSource struct {
	items   []domain.MediaItem
	release chan struct{}
}

func (g *gatedSource) List(_ context.Context, _ domain.MediaListParams) (*domain.MediaListPage, error) {
	<-g.release
	return &domain.MediaListPage{Items: g.items, Total: len(g.items)}, nil
}

func TestCatalogLoadDoesNotBlockFirstFrame(t *testing.T) {
	release := make(chan struct{})
	screen := New(Options{Source: &gatedSource{items: testItems(), release: release}})
	// List never returns until release closes; the first frame must still
	// render, showing the skeleton instead of blocking on the source.
	tt := mui.NewTester(screen.View, 1000, 700)
	if _, ok := tt.Find("catalog-loading"); !ok {
		t.Fatalf("skeleton grid missing while loading: %q", tt.Texts())
	}
	if tt.HasText("No media found") {
		t.Fatal("empty state showed while the first page was still loading")
	}
	close(release)
	waitLoaded(t, tt, screen)
	if !tt.HasText("cat.jpg") || !tt.HasText("dog.png") {
		t.Fatalf("cards missing after load: %q", tt.Texts())
	}
	if _, ok := tt.Find("catalog-loading"); ok {
		t.Fatal("skeleton grid still shown after the page loaded")
	}
}

func TestCatalogSkeletonShowsWhileLoading(t *testing.T) {
	release := make(chan struct{})
	screen := New(Options{Source: &gatedSource{items: testItems(), release: release}})
	tt := mui.NewTester(screen.View, 1000, 700)
	if _, ok := tt.Find("catalog-loading"); !ok {
		t.Fatalf("skeleton grid missing while loading: %q", tt.Texts())
	}
	if !tt.HasText("Loading…") {
		t.Fatalf("loading tag missing while loading: %q", tt.Texts())
	}
	close(release)
	waitLoaded(t, tt, screen)
	if !tt.HasText("3 items") {
		t.Fatalf("item count missing after load: %q", tt.Texts())
	}
	if tt.HasText("Loading…") {
		t.Fatalf("loading tag still shown after load: %q", tt.Texts())
	}
}

func TestCatalogRevealStaggersOnFirstLoad(t *testing.T) {
	screen, _, _, _ := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	waitLoaded(t, tt, screen)
	if screen.reveal == nil {
		t.Fatal("first load did not start the reveal choreography")
	}
	reveal := screen.reveal
	if err := tt.Click("Photos"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, tt, func() bool {
		return screen.fetchKey == "" && tt.HasText("cat.jpg") && !tt.HasText("cat.mp4")
	})
	if screen.reveal != reveal {
		t.Fatal("requery restarted the reveal choreography")
	}
}

// bitmapWait pumps frames until cond holds, for bitmapFor assertions made
// without a full tester window.
func bitmapWait(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("bitmap condition never met")
}

func TestCatalogBitmapForDisabledStopsFetching(t *testing.T) {
	enabled := false
	thumbs := &fakeThumbs{}
	screen := New(Options{Thumbs: thumbs, ThumbsEnabled: func() bool { return enabled }})
	item := testItems()[0]

	// While the setting is off, no bitmap is served and no worker is scheduled.
	if bitmap := screen.bitmapFor(item); bitmap != nil {
		t.Fatal("disabled setting served a bitmap")
	}
	time.Sleep(30 * time.Millisecond)
	if calls := thumbs.calls.Load(); calls != 0 {
		t.Fatalf("disabled setting scheduled %d fetches, want 0", calls)
	}

	// Enabling fetches and caches the decoded bitmap.
	enabled = true
	if bitmap := screen.bitmapFor(item); bitmap != nil {
		t.Fatal("first bitmapFor should be async, not synchronous")
	}
	bitmapWait(t, func() bool {
		screen.bitmapFor(item) // View pumps this every frame in production.
		return screen.bitmaps[item.FileHash] != nil
	})

	// Disabling again hides the cached bitmap without new fetches.
	enabled = false
	if bitmap := screen.bitmapFor(item); bitmap != nil {
		t.Fatal("disabled setting served a cached bitmap")
	}
	if calls := thumbs.calls.Load(); calls != 1 {
		t.Fatalf("disabled setting scheduled extra fetches: %d", calls)
	}

	// Re-enabling serves the cached bitmap instantly.
	enabled = true
	if bitmap := screen.bitmapFor(item); bitmap == nil {
		t.Fatal("re-enabled setting lost the cached bitmap")
	}
	if calls := thumbs.calls.Load(); calls != 1 {
		t.Fatalf("re-enable refetched: %d calls, want 1", calls)
	}
}

func TestCatalogBitmapForNilPredicateFetches(t *testing.T) {
	thumbs := &fakeThumbs{}
	screen := New(Options{Thumbs: thumbs})
	item := testItems()[0]

	if bitmap := screen.bitmapFor(item); bitmap != nil {
		t.Fatal("first bitmapFor should be async, not synchronous")
	}
	bitmapWait(t, func() bool {
		screen.bitmapFor(item) // View pumps this every frame in production.
		return screen.bitmaps[item.FileHash] != nil
	})
	if calls := thumbs.calls.Load(); calls != 1 {
		t.Fatalf("nil predicate blocked fetching: %d calls", calls)
	}
}
