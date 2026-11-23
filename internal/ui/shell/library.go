package shell

import (
	"context"
	"path/filepath"
	"time"

	"painless/internal/domain"
	"painless/internal/fileops"
	"painless/internal/scanner"
	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/thumbnail"
	"painless/internal/ui/screens/albums"
	"painless/internal/ui/screens/discover"
	"painless/internal/ui/screens/viewer"
)

// libraryActions routes catalog commands to the screens that own them. Start
// Scan and Delete open Discover, where the work is reviewed before it touches
// the disk. OpenInViewer and Inspect open the media viewer.
type libraryActions struct {
	store    *state.Store
	media    *storage.MediaRepo
	discover *discover.Screen
	viewer   *viewer.Screen
}

// openViewer loads an item into the viewer screen and navigates to it.
func (a libraryActions) openViewer(id int64) {
	if a.viewer != nil {
		a.viewer.Open(id)
	}
	a.store.Navigate(state.ScreenViewer)
}

func (a libraryActions) OpenInViewer(item domain.MediaItem) { a.openViewer(item.ID) }
func (a libraryActions) AssignToAlbum([]int64)              {}
func (a libraryActions) RemoveFromAlbum([]int64)            {}
func (a libraryActions) Reveal(domain.MediaItem)            {}
func (a libraryActions) Inspect(item domain.MediaItem)      { a.openViewer(item.ID) }
func (a libraryActions) StartScan()                         { a.store.Navigate(state.ScreenDiscover) }

// newViewer builds a viewer screen; nil dependencies yield a viewer that
// shows its empty state until the library controller binds a drive.
func newViewer(store *state.Store, redraw func(), root string, source viewer.Source, throttle viewer.Throttler) *viewer.Screen {
	return viewer.New(viewer.Options{
		DriveRoot: root,
		Source:    source,
		Throttler: throttle,
		Autoplay: func() bool {
			return store == nil || store.AutoplayEnabled()
		},
		RefreshWindow: redraw,
		OnClose: func() {
			if store != nil {
				store.Navigate(store.State.BackScreen())
			}
		},
	})
}

// Delete stages the chosen items for deletion rather than removing them now.
func (a libraryActions) Delete(ids []int64) {
	if a.media == nil || a.discover == nil || len(ids) == 0 {
		return
	}
	items := make([]domain.MediaItem, 0, len(ids))
	for _, id := range ids {
		if item, err := a.media.GetByID(context.Background(), id); err == nil && item != nil {
			items = append(items, *item)
		}
	}
	a.discover.StageDeletion(items)
	a.store.Navigate(state.ScreenDiscover)
}

// libraryController keeps the catalog pointed at the active drive's portable
// SQLite library. It runs on the UI thread: the window bridge delivers drive
// events there, so opening, closing and replacing services need no locks.
type libraryController struct {
	store  *state.Store
	shell  *Shell
	db     *storage.DB
	thumbs *thumbnail.Service
	drive  string
	root   string
	update func(func())
}

func newLibraryController(store *state.Store, shell *Shell) *libraryController {
	return &libraryController{store: store, shell: shell}
}

// logf appends one entry to the shell's logs console when a logger exists
// (tests may build the shell without one). Safe from any goroutine.
func (l *libraryController) logf(level domain.LogLevel, format string, args ...any) {
	if l.shell != nil && l.shell.logger != nil {
		l.shell.logger.Log(level, "library", format, args...)
	}
}

// HandleEvent reopens the library when the active drive changes.
func (l *libraryController) HandleEvent(event state.Event) {
	switch event.Kind {
	case state.EventDrivesChanged, state.EventActiveDriveChange:
		l.sync()
	case state.EventLibraryRestored:
		// A backup snapshot replaced the database file; force sync to
		// reopen it even though the active drive did not change.
		l.drive = ""
		l.sync()
	case state.EventMediaChanged:
		if c := l.shell.Catalog(); c != nil {
			c.Reload()
		}
		l.shell.Albums().Reload()
		l.shell.Dashboard().Reload()
	case state.EventScreenChanged:
		if l.store.State.Screen() == state.ScreenDiscover {
			l.shell.Discover().ReloadAlbums()
		}
	}
}

