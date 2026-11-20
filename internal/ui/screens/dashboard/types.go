package dashboard

import (
	"context"

	"painless/internal/domain"
)

// Stats aggregates media counts and disk capacity numbers for the dashboard cards.
type Stats struct {
	TotalMedia    int64
	TotalAlbums   int64
	StorageUsed   uint64
	StorageTotal  uint64
	UsedPercent   float64
	AlbumLocation string
	Scanning      bool
	FoundCount    int
}

// Source feeds media items, albums, and aggregate stats for the active drive.
type Source interface {
	GetDashboardStats(ctx context.Context) (Stats, error)
	GetRecentMedia(ctx context.Context, limit int) ([]domain.MediaItem, error)
	GetRecentAlbums(ctx context.Context, limit int) ([]domain.AlbumSummary, error)
}

// DriveSource provides connected drive enumeration and selection.
type DriveSource interface {
	ListDrives(ctx context.Context) ([]domain.DriveInfo, error)
}

// Thumbnailer returns cached or freshly generated preview bytes.
type Thumbnailer interface {
	Get(ctx context.Context, item domain.MediaItem) ([]byte, error)
}

// Actions handles navigation and user-initiated commands on the dashboard.
type Actions interface {
	SelectDrive(driveID string)
	NavigateToMedia()
	NavigateToAlbums()
	NavigateToDiscover()
	OpenViewer(item domain.MediaItem, allItems []domain.MediaItem)
	OpenAlbum(album domain.AlbumSummary)
	DisconnectDrive()
	Refresh()
}

// NoopActions provides default empty actions.
type NoopActions struct{}

func (NoopActions) SelectDrive(string)                              {}
func (NoopActions) NavigateToMedia()                                {}
func (NoopActions) NavigateToAlbums()                               {}
func (NoopActions) NavigateToDiscover()                             {}
func (NoopActions) OpenViewer(domain.MediaItem, []domain.MediaItem) {}
func (NoopActions) OpenAlbum(domain.AlbumSummary)                   {}
func (NoopActions) DisconnectDrive()                                {}
func (NoopActions) Refresh()                                        {}

// Options configures a new dashboard screen.
type Options struct {
	Source      Source
	DriveSource DriveSource
	Thumbs      Thumbnailer
	// ThumbsEnabled reports the live display-thumbnails setting; while it
	// returns false, cards render without preview images and no fetch
	// workers are scheduled. Nil means thumbnails are always enabled.
	ThumbsEnabled func() bool
	Actions       Actions
	RefreshWindow func()
	ActiveDrive   *domain.DriveInfo
}
