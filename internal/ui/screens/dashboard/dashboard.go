package dashboard

import (
	"context"
	"fmt"
	"sync"
	"time"

	mui "github.com/egoist/mygo/ui"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	uitheme "painless/internal/ui/theme"
)

// Screen coordinates the main entry landing: drive selection or drive overview dashboard.
type Screen struct {
	source        Source
	driveSource   DriveSource
	thumbs        Thumbnailer
	actions       Actions
	refreshWindow func()

	mu           sync.RWMutex
	activeDrive  *domain.DriveInfo
	stats        Stats
	recentMedia  []domain.MediaItem
	recentAlbums []domain.AlbumSummary
	drives       []domain.DriveInfo
	loading        bool
	lastLoadedAt   time.Time
	bitmaps        map[string]*mui.Bitmap
	pendingBitmaps map[string]bool
}

// New creates an initialized Dashboard screen.
func New(opts Options) *Screen {
	actions := opts.Actions
	if actions == nil {
		actions = NoopActions{}
	}

	s := &Screen{
		source:        opts.Source,
		driveSource:   opts.DriveSource,
		thumbs:        opts.Thumbs,
		actions:       actions,
		refreshWindow: opts.RefreshWindow,
		activeDrive:   opts.ActiveDrive,
	}

	s.Reload()
	return s
}

// SetActiveDrive changes the active drive and refreshes the dashboard data.
func (s *Screen) SetActiveDrive(drive *domain.DriveInfo) {
	s.mu.Lock()
	s.activeDrive = drive
	s.stats = Stats{}
	s.recentMedia = nil
	s.recentAlbums = nil
	s.mu.Unlock()

	s.Reload()
}

// SetSource updates the data source and thumbnailer when a drive database is opened.
func (s *Screen) SetSource(src Source, thumbs Thumbnailer) {
	s.mu.Lock()
	s.source = src
	s.thumbs = thumbs
	s.mu.Unlock()

	s.Reload()
}

// Reload queries drives or dashboard metrics depending on whether a drive is active.
func (s *Screen) Reload() {
	s.mu.Lock()
	active := s.activeDrive
	src := s.source
	driveSrc := s.driveSource
	s.loading = true
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		if active == nil || active.Status == "unmounted" {
			// Fetch connected drives for drive picker
			var drives []domain.DriveInfo
			if driveSrc != nil {
				if dList, err := driveSrc.ListDrives(ctx); err == nil {
					drives = dList
				}
			}

			s.mu.Lock()
			s.drives = drives
			s.loading = false
			s.lastLoadedAt = time.Now()
			s.mu.Unlock()
			s.requestRedraw()
			return
		}

		// Fetch dashboard statistics, recent media, and recent albums
		var (
			stats        Stats
			recentMedia  []domain.MediaItem
			recentAlbums []domain.AlbumSummary
		)

		if src != nil {
			if st, err := src.GetDashboardStats(ctx); err == nil {
				stats = st
			}
			if media, err := src.GetRecentMedia(ctx, 8); err == nil {
				recentMedia = media
			}
			if albums, err := src.GetRecentAlbums(ctx, 4); err == nil {
				recentAlbums = albums
			}
		}

		s.mu.Lock()
		s.stats = stats
		s.recentMedia = recentMedia
		s.recentAlbums = recentAlbums
		s.loading = false
		s.lastLoadedAt = time.Now()
		s.mu.Unlock()
		s.requestRedraw()
	}()
}

// View renders the dashboard or drive picker screen for the current frame.
func (s *Screen) View(c *mui.Context) {
	uitheme.Apply(c)
	tk := lobetheme.TokensOf(c)

	s.mu.RLock()
	active := s.activeDrive
	s.mu.RUnlock()

	mui.Scroll(c).Grow(1).Basis(0).Children(func() {
		mui.Column(c).Fill().Padding(28, 28, 40).Gap(24).Children(func() {
			if active == nil || active.Status == "unmounted" {
				s.drivePicker(c, tk)
			} else {
				s.driveHeader(c, tk, active)
				s.statsPanel(c, tk, active)
				s.recentMediaSection(c, tk)
				s.recentAlbumsSection(c, tk)
			}
		})
	})
}

func (s *Screen) requestRedraw() {
	if s.refreshWindow != nil {
		s.refreshWindow()
	}
}

// formatBytes returns human-readable formatted storage units.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
