// Package catalog renders the media library grid.
package catalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/nav"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/ui/components/mediacard"
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
	OpenInViewer(item domain.MediaItem, all []domain.MediaItem)
	AssignToAlbum(ids []int64)
	RemoveFromAlbum(ids []int64)
	Delete(ids []int64)
	Reveal(item domain.MediaItem)
	Inspect(item domain.MediaItem)
	StartScan()
}

// NoopActions discards commands until the other screens and services land.
type NoopActions struct{}

func (NoopActions) OpenInViewer(domain.MediaItem, []domain.MediaItem) {}
func (NoopActions) AssignToAlbum([]int64)                             {}
func (NoopActions) RemoveFromAlbum([]int64)                           {}
func (NoopActions) Delete([]int64)                                    {}
func (NoopActions) Reveal(domain.MediaItem)                           {}
func (NoopActions) Inspect(domain.MediaItem)                          {}
func (NoopActions) StartScan()                                        {}

type emptySource struct{}

func (emptySource) List(context.Context, domain.MediaListParams) (*domain.MediaListPage, error) {
	return &domain.MediaListPage{}, nil
}

// pageResult carries one finished fetch from its worker goroutine to the UI
// thread. The key lets a stale result, one fetched before the query changed,
// be told apart from the one now wanted.
type pageResult struct {
	key  string
	page *domain.MediaListPage
	err  error
}

// Options configures a catalog screen.
type Options struct {
	Source  Source
	Thumbs  Thumbnailer
	Actions Actions
	// ThumbsEnabled reports the live display-thumbnails setting; while it
	// returns false, cards render without preview images and no fetch
	// workers are scheduled. Nil means thumbnails are always enabled.
	ThumbsEnabled func() bool
}

// Screen owns the catalog's filter, selection and thumbnail state. It is
// used on the UI thread; worker results are collected under mu.
type Screen struct {
	source        Source
	thumbs        Thumbnailer
	thumbsEnabled func() bool
	actions       Actions

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

	// pageNum is the 1-based catalog page; nav.Pagination mutates it in place.
	pageNum    int
	lastKey    string
	loadedPage int

	// fetchKey is the page now being fetched off-thread; results collects
	// finished fetches for the UI thread to drain; reveal choreographs the
	// cards' staggered entrance when the first page arrives.
	fetchKey     string
	results      chan pageResult
	reveal       *mediacard.Reveal
	skeletonGrid mui.GridState

	grid      mui.GridState
	selected  int
	pivot     int
	selection mui.Selection[int64]

	mu       sync.Mutex
	pending  map[string]bool
	fetched  map[string][]byte
	limits   chan struct{}
	services mui.Services

	bitmaps map[string]*mui.Bitmap
}

const (
	// catalogPageSize is how many items one catalog page loads from SQLite and
	// renders into the virtualized grid.
	catalogPageSize = 60

	debounceDelay = 250 * time.Millisecond

	// Reveal choreography of the cards' first entrance: 18ms per card, the
	// whole wave capped at 360ms of delays, each card rising for 300ms.
	revealPer      = 18 * time.Millisecond
	revealMax      = 360 * time.Millisecond
	revealDuration = 300 * time.Millisecond
)

// New creates a catalog screen. Missing dependencies get harmless defaults.
func New(opts Options) *Screen {
	if opts.Source == nil {
		opts.Source = emptySource{}
	}
	if opts.Actions == nil {
		opts.Actions = NoopActions{}
	}
	screen := &Screen{
		source:        opts.Source,
		thumbs:        opts.Thumbs,
		thumbsEnabled: opts.ThumbsEnabled,
		actions:       opts.Actions,
		selected:      -1,
		pivot:         -1,
		pending:       make(map[string]bool),
		fetched:       make(map[string][]byte),
		limits:        make(chan struct{}, 4),
		bitmaps:       make(map[string]*mui.Bitmap),
		page:          &domain.MediaListPage{},
		results:       make(chan pageResult, 1),
		pageNum:       1,
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
		s.pager(c)
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
	s.drainResults()
	base := pageKey(s.committedQuery, s.filter, s.sort)
	if base != s.lastKey {
		// A new query, filter or sort always restarts at the first page.
		s.pageNum = 1
		s.lastKey = base
	}
	key := base + "|" + strconv.Itoa(s.pageNum)
	if key == s.loadedFor || key == s.fetchKey {
		return
	}
	if s.pageNum != s.loadedPage {
		// The page-local grid cursor has no meaning on another page.
		s.selected, s.pivot = -1, -1
		s.loadedPage = s.pageNum
	}
	s.fetchKey = key
	params := s.listParams()
	source, results, services := s.source, s.results, s.services
	go func() {
		page, err := source.List(context.Background(), params)
		results <- pageResult{key: key, page: page, err: err}
		services.Invalidate()
	}()
}

// drainResults applies finished fetches. It runs before a new fetch starts so
// a worker blocked on the one-slot channel is released: with at most two
// fetches in flight, draining here keeps every send from sticking.
func (s *Screen) drainResults() {
	for {
		select {
		case r := <-s.results:
			if r.key != s.fetchKey {
				continue
			}
			wasEmpty := len(s.page.Items) == 0
			s.fetchKey = ""
			s.loadErr = r.err
			if r.page == nil {
				r.page = &domain.MediaListPage{}
			}
			s.page, s.loadedFor = r.page, r.key
			if s.selected >= len(s.page.Items) {
				s.selected = -1
			}
			// After deletions the stored page can sit past the end; snap back
			// to the last non-empty page and refetch it next frame.
			if len(s.page.Items) == 0 && s.page.Total > 0 && s.pageNum > 1 {
				if pages := (s.page.Total + catalogPageSize - 1) / catalogPageSize; pages > 0 && s.pageNum > pages {
					s.pageNum = pages
					s.loadedFor = ""
				}
			}
			if wasEmpty && len(s.page.Items) > 0 {
				s.reveal = mediacard.NewReveal(time.Now(), revealPer, revealMax, revealDuration)
			}
		default:
			return
		}
	}
}

// listParams builds the query the current filter and sort selection mean.
func (s *Screen) listParams() domain.MediaListParams {
	params := domain.MediaListParams{
		Limit:     catalogPageSize,
		Offset:    (s.pageNum - 1) * catalogPageSize,
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
	return params
}

// pageKey identifies a list query: committed search, filter and sort.
func pageKey(query string, filter, sort int) string {
	return strings.Join([]string{query, strconv.Itoa(filter), strconv.Itoa(sort)}, "|")
}

// Reload invalidates the cached page after an active library mutation.
func (s *Screen) Reload() { s.loadedFor = "" }

// pager pages through the whole result set below the grid. nav.Pagination
// mutates pageNum in place; the next frame refetches with the new offset.
func (s *Screen) pager(c *mui.Context) {
	if s.page == nil || s.page.Total <= catalogPageSize {
		return
	}
	mui.Row(c).Shrink(0).AlignItems(mui.Center).Children(func() {
		nav.Pagination(c, &s.pageNum, nav.PaginationProps{
			Total:            s.page.Total,
			PageSize:         catalogPageSize,
			HideOnSinglePage: true,
			Size:             lobetheme.Small,
			ShowTotal: func(total, from, to int) string {
				return fmt.Sprintf("%d-%d of %d", from, to, total)
			},
		})
	})
}

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
	if s.thumbsDisabled() {
		return nil
	}
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

// thumbsDisabled reports whether the display-thumbnails setting is off.
func (s *Screen) thumbsDisabled() bool {
	return s.thumbsEnabled != nil && !s.thumbsEnabled()
}
