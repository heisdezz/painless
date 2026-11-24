package dashboard

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

type mockDriveSource struct {
	drives []domain.DriveInfo
}

func (m *mockDriveSource) ListDrives(ctx context.Context) ([]domain.DriveInfo, error) {
	return m.drives, nil
}

type mockDashboardSource struct {
	stats   Stats
	media   []domain.MediaItem
	albums  []domain.AlbumSummary
	statsMu sync.Mutex
}

func (m *mockDashboardSource) GetDashboardStats(ctx context.Context) (Stats, error) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	return m.stats, nil
}

func (m *mockDashboardSource) GetRecentMedia(ctx context.Context, limit int) ([]domain.MediaItem, error) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	if limit > len(m.media) {
		limit = len(m.media)
	}
	return m.media[:limit], nil
}

func (m *mockDashboardSource) GetRecentAlbums(ctx context.Context, limit int) ([]domain.AlbumSummary, error) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	if limit > len(m.albums) {
		limit = len(m.albums)
	}
	return m.albums[:limit], nil
}

type mockThumbnailer struct{}

func (m *mockThumbnailer) Get(ctx context.Context, item domain.MediaItem) ([]byte, error) {
	return nil, nil
}

type recordedActions struct {
	mu           sync.Mutex
	selected     string
	navMedia     bool
	navAlbums    bool
	navDiscover  bool
	openedViewer *domain.MediaItem
	openedAlbum  *domain.AlbumSummary
	disconnected bool
	refreshed    bool
}

func (r *recordedActions) SelectDrive(driveID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.selected = driveID
}

func (r *recordedActions) NavigateToMedia() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.navMedia = true
}

func (r *recordedActions) NavigateToAlbums() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.navAlbums = true
}

func (r *recordedActions) NavigateToDiscover() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.navDiscover = true
}

func (r *recordedActions) OpenViewer(item domain.MediaItem, allItems []domain.MediaItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.openedViewer = &item
}

func (r *recordedActions) OpenAlbum(album domain.AlbumSummary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.openedAlbum = &album
}

func (r *recordedActions) DisconnectDrive() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disconnected = true
}

func (r *recordedActions) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshed = true
}

func testDrive() domain.DriveInfo {
	return domain.DriveInfo{
		ID:         "drive-1",
		Name:       "TOSHIBA MQ01ABD100",
		Path:       "/run/media/destiny/Ventoy",
		Type:       "external",
		Status:     "mounted",
		TotalBytes: 1000204886016, // ~932 GB
	}
}

func testStats() Stats {
	return Stats{
		TotalMedia:    15278,
		TotalAlbums:   2,
		StorageUsed:   0,
		StorageTotal:  1000204886016,
		UsedPercent:   0,
		AlbumLocation: "/run/media/destiny/Ventoy/media/albums",
	}
}

func testMedia() []domain.MediaItem {
	return []domain.MediaItem{
		{
			ID:                  1,
			FileHash:            "hash1",
			CurrentRelativePath: "photos/vacation.jpg",
			FileSize:            2048000,
			MimeType:            "image/jpeg",
		},
		{
			ID:                  2,
			FileHash:            "hash2",
			CurrentRelativePath: "videos/sunset.mp4",
			FileSize:            10480000,
			MimeType:            "video/mp4",
		},
	}
}

func testAlbums() []domain.AlbumSummary {
	return []domain.AlbumSummary{
		{
			ID:         1,
			Name:       "Summer 2026",
			MediaCount: 15,
		},
		{
			ID:         2,
			Name:       "unknown",
			MediaCount: 100,
		},
	}
}

func TestDashboard_DrivePickerEmptyState(t *testing.T) {
	screen := New(Options{
		DriveSource: &mockDriveSource{drives: nil},
	})

	time.Sleep(50 * time.Millisecond)

	tt := mui.NewTester(screen.View, 1200, 800)
	if !tt.HasText("SELECT A CONNECTED DRIVE") {
		t.Fatalf("expected title, got: %q", tt.Texts())
	}
	if !tt.HasText("No Drives Connected") {
		t.Fatalf("expected empty state, got: %q", tt.Texts())
	}
}

