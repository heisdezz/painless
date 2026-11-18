// Package albums provides the album gallery grid, detail view, cover thumbnails,
// and management dialogs (create, edit, delete, merge).
package albums

import (
	"context"
	"fmt"
	"sync"
	"time"

	mui "github.com/egoist/mygo/ui"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/ui/components/mediacard"
	uitheme "painless/internal/ui/theme"
)

// AlbumRepository abstracts persistence operations for albums.
type AlbumRepository interface {
	ListSummaries(ctx context.Context) ([]domain.AlbumSummary, error)
	GetByID(ctx context.Context, id int64) (*domain.Album, error)
	Create(ctx context.Context, name, description string) (*domain.Album, error)
	Update(ctx context.Context, album domain.Album) error
	Delete(ctx context.Context, id int64, softDelete bool) error
	Merge(ctx context.Context, sourceID, targetID int64) error
}

// MediaRepository abstracts queries for member items in an album.
type MediaRepository interface {
	List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error)
}

// memberPageSize is how many member cards one album detail page loads.
const memberPageSize = 60

// Reveal choreography of the cards' first entrance in the gallery and the
// detail strip: 18ms per card, the whole wave capped at 360ms of delays, each
// card rising for 300ms. Matches the catalog's reveal.
const (
	revealPer      = 18 * time.Millisecond
	revealMax      = 360 * time.Millisecond
	revealDuration = 300 * time.Millisecond
)

// albumsResult carries one finished summary fetch from its worker goroutine
// to the UI thread.
type albumsResult struct {
	summaries []domain.AlbumSummary
	err       error
}

// detailResult carries one finished member fetch. The key identifies the
// album|page it was fetched for, so a stale result is dropped.
type detailResult struct {
	key   string
	items []domain.MediaItem
	total int
	err   error
}

// Thumbnailer generates or serves cached previews.
type Thumbnailer interface {
	Get(ctx context.Context, item domain.MediaItem) ([]byte, error)
}

// Options configures the album screen.
type Options struct {
	Albums        AlbumRepository
	Media         MediaRepository
	Thumbs        Thumbnailer
	RefreshWindow func()
	DriveRoot     string
	ReadOnly      bool
	// OnOpenItem, when set, is invoked when a member card is clicked; the
	// shell attaches the media viewer deep-link here.
	OnOpenItem func(item domain.MediaItem)
}

// Screen manages album browsing, member viewing, and modal lifecycle.
type Screen struct {
	albums        AlbumRepository
	media         MediaRepository
	thumbs        Thumbnailer
	refreshWindow func()
	driveRoot     string
	readOnly      bool
	onOpenItem    func(domain.MediaItem)

	// Member-thumbnail cache. bitmapMu guards the maps; workers never touch
	// *mui.Context, decoded bitmaps are only swapped in on the UI thread.
	bitmapMu sync.Mutex
	pending  map[string]bool
	fetched  map[string][]byte
	limits   chan struct{}
	services mui.Services
	bitmaps  map[string]*mui.Bitmap

	// Gallery & Detail state
	summaries     []domain.AlbumSummary
	selectedAlbum *domain.AlbumSummary
	detailItems   []domain.MediaItem
	detailTotal   int
	searchQuery   string
	loadErr       error

	// Async gallery & detail loads. The channels carry worker results to the
	// UI thread, which drains them at the top of every frame; the flags keep
	// the views on their skeleton states while a fetch is in flight.
	albumsLoading bool
	albumsChan    chan albumsResult
	galleryReveal *mediacard.Reveal

	detailFetchKey string
	detailLoading  bool
	detailChan     chan detailResult
	detailReveal   *mediacard.Reveal

	// memberPage is the 1-based member page; nav.Pagination mutates it in
	// place. membersFor caches which album+page the member list came from.
	// Both are used on the UI thread only.
	memberPage int
	membersFor string

	// Create Modal state
	createOpen bool
	createName string
	createDesc string
	createErr  string

	// Edit Modal state
	editOpen   bool
	editTarget *domain.AlbumSummary
	editName   string
	editDesc   string
	editErr    string

	// Delete Modal state
	deleteOpen   bool
	deleteTarget *domain.AlbumSummary
	deleteMode   string // "soft" or "permanent"
	deleteErr    string

	// Merge Modal state
	mergeOpen   bool
	mergeSource *domain.AlbumSummary
	mergeTarget string // album name or id string
	mergeErr    string

	mu sync.RWMutex
}

