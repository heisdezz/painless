package albums

import (
	"context"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

// gatedAlbumRepo blocks ListSummaries until its release channel closes, so a
// test can hold the first gallery load in flight and inspect the skeleton.
type gatedAlbumRepo struct {
	*mockAlbumRepo
	release chan struct{}
}

func (g *gatedAlbumRepo) ListSummaries(ctx context.Context) ([]domain.AlbumSummary, error) {
	<-g.release
	return g.mockAlbumRepo.ListSummaries(ctx)
}

// gatedMediaRepo blocks List until its release channel closes.
type gatedMediaRepo struct {
	*mockMediaRepo
	release chan struct{}
}

func (g *gatedMediaRepo) List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error) {
	<-g.release
	return g.mockMediaRepo.List(ctx, params)
}

// TestAlbumGalleryDoesNotBlockFirstFrame pins the gallery's async contract:
// the first frame renders the loading skeleton — not the empty state — while
// summaries are still being fetched, and the real cards replace it once the
// worker lands.
func TestAlbumGalleryDoesNotBlockFirstFrame(t *testing.T) {
	repo := &gatedAlbumRepo{
		mockAlbumRepo: newMockAlbumRepo(),
		release:       make(chan struct{}),
	}
	screen := New(Options{Albums: repo})

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	if !tt.HasText("albums-loading") {
		t.Fatalf("expected loading skeleton on first frame, got: %q", tt.Texts())
	}
	if tt.HasText("No albums yet") {
		t.Fatal("empty state shown while summaries were still loading")
	}

	close(repo.release)
	waitSummaries(t, tt, screen)
	if !tt.HasText("Summer 2026") {
		t.Fatalf("expected album cards after load, got: %q", tt.Texts())
	}
	if tt.HasText("albums-loading") {
		t.Fatal("skeleton still shown after summaries landed")
	}
}

// TestAlbumDetailSkeletonWhileLoading pins the detail view's async contract:
// opening an album shows the member skeleton until the member page arrives.
func TestAlbumDetailSkeletonWhileLoading(t *testing.T) {
	albumID := int64(1)
	repo := newMockAlbumRepo()
	media := &gatedMediaRepo{
		mockMediaRepo: &mockMediaRepo{items: []domain.MediaItem{
			{ID: 101, AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
		}},
		release: make(chan struct{}),
	}
	screen := New(Options{Albums: repo, Media: media})

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	screen.OpenAlbum(repo.summaries[0])
	waitUntil(t, tt, func() bool { return tt.HasText("album-members-loading") })
	if tt.HasText("beach.jpg") {
		t.Fatal("member cards shown before the member fetch landed")
	}

	close(media.release)
	waitMembers(t, tt, screen, "beach.jpg")
	if tt.HasText("album-members-loading") {
		t.Fatal("skeleton still shown after members landed")
	}
}

// TestAlbumRevealStaggers checks the reveal choreography: a Reveal is armed
// once, when the gallery or member list first fills from empty, and reloading
// does not re-arm it (restrained motion — no restagger on refresh).
func TestAlbumRevealStaggers(t *testing.T) {
	repo := &gatedAlbumRepo{
		mockAlbumRepo: newMockAlbumRepo(),
		release:       make(chan struct{}),
	}
	screen := New(Options{Albums: repo})

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	if screen.galleryReveal != nil {
		t.Fatal("reveal armed before any summaries loaded")
	}

	close(repo.release)
	waitSummaries(t, tt, screen)
	if screen.galleryReveal == nil {
		t.Fatal("reveal not armed after gallery loaded")
	}
	first := screen.galleryReveal

	// A refresh while cards are shown must not restart the stagger.
	screen.Reload()
	waitUntil(t, tt, func() bool { return !screen.albumsLoading })
	if screen.galleryReveal != first {
		t.Fatal("reveal re-armed on refresh; only the empty-to-filled transition should stagger")
	}

	// Opening an album arms the member strip's own reveal.
	albumID := int64(1)
	media := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 101, AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
	}}
	screen.media = media
	screen.OpenAlbum(repo.summaries[0])
	waitMembers(t, tt, screen, "beach.jpg")
	if screen.detailReveal == nil {
		t.Fatal("reveal not armed after member list loaded")
	}
}
