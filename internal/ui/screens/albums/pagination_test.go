package albums

import (
	"fmt"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

func makeMemberItems(albumID int64, n int) []domain.MediaItem {
	items := make([]domain.MediaItem, 0, n)
	for i := 1; i <= n; i++ {
		id := albumID
		items = append(items, domain.MediaItem{
			ID:                  int64(i),
			AlbumID:             &id,
			CurrentRelativePath: fmt.Sprintf("photos/m%03d.jpg", i),
			MimeType:            "image/jpeg",
			FileSize:            1024,
		})
	}
	return items
}

func waitForMembers(t *testing.T, tt *mui.Tester, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		// Member pages load off-thread; frames drain the worker's result.
		tt.Frame()
		if done() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met within 2s")
}

func TestAlbumMemberPagination(t *testing.T) {
	repo := newMockAlbumRepo()
	mediaRepo := &mockMediaRepo{items: makeMemberItems(1, 130)}
	screen := New(Options{Albums: repo, Media: mediaRepo})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	waitForMembers(t, tt, func() bool { return len(screen.detailItems) == memberPageSize })
	if !tt.HasText("m001.jpg") || tt.HasText("m061.jpg") {
		t.Fatalf("page 1 should show members 1-60: %q", tt.Texts())
	}
	if mediaRepo.last.Limit != memberPageSize || mediaRepo.last.Offset != 0 {
		t.Fatalf("first page params = limit %d offset %d", mediaRepo.last.Limit, mediaRepo.last.Offset)
	}

	if err := tt.Click("2"); err != nil {
		t.Fatal(err)
	}
	waitForMembers(t, tt, func() bool { return mediaRepo.last.Offset == memberPageSize })
	if !tt.HasText("m061.jpg") || tt.HasText("m121.jpg") {
		t.Fatalf("page 2 should show members 61-120: %q", tt.Texts())
	}
	if !tt.HasText("m120.jpg") || tt.HasText("m130.jpg") {
		t.Fatalf("page 2 should not show page-3 members: %q", tt.Texts())
	}
}

func TestAlbumMemberPaginationResetsOnAlbumOpen(t *testing.T) {
	repo := newMockAlbumRepo()
	mediaRepo := &mockMediaRepo{items: makeMemberItems(1, 130)}
	screen := New(Options{Albums: repo, Media: mediaRepo})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	tt.Frame()
	waitForMembers(t, tt, func() bool { return len(screen.detailItems) == memberPageSize })
	if err := tt.Click("2"); err != nil {
		t.Fatal(err)
	}
	waitForMembers(t, tt, func() bool { return mediaRepo.last.Offset == memberPageSize })

	// Reopening the album (after closing detail) returns to the first page.
	screen.CloseDetail()
	screen.OpenAlbum(repo.summaries[0])
	waitForMembers(t, tt, func() bool { return mediaRepo.last.Offset == 0 && len(screen.detailItems) == memberPageSize })
	if screen.memberPage != 1 {
		t.Fatalf("member page after reopen = %d, want 1", screen.memberPage)
	}
}