func TestDashboard_DrivePickerWithDrives(t *testing.T) {
	drive := testDrive()
	actions := &recordedActions{}
	screen := New(Options{
		DriveSource: &mockDriveSource{drives: []domain.DriveInfo{drive}},
		Actions:     actions,
	})

	time.Sleep(50 * time.Millisecond)

	tt := mui.NewTester(screen.View, 1200, 800)
	if !tt.HasText("TOSHIBA MQ01ABD100") {
		t.Fatalf("expected drive name, got: %q", tt.Texts())
	}
	if !tt.HasText("Mounted") {
		t.Fatalf("expected mounted tag, got: %q", tt.Texts())
	}

	if err := tt.Click("TOSHIBA MQ01ABD100"); err != nil {
		t.Fatalf("failed to click drive card: %v", err)
	}

	actions.mu.Lock()
	selected := actions.selected
	actions.mu.Unlock()

	if selected != "drive-1" {
		t.Fatalf("expected drive-1 selected, got: %s", selected)
	}
}

func TestDashboard_DrivePickerBlocksSystemBootDrive(t *testing.T) {
	efi := domain.DriveInfo{
		ID:         "/boot/efi",
		Name:       "efi",
		Path:       "/boot/efi",
		Type:       domain.DriveTypeInternal,
		Status:     domain.DriveStatusMounted,
		TotalBytes: 512 << 20,
	}
	actions := &recordedActions{}
	screen := New(Options{
		DriveSource: &mockDriveSource{drives: []domain.DriveInfo{efi}},
		Actions:     actions,
	})

	time.Sleep(50 * time.Millisecond)

	tt := mui.NewTester(screen.View, 1200, 800)
	if !tt.HasText("System Boot") {
		t.Fatalf("expected System Boot tag, got: %q", tt.Texts())
	}
	if err := tt.Click("efi"); err != nil {
		t.Fatalf("failed to click drive card: %v", err)
	}

	actions.mu.Lock()
	selected := actions.selected
	actions.mu.Unlock()
	if selected != "" {
		t.Fatalf("expected system boot drive not to be selected, got: %s", selected)
	}
}

func TestDashboard_MountedView(t *testing.T) {
	drive := testDrive()
	src := &mockDashboardSource{
		stats:  testStats(),
		media:  testMedia(),
		albums: testAlbums(),
	}
	actions := &recordedActions{}
	thumbs := &mockThumbnailer{}

	screen := New(Options{
		ActiveDrive: &drive,
		Source:      src,
		Thumbs:      thumbs,
		Actions:     actions,
	})

	time.Sleep(50 * time.Millisecond)

	tt := mui.NewTester(screen.View, 1200, 800)

	// 1. Header checks
	if !tt.HasText("TOSHIBA MQ01ABD100") {
		t.Fatalf("expected drive name in header, got: %q", tt.Texts())
	}
	if !tt.HasText("/run/media/destiny/Ventoy/media/albums") {
		t.Fatalf("expected album folder path, got: %q", tt.Texts())
	}
	if !tt.HasText("Scan Volume") {
		t.Fatalf("expected Scan Volume button, got: %q", tt.Texts())
	}

	// 2. Stats cards checks
	if !tt.HasText("TOTAL MEDIA ITEMS") || !tt.HasText("15278") {
		t.Fatalf("expected media count 15278, got: %q", tt.Texts())
	}
	if !tt.HasText("TOTAL ALBUMS") || !tt.HasText("2") {
		t.Fatalf("expected album count 2, got: %q", tt.Texts())
	}
	if !tt.HasText("STORAGE USED") {
		t.Fatalf("expected storage used stat, got: %q", tt.Texts())
	}

	// 3. Recent media section checks
	if !tt.HasText("RECENTLY ADDED MEDIA") || !tt.HasText("View All Media →") {
		t.Fatalf("expected recent media header, got: %q", tt.Texts())
	}
	if !tt.HasText("vacation.jpg") {
		t.Fatalf("expected vacation.jpg preview, got: %q", tt.Texts())
	}

	// 4. Recent albums section checks
	if !tt.HasText("RECENT ALBUMS") || !tt.HasText("View All Albums →") {
		t.Fatalf("expected recent albums header, got: %q", tt.Texts())
	}
	if !tt.HasText("Summer 2026") || !tt.HasText("Unsorted Media") {
		t.Fatalf("expected album cards, got: %q", tt.Texts())
	}
}