// sync binds the catalog to the active drive. With no drive it installs the
// empty catalog source so the library shows its empty state.
func (l *libraryController) sync() {
	drive, ok := l.store.State.ActiveDrive()
	libraryRoot, albumFolder := "", ""
	if ok {
		libraryRoot = loadLibraryRoot(drive.Path)
		if libraryRoot == "" {
			// Existing libraries keep their database at the drive root; the
			// album folder only becomes the library root once chosen.
			libraryRoot = drive.Path
			albumFolder = filepath.Join(drive.Path, "media", "albums")
		} else {
			albumFolder = libraryRoot
		}
	}
	if ok && drive.ID == l.drive && libraryRoot == l.root && l.db != nil {
		return
	}
	l.Close()
	if !ok {
		if l.drive != "" || l.root != "" {
			l.logf(domain.LogLevelInfo, "library detached: no active drive")
		}
		return
	}

	db, err := storage.Open(libraryRoot, storage.OpenOptions{})
	if err != nil {
		l.logf(domain.LogLevelError, "open library at %s failed: %v", libraryRoot, err)
		l.shell.toaster.Error("Could not open library", err.Error())
		return
	}
	thumbs := thumbnail.New(thumbnail.Options{DriveRoot: drive.Path, Store: l.store})
	mediaRepo, albumRepo := storage.NewMediaRepo(db), storage.NewAlbumRepo(db)
	l.db, l.thumbs, l.drive, l.root = db, thumbs, drive.ID, libraryRoot
	if db.IsReadOnly() {
		l.logf(domain.LogLevelWarn, "opened library at %s read-only (another instance may hold it)", libraryRoot)
	} else {
		l.logf(domain.LogLevelInfo, "opened library at %s", libraryRoot)
	}

	// Create the managed folders up front so the Discover scaffold banner only
	// reports genuinely absent folders instead of ones consolidation creates later.
	if !db.IsReadOnly() {
		if err := fileops.ScaffoldLibrary(drive.Path); err != nil {
			l.logf(domain.LogLevelError, "scaffold library folders failed: %v", err)
			l.shell.toaster.Error("Could not prepare library folders", err.Error())
		}
	}

	opts := discover.Options{Store: l.store, DriveRoot: drive.Path, Update: l.update,
		AlbumSource: albumRepo, MediaStore: mediaRepo, AlbumStore: albumRepo,
		AlbumFolder:          albumFolder,
		OnAlbumFolderChanged: func(path string) { l.selectLibraryRoot(drive.Path, path) },
	}
	opts.CatalogCount = cachedMediaCount(mediaRepo)
	var screen *discover.Screen
	if !db.IsReadOnly() {
		opts.ScannerFactory = func(cfg scanner.Config, progress func(scanner.Progress)) discover.Scanner {
			cfg.Albums = albumRepo
			return scanner.New(cfg, scanInserter{MediaRepo: mediaRepo, queue: screen.QueueDiscovered}, progress)
		}
		opts.AlbumScannerFactory = func(cfg scanner.Config, progress func(scanner.Progress)) discover.Scanner {
			cfg.Albums = albumRepo
			return scanner.New(cfg, scanInserter{MediaRepo: mediaRepo}, progress)
		}
	}
	screen = discover.New(opts)
	screen.ReloadAlbums()
	l.shell.SetDiscover(screen)
	l.shell.SetViewer(newViewer(l.store, l.redraw, drive.Path, mediaRepo, thumbs.Throttler()))
	l.shell.SetCatalog(mediaRepo, thumbs, libraryActions{store: l.store, media: mediaRepo, discover: screen, viewer: l.shell.Viewer()})
	l.syncDashboard(drive, mediaRepo, albumRepo, thumbs)
	l.shell.SetAlbums(albums.New(albums.Options{
		Albums: albumRepository{albumRepo}, Media: mediaRepo, Thumbs: thumbs,
		ThumbsEnabled: l.shell.thumbsEnabledFn(),
		DriveRoot:     drive.Path, ReadOnly: db.IsReadOnly(),
	}))
}

