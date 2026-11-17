// Package albums provides the album gallery grid, detail view, cover thumbnails,
// and management dialogs (create, edit, delete, merge).
package albums

import (
	"context"
	"sync"

	mui "github.com/egoist/mygo/ui"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
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
}

// Screen manages album browsing, member viewing, and modal lifecycle.
type Screen struct {
	albums        AlbumRepository
	media         MediaRepository
	thumbs        Thumbnailer
	refreshWindow func()
	driveRoot     string

	// Gallery & Detail state
	summaries     []domain.AlbumSummary
	selectedAlbum *domain.AlbumSummary
	detailItems   []domain.MediaItem
	detailTotal   int
	searchQuery   string
	loadErr       error

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
		deleteMode:    "soft",
	}
	s.Reload()
	return s
}

// Reload refreshes the list of album summaries.
func (s *Screen) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.albums != nil {
		summaries, err := s.albums.ListSummaries(context.Background())
		if err != nil {
			s.loadErr = err
		} else {
			s.summaries = summaries
			s.loadErr = nil
		}
	}
}

// View builds one frame of the album screen.
func (s *Screen) View(c *mui.Context) {
	uitheme.Apply(c)
	tk := lobetheme.TokensOf(c)

	container := mui.Column(c).Fill().Padding(20, 24).Gap(16)
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

func (s *Screen) loadDetailItems(albumID int64) {
	if s.media == nil {
		return
	}
	ctx := context.Background()
	page, err := s.media.List(ctx, domain.MediaListParams{
		AlbumID:   &albumID,
		SortBy:    domain.SortByDate,
		SortOrder: domain.SortOrderDesc,
		Limit:     100,
	})
	if err == nil && page != nil {
		s.mu.Lock()
		s.detailItems = page.Items
		s.detailTotal = page.Total
		s.mu.Unlock()
	}
}

func (s *Screen) requestRedraw() {
	if s.refreshWindow != nil {
		s.refreshWindow()
	}
}
