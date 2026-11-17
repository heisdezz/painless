// Package catalog renders the media library grid.
package catalog

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

// Source lists media items. *storage.MediaRepo satisfies it.
type Source interface {
	List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error)
}

// Thumbnailer returns cached or freshly generated preview bytes.
type Thumbnailer interface {
	Get(ctx context.Context, item domain.MediaItem) ([]byte, error)
}

// Actions handles the commands offered by cards, the context menu and the
// batch bar. Screens that are not built yet use NoopActions.
type Actions interface {
	OpenInViewer(item domain.MediaItem)
	AssignToAlbum(ids []int64)
	RemoveFromAlbum(ids []int64)
	Delete(ids []int64)
	Reveal(item domain.MediaItem)
	Inspect(item domain.MediaItem)
	StartScan()
}

// NoopActions discards commands until the other screens and services land.
type NoopActions struct{}

func (NoopActions) OpenInViewer(domain.MediaItem) {}
func (NoopActions) AssignToAlbum([]int64)         {}
func (NoopActions) RemoveFromAlbum([]int64)       {}
func (NoopActions) Delete([]int64)                {}
func (NoopActions) Reveal(domain.MediaItem)       {}
func (NoopActions) Inspect(domain.MediaItem)      {}
func (NoopActions) StartScan()                    {}

type emptySource struct{}

func (emptySource) List(context.Context, domain.MediaListParams) (*domain.MediaListPage, error) {
	return &domain.MediaListPage{}, nil
}

// Options configures a catalog screen.
type Options struct {
	Source  Source
	Thumbs  Thumbnailer
	Actions Actions
}

// Screen owns the catalog's filter, selection and thumbnail state. It is
// used on the UI thread; worker results are collected under mu.
type Screen struct {
	source  Source
	thumbs  Thumbnailer
	actions Actions

	query          string
	lastQuery      string
	committedQuery string
	queryChangedAt time.Time
	debounceTimer  *time.Timer
	filter         int
	lastFilter     int
	sort           int
	lastSort       int

	page      *domain.MediaListPage
	loadErr   error
	loadedFor string

	grid      mui.GridState
	selected  int
	selection mui.Selection[int64]

	mu       sync.Mutex
	pending  map[string]bool
	fetched  map[string][]byte
	limits   chan struct{}
	services mui.Services

	bitmaps map[string]*mui.Bitmap
}

const debounceDelay = 250 * time.Millisecond

// New creates a catalog screen. Missing dependencies get harmless defaults.
func New(opts Options) *Screen {
	if opts.Source == nil {
		opts.Source = emptySource{}
	}
	if opts.Actions == nil {
		opts.Actions = NoopActions{}
	}
	screen := &Screen{
		source:   opts.Source,
		thumbs:   opts.Thumbs,
		actions:  opts.Actions,
		selected: -1,
		pending:  make(map[string]bool),
		fetched:  make(map[string][]byte),
		limits:   make(chan struct{}, 4),
		bitmaps:  make(map[string]*mui.Bitmap),
	}
	screen.grid.Key = func(i int) any { return screen.page.Items[i].ID }
	screen.grid.Label = func(i int) string {
		path := screen.page.Items[i].CurrentRelativePath
		if slash := strings.LastIndexAny(path, `/\\`); slash >= 0 {
			return path[slash+1:]
		}
		return path
	}
	screen.grid.Selected = &screen.selected
	screen.grid.Selection = &screen.selection
	return screen
}

// View builds one frame of the catalog.
func (s *Screen) View(c *mui.Context) {
	s.services = c.Services()
	s.syncSearch()
	s.load()
	// Match the 28 DIP horizontal padding of the shell's screen heading so
	// the filter bar and grid line up with the title and keep off the edges.
	mui.Column(c).Fill().Padding(0, 28, 24).Gap(12).Children(func() {
		s.filterBar(c)
		s.gridView(c)
		s.actionBar(c)
	})
}

// syncSearch commits the query once typing has paused. The timer asks the
// window for another frame; only the UI thread changes the committed query.
func (s *Screen) syncSearch() {
	if s.query != s.lastQuery {
		s.lastQuery = s.query
		s.queryChangedAt = time.Now()
		if s.debounceTimer != nil {
			s.debounceTimer.Stop()
		}
		services := s.services
		s.debounceTimer = time.AfterFunc(debounceDelay, services.Invalidate)
	}
	if s.query != s.committedQuery && time.Since(s.queryChangedAt) >= debounceDelay {
		s.committedQuery = s.query
	}
}

func (s *Screen) load() {
	key := strings.Join([]string{s.committedQuery, strconv.Itoa(s.filter), strconv.Itoa(s.sort)}, "|")
	if key == s.loadedFor && s.page != nil {
		return
	}
	s.loadedFor = key
	params := domain.MediaListParams{
		Limit:     500,
		Search:    s.committedQuery,
		SortOrder: domain.SortOrderDesc,
		SortBy:    domain.SortByDate,
	}
	switch s.filter {
	case 1:
		params.Filter = domain.MediaFilterImages
	case 2:
		params.Filter = domain.MediaFilterVideos
	}
	switch s.sort {
	case 1:
		params.SortOrder = domain.SortOrderAsc
	case 2:
		params.SortBy, params.SortOrder = domain.SortByName, domain.SortOrderAsc
	case 3:
		params.SortBy = domain.SortBySize
	}
	page, err := s.source.List(context.Background(), params)
	s.page, s.loadErr = page, err
	if s.page == nil {
		s.page = &domain.MediaListPage{}
	}
	if s.selected >= len(s.page.Items) {
		s.selected = -1
	}
}

// Reload invalidates the cached page after an active library mutation.
func (s *Screen) Reload() { s.loadedFor = "" }

// SelectedIDs returns the IDs chosen in the grid.
func (s *Screen) SelectedIDs() []int64 {
	ids := make([]int64, 0, s.selection.Len())
	for id := range s.selection.All() {
		ids = append(ids, id)
	}
	return ids
}

// bitmapFor serves decoded thumbnails from a UI-thread cache and loads
// missing ones on a bounded worker pool. Workers never touch *mui.Context.
func (s *Screen) bitmapFor(item domain.MediaItem) *mui.Bitmap {
	if bitmap, ok := s.bitmaps[item.FileHash]; ok {
		return bitmap
	}
	s.mu.Lock()
	if data, ok := s.fetched[item.FileHash]; ok {
		delete(s.fetched, item.FileHash)
		s.mu.Unlock()
		bitmap, err := mui.DecodeBitmap(data)
		if err != nil {
			return nil
		}
		s.bitmaps[item.FileHash] = bitmap
		return bitmap
	}
	if s.pending[item.FileHash] || s.thumbs == nil {
		s.mu.Unlock()
		return nil
	}
	s.pending[item.FileHash] = true
	services := s.services
	s.mu.Unlock()

	go func(hash string, media domain.MediaItem) {
		defer func() {
			s.mu.Lock()
			delete(s.pending, hash)
			s.mu.Unlock()
		}()
		select {
		case s.limits <- struct{}{}:
			defer func() { <-s.limits }()
		default:
			return
		}
		data, err := s.thumbs.Get(context.Background(), media)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.fetched[hash] = data
		s.mu.Unlock()
		services.Invalidate()
	}(item.FileHash, item)
	return nil
}
