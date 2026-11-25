package shell

import (
	"context"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/state"
	"painless/internal/storage"
)

// TestAlbumMemberOpensViewer checks that the shell links album member cards
// to the media viewer: clicking a member in the albums screen deep-links into
// the viewer for that item.
func TestAlbumMemberOpensViewer(t *testing.T) {
	root := makeLibraryDrive(t, "album")
	store := state.NewStore()

	// Seed an album containing the library item before the controller binds
	// the drive, so the album detail view renders the member card.
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	repo := storage.NewMediaRepo(db)
	page, err := repo.List(context.Background(), domain.MediaListParams{Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("seed items = %+v, %v", page, err)
	}
	item := page.Items[0]
	album, err := storage.NewAlbumRepo(db).Create(context.Background(), domain.CreateAlbumInput{Name: "Trip"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.MoveToAlbum(context.Background(), album.ID, []int64{item.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	shell := New(store)
	library := newLibraryController(store, shell)
	store.ApplyDrives([]domain.DriveInfo{{
		ID: "album-drive", Name: "Album drive", Path: root,
		Type: domain.DriveTypeExternal, Status: domain.DriveStatusMounted, HasLibrary: true,
	}})
	library.HandleEvent(state.Event{Kind: state.EventActiveDriveChange})

	store.Navigate(state.ScreenAlbums)
	tt := mui.NewTester(shell.View, 1200, 800)
	tt.Frame()
	if !tt.HasText("Trip") {
		t.Fatalf("album gallery did not render: %q", tt.Texts())
	}
	if err := tt.Click("Trip"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("album.jpg") {
		t.Fatalf("album detail did not render the member card: %q", tt.Texts())
	}
	if err := tt.Click("album.jpg"); err != nil {
		t.Fatal(err)
	}
	if store.State.Screen() != state.ScreenViewer {
		t.Fatalf("screen after member click = %q, want viewer", store.State.Screen())
	}
	if !shell.Viewer().IsOpen() {
		t.Fatal("viewer is not open after album member click")
	}

	// Seed a second item in the album and verify Next/Previous navigation works.
	db2, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	repo2 := storage.NewMediaRepo(db2)
	second := domain.MediaItem{
		FileHash:            "album-second-hash",
		CurrentRelativePath: "media/albums/Trip/second.mp4",
		FileSize:            4096,
		MimeType:            "video/mp4",
	}
	if _, err := repo2.InsertBatch(context.Background(), []domain.MediaItem{second}); err != nil {
		t.Fatal(err)
	}
	inserted, err := repo2.GetByHash(context.Background(), second.FileHash)
	if err != nil || inserted == nil {
		t.Fatalf("get second item: %v", err)
	}
	if _, err := repo2.MoveToAlbum(context.Background(), album.ID, []int64{inserted.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db2.Close(); err != nil {
		t.Fatal(err)
	}

	store.Navigate(state.ScreenAlbums)
	shell.Albums().OpenAlbum(domain.AlbumSummary{ID: album.ID, Name: "Trip"})
	tt.Frame()
	if !tt.HasText("second.mp4") {
		t.Fatalf("album detail did not render second.mp4: %q", tt.Texts())
	}
	if err := tt.Click("album.jpg"); err != nil {
		t.Fatal(err)
	}
	firstID := shell.Viewer().CurrentItem().ID
	shell.Viewer().NextItem()
	if got := shell.Viewer().CurrentItem().ID; got == firstID {
		t.Fatalf("NextItem did not advance from item %d", firstID)
	}
	shell.Viewer().PrevItem()
	if got := shell.Viewer().CurrentItem().ID; got != firstID {
		t.Fatalf("PrevItem returned %d, want %d", got, firstID)
	}
}
