package shell

import (
	"context"
	"sort"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/thumbnail"
	"painless/internal/ui/screens/dashboard"
	"painless/internal/ui/screens/viewer"
)

// Compile-time proof the adapters satisfy the dashboard screen contracts.
var (
	_ dashboard.Source      = dashboardSource{}
	_ dashboard.DriveSource = dashboardDrives{}
	_ dashboard.Actions     = dashboardActions{}
)

// redraw schedules an empty UI-thread frame through Window.Update; it is
// a no-op before the window exists (tests).
func (l *libraryController) redraw() {
	if l.update != nil {
		l.update(func() {})
	}
}

// dashboardSource feeds the dashboard from the active drive's repositories.
type dashboardSource struct {
	store       *state.Store
	media       *storage.MediaRepo
	albums      *storage.AlbumRepo
	albumFolder func() string
}

// GetDashboardStats aggregates media counts, album count, disk usage and
// the album storage location for the stat cards.
func (d dashboardSource) GetDashboardStats(ctx context.Context) (dashboard.Stats, error) {
	count, err := d.media.Count(ctx)
	if err != nil {
		return dashboard.Stats{}, err
	}
	stats := dashboard.Stats{TotalMedia: count}

	albums, err := d.albums.List(ctx)
	if err != nil {
		return dashboard.Stats{}, err
	}
	stats.TotalAlbums = int64(len(albums))
	if d.albumFolder != nil {
		stats.AlbumLocation = d.albumFolder()
	}

	if d.store != nil {
		if d.store != nil {
			if drive, ok := d.store.State.ActiveDrive(); ok {
				stats.StorageTotal = drive.TotalBytes
				stats.StorageUsed = drive.TotalBytes - drive.FreeBytes
				stats.UsedPercent = drive.UsedPercentage
			}
		}
	}
	return stats, nil
}

// GetRecentMedia returns the most recently added media items for the carousel.
func (d dashboardSource) GetRecentMedia(ctx context.Context, limit int) ([]domain.MediaItem, error) {
	page, err := d.media.List(ctx, domain.MediaListParams{
		Limit:     limit,
		SortBy:    domain.SortByAdded,
		SortOrder: domain.SortOrderDesc,
	})
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// GetRecentAlbums returns the most recently created albums.
func (d dashboardSource) GetRecentAlbums(ctx context.Context, limit int) ([]domain.AlbumSummary, error) {
	albums, err := d.albums.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(albums, func(i, j int) bool {
		return albums[i].CreatedAt.After(albums[j].CreatedAt)
	})
	if len(albums) > limit {
		albums = albums[:limit]
	}
	return albums, nil
}

// dashboardDrives enumerates connected drives for the picker grid.
type dashboardDrives struct {
	store *state.Store
}

// ListDrives returns the currently detected drives.
func (d dashboardDrives) ListDrives(ctx context.Context) ([]domain.DriveInfo, error) {
	return d.store.State.Drives(), nil
}

// dashboardActions routes dashboard commands into the store.
type dashboardActions struct {
	store  *state.Store
	shell  *Shell
	viewer *viewer.Screen
	reload func() // refreshes the dashboard data itself
}

func (a dashboardActions) SelectDrive(driveID string) { a.store.SetActiveDrive(driveID) }
func (a dashboardActions) NavigateToMedia()           { a.store.Navigate(state.ScreenCatalog) }
func (a dashboardActions) NavigateToAlbums()          { a.store.Navigate(state.ScreenAlbums) }
func (a dashboardActions) NavigateToDiscover()        { a.store.Navigate(state.ScreenDiscover) }

// OpenViewer deep-links into the viewer screen for the chosen item.
func (a dashboardActions) OpenViewer(item domain.MediaItem, all []domain.MediaItem) {
	if a.viewer != nil {
		a.viewer.OpenItem(item, all)
	}
	a.store.Navigate(state.ScreenViewer)
}

func (a dashboardActions) OpenAlbum(album domain.AlbumSummary) {
	if a.shell != nil {
		if alb := a.shell.Albums(); alb != nil {
			alb.OpenAlbum(album)
		}
	}
	a.store.Navigate(state.ScreenAlbums)
}

func (a dashboardActions) DisconnectDrive() {
	a.store.ClearActiveDrive()
	if a.reload != nil {
		a.reload()
	}
}

func (a dashboardActions) Refresh() {
	if a.reload != nil {
		a.reload()
	}
}

// syncDashboard rebuilds the dashboard around the active drive's
// repositories. Called from libraryController.sync on every drive change.
func (l *libraryController) syncDashboard(drive domain.DriveInfo, mediaRepo *storage.MediaRepo, albumRepo *storage.AlbumRepo, thumbs *thumbnail.Service) {
	screen := dashboard.New(dashboard.Options{
		Source: dashboardSource{
			store:       l.store,
			media:       mediaRepo,
			albums:      albumRepo,
			albumFolder: l.shell.Discover().AlbumFolder,
		},
		DriveSource:   dashboardDrives{store: l.store},
		Thumbs:        thumbs,
		ThumbsEnabled: l.shell.thumbsEnabledFn(),
		Actions: dashboardActions{
			store:  l.store,
			shell:  l.shell,
			viewer: l.shell.Viewer(),
			reload: func() { l.shell.Dashboard().Reload() },
		},
		RefreshWindow: l.redraw,
		ActiveDrive:   &drive,
	})
	l.shell.SetDashboard(screen)
}

// resetDashboard drops back to the drive picker, keeping the detected
// drives listed. Called from libraryController.Close on detach.
func (l *libraryController) resetDashboard() {
	l.shell.SetDashboard(dashboard.New(dashboard.Options{
		DriveSource:   dashboardDrives{store: l.store},
		RefreshWindow: l.redraw,
	}))
}