// selectLibraryRoot persists the folder chosen in Discover and reopens every
// screen against its catalog. Media paths stay relative to the drive mount.
func (l *libraryController) selectLibraryRoot(driveRoot, path string) {
	if !isWithinDrive(driveRoot, path) {
		l.shell.toaster.Error("Invalid library folder", "Choose a folder inside the active drive.")
		return
	}
	if err := saveLibraryRoot(driveRoot, path); err != nil {
		l.logf(domain.LogLevelError, "save library folder failed: %v", err)
		l.shell.toaster.Error("Could not save library folder", err.Error())
		return
	}
	l.logf(domain.LogLevelInfo, "library folder changed to %s", path)
	l.drive = "" // force sync to reopen at the new location
	l.sync()
}

// Close detaches the old screens immediately, then releases the database
// once canceled scanner and staging jobs have left it.
func (l *libraryController) Close() {
	done := l.shell.Discover().Close()
	l.shell.SetDiscover(discover.New(discover.Options{}))
	l.shell.SetAlbums(albums.New(albums.Options{}))
	l.shell.SetCatalog(nil, nil, libraryActions{store: l.store})
	l.shell.SetViewer(newViewer(l.store, l.redraw, "", nil, nil))
	l.resetDashboard()
	if l.thumbs != nil {
		l.thumbs.Close()
	}
	if db := l.db; db != nil {
		root := l.root
		go func() {
			<-done
			_ = db.Close()
		}()
		l.logf(domain.LogLevelInfo, "closed library at %s", root)
	}
	l.db, l.thumbs, l.drive, l.root = nil, nil, "", ""
}

// cachedMediaCount backs discover's CatalogCount hook with a 500ms-TTL
// cached count so the per-frame status panel does not query SQLite on every
// render. Only the UI thread invokes the returned closure.
func cachedMediaCount(media *storage.MediaRepo) func() int {
	var (
		queried time.Time
		total   int
	)
	return func() int {
		if time.Since(queried) < 500*time.Millisecond {
			return total
		}
		count, err := media.Count(context.Background())
		if err != nil {
			return total
		}
		queried, total = time.Now(), int(count)
		return total
	}
}

type albumRepository struct{ *storage.AlbumRepo }

func (r albumRepository) ListSummaries(ctx context.Context) ([]domain.AlbumSummary, error) {
	return r.List(ctx)
}

func (r albumRepository) Create(ctx context.Context, name, description string) (*domain.Album, error) {
	return r.AlbumRepo.Create(ctx, domain.CreateAlbumInput{Name: name, Description: &description})
}

func (r albumRepository) Update(ctx context.Context, album domain.Album) error {
	return r.AlbumRepo.Update(ctx, domain.EditAlbumInput{ID: album.ID, NewName: album.Name, Description: album.Description})
}

func (r albumRepository) Delete(ctx context.Context, id int64, soft bool) error {
	_, err := r.AlbumRepo.Delete(ctx, id, !soft)
	return err
}

func (r albumRepository) Merge(ctx context.Context, source, target int64) error {
	_, err := r.AlbumRepo.Merge(ctx, target, []int64{source})
	return err
}

type scanInserter struct {
	*storage.MediaRepo
	queue func([]domain.MediaItem)
}

func (r scanInserter) InsertBatch(ctx context.Context, items []domain.MediaItem) error {
	if r.queue == nil {
		_, err := r.MediaRepo.InsertBatch(ctx, items)
		return err
	}
	inserted, err := r.MediaRepo.InsertScanBatch(ctx, items)
	if err == nil {
		r.queue(inserted)
	}
	return err
}