// New creates an initialized album screen.
func New(opts Options) *Screen {
	s := &Screen{
		albums:        opts.Albums,
		media:         opts.Media,
		thumbs:        opts.Thumbs,
		refreshWindow: opts.RefreshWindow,
		driveRoot:     opts.DriveRoot,
		readOnly:      opts.ReadOnly || opts.Albums == nil,
		deleteMode:    "soft",
		onOpenItem:    opts.OnOpenItem,
		pending:       make(map[string]bool),
		fetched:       make(map[string][]byte),
		limits:        make(chan struct{}, 4),
		bitmaps:       make(map[string]*mui.Bitmap),
		albumsChan:    make(chan albumsResult, 1),
		detailChan:    make(chan detailResult, 1),
	}
	s.Reload()
	return s
}

// SetOnOpenItem attaches the open-in-viewer action after construction. Call
// before the window is shown; the callback runs on the UI thread.
func (s *Screen) SetOnOpenItem(fn func(item domain.MediaItem)) {
	s.onOpenItem = fn
}

// Reload refreshes the list of album summaries. The query runs off-thread —
// the library lives on the drive, so it must never block the UI thread — and
// the screen keeps showing the previous summaries until the fresh ones land.
func (s *Screen) Reload() {
	if s.albums == nil {
		return
	}
	s.mu.Lock()
	s.albumsLoading = true
	s.mu.Unlock()
	albums, ch, services := s.albums, s.albumsChan, s.services
	go func() {
		summaries, err := albums.ListSummaries(context.Background())
		ch <- albumsResult{summaries: summaries, err: err}
		services.Invalidate()
	}()
}

// drainResults applies finished gallery and member fetches. It runs at the top
// of every frame, before anything can start a new fetch, so a worker blocked
// on a one-slot channel is always released.
func (s *Screen) drainResults() {
	for {
		select {
		case r := <-s.albumsChan:
			s.mu.Lock()
			s.albumsLoading = false
			wasEmpty := len(s.summaries) == 0
			if r.err != nil {
				s.loadErr = r.err
			} else {
				s.summaries, s.loadErr = r.summaries, nil
				if wasEmpty && len(s.summaries) > 0 {
					s.galleryReveal = mediacard.NewReveal(time.Now(), revealPer, revealMax, revealDuration)
				}
			}
			s.mu.Unlock()
		case r := <-s.detailChan:
			s.mu.Lock()
			if r.key == s.detailFetchKey {
				s.detailFetchKey = ""
				s.membersFor = r.key
				s.detailLoading = false
				wasEmpty := len(s.detailItems) == 0
				if r.err == nil {
					s.detailItems, s.detailTotal = r.items, r.total
					if wasEmpty && len(r.items) > 0 {
						s.detailReveal = mediacard.NewReveal(time.Now(), revealPer, revealMax, revealDuration)
					}
				}
			}
			s.mu.Unlock()
		default:
			return
		}
	}
}

// View builds one frame of the album screen.
func (s *Screen) View(c *mui.Context) {
	s.services = c.Services()
	s.drainResults()
	uitheme.Apply(c)
	tk := lobetheme.TokensOf(c)

	container := mui.Column(c).Fill().Padding(28, 28, 40).Gap(16)
	container.Children(func() {
		if s.selectedAlbum == nil {
			s.galleryView(c, tk)
		} else {
			s.detailView(c, tk)
		}

		// Modals
		s.createModal(c, tk)
		s.editModal(c, tk)
		s.deleteModal(c, tk)
		s.mergeModal(c, tk)
	})
}

// SelectedAlbum returns the currently inspected album in detail view, or nil if in gallery view.
func (s *Screen) SelectedAlbum() *domain.AlbumSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedAlbum
}

