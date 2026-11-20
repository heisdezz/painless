package dashboard

import (
	"context"
	"sync"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

// mutableSource lets a test swap the media list between loads.
type mutableSource struct {
	mu    sync.Mutex
	media []domain.MediaItem
}

func (m *mutableSource) GetDashboardStats(ctx context.Context) (Stats, error) { return Stats{}, nil }

func (m *mutableSource) GetRecentMedia(ctx context.Context, limit int) ([]domain.MediaItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit > len(m.media) {
		limit = len(m.media)
	}
	return append([]domain.MediaItem(nil), m.media[:limit]...), nil
}

func (m *mutableSource) GetRecentAlbums(ctx context.Context, limit int) ([]domain.AlbumSummary, error) {
	return nil, nil
}

func (m *mutableSource) setMedia(media []domain.MediaItem) {
	m.mu.Lock()
	m.media = media
	m.mu.Unlock()
}

func waitForText(t *testing.T, tt *mui.Tester, text string) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		tt.Frame()
		if tt.HasText(text) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestRefreshDashboardShowsNewlyAddedMedia checks that clicking the
// "Refresh dashboard" button re-queries the source and shows newly added
// media.
func TestRefreshDashboardShowsNewlyAddedMedia(t *testing.T) {
	src := &mutableSource{media: []domain.MediaItem{{ID: 1, CurrentRelativePath: "old.jpg", FileSize: 100}}}
	screen := New(Options{
		ActiveDrive: &domain.DriveInfo{ID: "d1", Name: "Test", Path: "/media/d1", Status: domain.DriveStatusMounted},
		Source:      src,
		DriveSource: &mockDriveSource{},
		Actions:     &recordedActions{},
	})

	tt := mui.NewTester(screen.View, 1000, 800)
	if !waitForText(t, tt, "old.jpg") {
		t.Fatalf("initial texts = %q", tt.Texts())
	}

	// The user adds media to the drive; the source now returns it.
	src.setMedia([]domain.MediaItem{
		{ID: 1, CurrentRelativePath: "old.jpg", FileSize: 100},
		{ID: 2, CurrentRelativePath: "new.jpg", FileSize: 200},
	})

	if err := tt.Click("Refresh dashboard"); err != nil {
		t.Fatal(err)
	}

	if !waitForText(t, tt, "new.jpg") {
		t.Fatalf("new media missing after refresh; texts = %q", tt.Texts())
	}
}
