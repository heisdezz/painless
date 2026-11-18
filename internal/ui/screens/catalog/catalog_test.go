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

func (f *fakeActions) OpenInViewer(item domain.MediaItem) { f.opened = item }
func (f *fakeActions) AssignToAlbum(ids []int64)          { f.assigned = ids }
func (f *fakeActions) RemoveFromAlbum([]int64)            {}
func (f *fakeActions) Delete([]int64)                     {}
func (f *fakeActions) Reveal(domain.MediaItem)            {}
func (f *fakeActions) Inspect(domain.MediaItem)           {}
func (f *fakeActions) StartScan()                         { f.scanned = true }

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

func TestCatalogRendersItems(t *testing.T) {
	screen, _, _, _ := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
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

	if err := tt.Click("Photos"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if screen.filter != 1 || source.last.Filter != domain.MediaFilterImages {
		t.Fatalf("filter = %d, query params = %+v", screen.filter, source.last)
	}
	if !tt.HasText("cat.jpg") || tt.HasText("cat.mp4") {
		t.Fatalf("texts after photo filter = %q", tt.Texts())
	}

	tt.Key(mui.Cmd, mui.KeyF)
	tt.Type("dog")
	time.Sleep(debounceDelay + 60*time.Millisecond)
	tt.Frame()
	if source.last.Search != "dog" {
		t.Fatalf("search param = %q", source.last.Search)
	}
	if !tt.HasText("dog.png") || tt.HasText("cat.jpg") {
		t.Fatalf("texts after search = %q", tt.Texts())
	}
}

func TestCatalogEmptyState(t *testing.T) {
	source := &fakeSource{}
	actions := &fakeActions{}
	screen := New(Options{Source: source, Actions: actions})
	tt := mui.NewTester(screen.View, 1000, 700)
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
	if err := tt.Click("cat.jpg"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if screen.selection.Len() == 0 {
		t.Fatal("clicking a card did not select it")
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

func TestCatalogClickOpensViewer(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
	if err := tt.Click("cat.jpg"); err != nil {
		t.Fatal(err)
	}
	if actions.opened.ID != 1 {
		t.Fatalf("expected cat.jpg opened in viewer, got %+v", actions.opened)
	}
}

func TestCatalogCmdClickSelectsWithoutOpening(t *testing.T) {
	screen, _, _, actions := newTestScreen()
	tt := mui.NewTester(screen.View, 1000, 700)
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
	tt.Frame()
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