func TestDashboard_ActionsNavigation(t *testing.T) {
	drive := testDrive()
	src := &mockDashboardSource{
		stats:  testStats(),
		media:  testMedia(),
		albums: testAlbums(),
	}
	actions := &recordedActions{}

	screen := New(Options{
		ActiveDrive: &drive,
		Source:      src,
		Actions:     actions,
	})

	time.Sleep(50 * time.Millisecond)

	tt := mui.NewTester(screen.View, 1200, 800)

	// Click Scan Volume
	if err := tt.Click("Scan Volume"); err != nil {
		t.Fatalf("failed to click Scan Volume: %v", err)
	}
	actions.mu.Lock()
	if !actions.navDiscover {
		t.Fatalf("expected navDiscover to be true")
	}
	actions.mu.Unlock()

	// Click View All Media
	if err := tt.Click("View All Media →"); err != nil {
		t.Fatalf("failed to click View All Media: %v", err)
	}
	actions.mu.Lock()
	if !actions.navMedia {
		t.Fatalf("expected navMedia to be true")
	}
	actions.mu.Unlock()

	// Click View All Albums
	if err := tt.Click("View All Albums →"); err != nil {
		t.Fatalf("failed to click View All Albums: %v", err)
	}
	actions.mu.Lock()
	if !actions.navAlbums {
		t.Fatalf("expected navAlbums to be true")
	}
	actions.mu.Unlock()

	// Click album card
	if err := tt.Click("Summer 2026"); err != nil {
		t.Fatalf("failed to click Summer 2026: %v", err)
	}
	actions.mu.Lock()
	if actions.openedAlbum == nil || actions.openedAlbum.Name != "Summer 2026" {
		t.Fatalf("expected Summer 2026 album opened")
	}
	actions.mu.Unlock()

	// Click media card
	if err := tt.Click("vacation.jpg"); err != nil {
		t.Fatalf("failed to click vacation.jpg: %v", err)
	}
	actions.mu.Lock()
	if actions.openedViewer == nil || actions.openedViewer.ID != 1 {
		t.Fatalf("expected vacation.jpg opened in viewer")
	}
	actions.mu.Unlock()
}

func TestDashboard_ConcurrentAccess(t *testing.T) {
	drive := testDrive()
	src := &mockDashboardSource{
		stats:  testStats(),
		media:  testMedia(),
		albums: testAlbums(),
	}
	actions := &recordedActions{}

	screen := New(Options{
		ActiveDrive: &drive,
		Source:      src,
		Actions:     actions,
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				screen.Reload()
			} else {
				d := testDrive()
				d.Name = "Updated Drive"
				screen.SetActiveDrive(&d)
			}
		}(i)
	}

	tt := mui.NewTester(screen.View, 1200, 800)
	_ = tt.Texts()

	wg.Wait()
}

// pngThumbnailer serves a decodable tiny PNG and records calls.
type pngThumbnailer struct{ calls atomic.Int64 }

func (p *pngThumbnailer) Get(context.Context, domain.MediaItem) ([]byte, error) {
	p.calls.Add(1)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func TestDashboardThumbnailsDisabled(t *testing.T) {
	enabled := false
	drive := testDrive()
	src := &mockDashboardSource{
		stats:  testStats(),
		media:  testMedia(),
		albums: testAlbums(),
	}
	thumbs := &pngThumbnailer{}

	screen := New(Options{
		ActiveDrive:   &drive,
		Source:        src,
		Thumbs:        thumbs,
		ThumbsEnabled: func() bool { return enabled },
		Actions:       &recordedActions{},
	})

	tt := mui.NewTester(screen.View, 1200, 800)
	time.Sleep(80 * time.Millisecond)
	for i := 0; i < 5; i++ {
		tt.Frame()
	}
	if calls := thumbs.calls.Load(); calls != 0 {
		t.Fatalf("disabled setting scheduled %d fetches, want 0", calls)
	}
	if len(screen.bitmaps) != 0 {
		t.Fatal("disabled setting decoded bitmaps")
	}

	// Enabling the setting resumes fetching and decoding.
	enabled = true
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(screen.bitmaps) == 0 {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if thumbs.calls.Load() == 0 {
		t.Fatal("enabled setting never fetched")
	}
	if len(screen.bitmaps) == 0 {
		t.Fatal("enabled setting decoded no bitmaps")
	}
}