// OpenAlbum opens the detail view for an album.
func (s *Screen) OpenAlbum(summary domain.AlbumSummary) {
	s.mu.Lock()
	s.selectedAlbum = &summary
	s.detailItems = nil
	s.detailTotal = 0
	s.memberPage = 1
	s.membersFor = ""
	s.mu.Unlock()

	s.loadDetailItems(summary.ID)
	s.requestRedraw()
}

// CloseDetail closes the detail view and returns to the album gallery.
func (s *Screen) CloseDetail() {
	s.mu.Lock()
	s.selectedAlbum = nil
	s.detailItems = nil
	s.mu.Unlock()
	s.Reload()
	s.requestRedraw()
}

// SetSearchQuery sets the search query filter thread-safely.
func (s *Screen) SetSearchQuery(q string) {
	s.mu.Lock()
	s.searchQuery = q
	s.mu.Unlock()
	s.requestRedraw()
}

// SearchQuery returns the active search query thread-safely.
func (s *Screen) SearchQuery() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.searchQuery
}

// loadDetailItems fetches one page of the album's members off-thread; the
// detail view shows its skeleton until the result drains. The album ID and
// page are read on the calling (UI) thread and captured by the worker.
func (s *Screen) loadDetailItems(albumID int64) {
	if s.media == nil {
		return
	}
	s.mu.Lock()
	page := s.memberPage
	s.detailLoading = true
	s.detailFetchKey = fmt.Sprintf("%d|%d", albumID, page)
	s.mu.Unlock()
	media, ch, services := s.media, s.detailChan, s.services
	go func() {
		result, err := media.List(context.Background(), domain.MediaListParams{
			AlbumID:   &albumID,
			SortBy:    domain.SortByDate,
			SortOrder: domain.SortOrderDesc,
			Limit:     memberPageSize,
			Offset:    (page - 1) * memberPageSize,
		})
		r := detailResult{key: fmt.Sprintf("%d|%d", albumID, page), err: err}
		if result != nil {
			r.items, r.total = result.Items, result.Total
		}
		ch <- r
		services.Invalidate()
	}()
}

// ensureDetailItems refetches members when the album or page changed, e.g.
// after nav.Pagination mutates memberPage in place, or when a fetch for the
// wanted key is already in flight.
func (s *Screen) ensureDetailItems(albumID int64) {
	want := fmt.Sprintf("%d|%d", albumID, s.memberPage)
	s.mu.RLock()
	current, fetching := s.membersFor, s.detailFetchKey
	s.mu.RUnlock()
	if current != want && fetching != want {
		s.loadDetailItems(albumID)
	}
}

func (s *Screen) requestRedraw() {
	if s.refreshWindow != nil {
		s.refreshWindow()
	}
}

// bitmapFor serves decoded member thumbnails from a UI-thread cache and loads
// missing ones on a bounded worker pool, mirroring the catalog screen.
func (s *Screen) bitmapFor(item domain.MediaItem) *mui.Bitmap {
	if bitmap, ok := s.bitmaps[item.FileHash]; ok {
		return bitmap
	}
	s.bitmapMu.Lock()
	if data, ok := s.fetched[item.FileHash]; ok {
		delete(s.fetched, item.FileHash)
		s.bitmapMu.Unlock()
		bitmap, err := mui.DecodeBitmap(data)
		if err != nil {
			return nil
		}
		s.bitmaps[item.FileHash] = bitmap
		return bitmap
	}
	if s.pending[item.FileHash] || s.thumbs == nil {
		s.bitmapMu.Unlock()
		return nil
	}
	s.pending[item.FileHash] = true
	services := s.services
	s.bitmapMu.Unlock()

	go func(hash string, media domain.MediaItem) {
		defer func() {
			s.bitmapMu.Lock()
			delete(s.pending, hash)
			s.bitmapMu.Unlock()
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
		s.bitmapMu.Lock()
		s.fetched[hash] = data
		s.bitmapMu.Unlock()
		services.Invalidate()
	}(item.FileHash, item)
	return nil
}
