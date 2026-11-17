package shell

import (
	"context"

	"painless/internal/domain"
	"painless/internal/scanner"
	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/thumbnail"
	"painless/internal/ui/screens/albums"
	"painless/internal/ui/screens/discover"
)

// libraryActions routes catalog commands to the screens that own them. Start
// Scan and Delete open Discover, where the work is reviewed before it touches
// the disk.
type libraryActions struct {
	store    *state.Store
	media    *storage.MediaRepo
	discover *discover.Screen
}

func (a libraryActions) OpenInViewer(domain.MediaItem) {}
func (a libraryActions) AssignToAlbum([]int64)         {}
func (a libraryActions) RemoveFromAlbum([]int64)       {}
func (a libraryActions) Reveal(domain.MediaItem)       {}
func (a libraryActions) Inspect(domain.MediaItem)      {}
func (a libraryActions) StartScan()                    { a.store.Navigate(state.ScreenDiscover) }

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

// HandleEvent reopens the library when the active drive changes.
func (l *libraryController) HandleEvent(event state.Event) {
	switch event.Kind {
	case state.EventDrivesChanged, state.EventActiveDriveChange:
		l.sync()
	case state.EventMediaChanged:
		if c := l.shell.Catalog(); c != nil {
			c.Reload()
		}
		l.shell.Albums().Reload()
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
	if ok && drive.ID == l.drive && drive.Path == l.root && l.db != nil {
		return
	}
	l.Close()
	if !ok {
		return
	}

	db, err := storage.Open(drive.Path, storage.OpenOptions{})
	if err != nil {
		l.shell.toaster.Error("Could not open library", err.Error())
		return
	}
	thumbs := thumbnail.New(thumbnail.Options{DriveRoot: drive.Path})
	mediaRepo, albumRepo := storage.NewMediaRepo(db), storage.NewAlbumRepo(db)
	l.db, l.thumbs, l.drive, l.root = db, thumbs, drive.ID, drive.Path

	opts := discover.Options{Store: l.store, DriveRoot: drive.Path, Update: l.update,
		AlbumSource: albumRepo, MediaStore: mediaRepo, AlbumStore: albumRepo}
	var screen *discover.Screen
	if !db.IsReadOnly() {
		opts.ScannerFactory = func(cfg scanner.Config, progress func(scanner.Progress)) discover.Scanner {
			return scanner.New(cfg, scanInserter{mediaRepo, screen.QueueDiscovered}, progress)
		}
	}
	screen = discover.New(opts)
	screen.ReloadAlbums()
	l.shell.SetDiscover(screen)
	l.shell.SetCatalog(mediaRepo, thumbs, libraryActions{store: l.store, media: mediaRepo, discover: screen})
	l.shell.SetAlbums(albums.New(albums.Options{
		Albums: albumRepository{albumRepo}, Media: mediaRepo, Thumbs: thumbs,
		DriveRoot: drive.Path, ReadOnly: db.IsReadOnly(),
	}))
}

// Close detaches the old screens immediately, then releases the database
// once canceled scanner and staging jobs have left it.
func (l *libraryController) Close() {
	done := l.shell.Discover().Close()
	l.shell.SetDiscover(discover.New(discover.Options{}))
	l.shell.SetAlbums(albums.New(albums.Options{}))
	l.shell.SetCatalog(nil, nil, libraryActions{store: l.store})
	if l.thumbs != nil {
		l.thumbs.Close()
	}
	if db := l.db; db != nil {
		go func() {
			<-done
			_ = db.Close()
		}()
	}
	l.db, l.thumbs, l.drive, l.root = nil, nil, "", ""
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
	inserted, err := r.MediaRepo.InsertScanBatch(ctx, items)
	if err == nil {
		r.queue(inserted)
	}
	return err
}
