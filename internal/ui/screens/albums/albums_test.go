package albums

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
)

type mockAlbumRepo struct {
	mu        sync.Mutex
	albums    map[int64]domain.Album
	summaries []domain.AlbumSummary
	nextID    int64
	createErr error
	updateErr error
	deleteErr error
	mergeErr  error
}

func newMockAlbumRepo() *mockAlbumRepo {
	desc1 := "Fun in the sun"
	desc2 := "Winter adventures"
	return &mockAlbumRepo{
		nextID: 3,
		albums: map[int64]domain.Album{
			1: {ID: 1, Name: "Summer 2026", Description: &desc1, RelativePath: "albums/summer-2026", CreatedAt: time.Now()},
			2: {ID: 2, Name: "Winter 2025", Description: &desc2, RelativePath: "albums/winter-2025", CreatedAt: time.Now()},
		},
		summaries: []domain.AlbumSummary{
			{
				Album: domain.Album{
					ID: 1, Name: "Summer 2026", Description: &desc1, RelativePath: "albums/summer-2026", CreatedAt: time.Now(),
				},
				MediaCount: 15,
				TotalBytes: 1024 * 1024 * 50,
			},
			{
				Album: domain.Album{
					ID: 2, Name: "Winter 2025", Description: &desc2, RelativePath: "albums/winter-2025", CreatedAt: time.Now(),
				},
				MediaCount: 8,
				TotalBytes: 1024 * 1024 * 20,
			},
		},
	}
}

func (m *mockAlbumRepo) ListSummaries(ctx context.Context) ([]domain.AlbumSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]domain.AlbumSummary, len(m.summaries))
	copy(res, m.summaries)
	return res, nil
}

func (m *mockAlbumRepo) GetByID(ctx context.Context, id int64) (*domain.Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.albums[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &a, nil
}

func (m *mockAlbumRepo) Create(ctx context.Context, name, description string) (*domain.Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return nil, m.createErr
	}
	id := m.nextID
	m.nextID++
	var desc *string
	if description != "" {
		desc = &description
	}
	alb := domain.Album{
		ID:           id,
		Name:         name,
		Description:  desc,
		RelativePath: fmt.Sprintf("albums/%s", name),
		CreatedAt:    time.Now(),
	}
	m.albums[id] = alb
	m.summaries = append(m.summaries, domain.AlbumSummary{
		Album:      alb,
		MediaCount: 0,
		TotalBytes: 0,
	})
	return &alb, nil
}

func (m *mockAlbumRepo) Update(ctx context.Context, album domain.Album) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.albums[album.ID]; !ok {
		return errors.New("not found")
	}
	m.albums[album.ID] = album
	for i, s := range m.summaries {
		if s.ID == album.ID {
			m.summaries[i].Album = album
			break
		}
	}
	return nil
}

func (m *mockAlbumRepo) Delete(ctx context.Context, id int64, softDelete bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.albums, id)
	newSums := make([]domain.AlbumSummary, 0, len(m.summaries))
	for _, s := range m.summaries {
		if s.ID != id {
			newSums = append(newSums, s)
		}
	}
	m.summaries = newSums
	return nil
}

func (m *mockAlbumRepo) Merge(ctx context.Context, sourceID, targetID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mergeErr != nil {
		return m.mergeErr
	}
	delete(m.albums, sourceID)
	var count int
	var bytes int64
	for _, s := range m.summaries {
		if s.ID == sourceID {
			count = s.MediaCount
			bytes = s.TotalBytes
		}
	}
	newSums := make([]domain.AlbumSummary, 0)
	for _, s := range m.summaries {
		if s.ID == targetID {
			s.MediaCount += count
			s.TotalBytes += bytes
			newSums = append(newSums, s)
		} else if s.ID != sourceID {
			newSums = append(newSums, s)
		}
	}
	m.summaries = newSums
	return nil
}

type mockMediaRepo struct {
	mu    sync.Mutex
	items []domain.MediaItem
	last  domain.MediaListParams
}

func (m *mockMediaRepo) List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error) {
	m.mu.Lock()
	m.last = params
	m.mu.Unlock()
	var filtered []domain.MediaItem
	for _, it := range m.items {
		if params.AlbumID != nil && (it.AlbumID == nil || *it.AlbumID != *params.AlbumID) {
			continue
		}
		if params.Filter == domain.MediaFilterImages && it.Type() != domain.MediaTypeImage {
			continue
		}
		if params.Filter == domain.MediaFilterVideos && it.Type() != domain.MediaTypeVideo {
			continue
		}
		if params.Search != "" && !strings.Contains(strings.ToLower(it.CurrentRelativePath), strings.ToLower(params.Search)) {
			continue
		}
		filtered = append(filtered, it)
	}
	total := len(filtered)
	if params.Offset > len(filtered) {
		filtered = nil
	} else {
		filtered = filtered[params.Offset:]
	}
	if params.Limit > 0 && params.Limit < len(filtered) {
		filtered = filtered[:params.Limit]
	}
	return &domain.MediaListPage{
		Items:  filtered,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// waitUntil pumps frames until cond holds. Album summaries and member pages
// load off-thread now, so assertions must wait for the worker's result to
// drain into the view.
func waitUntil(t *testing.T, tt *mui.Tester, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition never met")
}

// waitSummaries waits for the async album summaries fetch to land.
func waitSummaries(t *testing.T, tt *mui.Tester, s *Screen) {
	t.Helper()
	waitUntil(t, tt, func() bool { return !s.albumsLoading })
}

// waitMembers waits for the async member fetch of the open album to land and
// render its cards.
func waitMembers(t *testing.T, tt *mui.Tester, s *Screen, name string) {
	t.Helper()
	waitUntil(t, tt, func() bool { return !s.detailLoading && tt.HasText(name) })
}

func TestAlbumMemberCardOpensViewer(t *testing.T) {
	repo := newMockAlbumRepo()
	albumID := int64(1)
	mediaRepo := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 101, AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
	}}

	var opened domain.MediaItem
	var openedAll []domain.MediaItem
	screen := New(Options{
		Albums: repo,
		Media:  mediaRepo,
		OnOpenItem: func(item domain.MediaItem, all []domain.MediaItem) {
			opened = item
			openedAll = all
		},
	})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	waitMembers(t, tt, screen, "beach.jpg")
	if err := tt.Click("beach.jpg"); err != nil {
		t.Fatal(err)
	}
	if opened.ID != 101 {
		t.Fatalf("member click opened item %+v, want id 101", opened)
	}
	if len(openedAll) != 1 || openedAll[0].ID != 101 {
		t.Fatalf("member click opened context %+v, want [101]", openedAll)
	}
}

func TestAlbumMemberCardNilCallback(t *testing.T) {
	repo := newMockAlbumRepo()
	albumID := int64(1)
	mediaRepo := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 101, AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
	}}

	screen := New(Options{Albums: repo, Media: mediaRepo})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	waitMembers(t, tt, screen, "beach.jpg")
	if err := tt.Click("beach.jpg"); err != nil {
		t.Fatal(err)
	}
}

func TestAlbumMemberThumbnailsLoad(t *testing.T) {
	repo := newMockAlbumRepo()
	albumID := int64(1)
	mediaRepo := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 101, FileHash: "hash-101", AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
	}}
	thumbs := &stubThumbs{}

	screen := New(Options{Albums: repo, Media: mediaRepo, Thumbs: thumbs})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(screen.bitmaps) == 0 {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if thumbs.calls.Load() == 0 {
		t.Fatal("thumbnail renderer was never called")
	}
	if len(screen.bitmaps) == 0 {
		t.Fatal("no member thumbnail was decoded")
	}
}

// stubThumbs renders a tiny PNG, mirroring the catalog's fake thumbnailer.
type stubThumbs struct{ calls atomic.Int64 }

func (f *stubThumbs) Get(context.Context, domain.MediaItem) ([]byte, error) {
	f.calls.Add(1)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 200, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func TestAlbumScreen_EmptyState(t *testing.T) {
	repo := &mockAlbumRepo{
		albums:    make(map[int64]domain.Album),
		summaries: nil,
	}
	screen := New(Options{
		Albums: repo,
	})

	tt := mui.NewTester(screen.View, 1000, 700)
	waitSummaries(t, tt, screen)
	if !tt.HasText("No albums yet") {
		t.Fatalf("expected empty state, got texts: %q", tt.Texts())
	}
	if !tt.HasText("Create Your First Album") {
		t.Fatalf("expected create button in empty state, got texts: %q", tt.Texts())
	}
}

func TestAlbumScreen_GalleryView(t *testing.T) {
	repo := newMockAlbumRepo()
	redraws := 0
	screen := New(Options{
		Albums: repo,
		RefreshWindow: func() {
			redraws++
		},
	})

	tt := mui.NewTester(screen.View, 1000, 700)
	waitSummaries(t, tt, screen)
	if !tt.HasText("Summer 2026") || !tt.HasText("Winter 2025") {
		t.Fatalf("expected album cards rendered, got: %q", tt.Texts())
	}
	if !tt.HasText("15") || !tt.HasText("8") {
		t.Fatalf("expected media count badges, got: %q", tt.Texts())
	}
}

func TestAlbumScreen_Filter(t *testing.T) {
	repo := newMockAlbumRepo()
	screen := New(Options{Albums: repo})

	tt := mui.NewTester(screen.View, 1000, 700)
	waitSummaries(t, tt, screen)

	screen.SetSearchQuery("Summer")
	filtered := screen.getFilteredSummaries()
	if len(filtered) != 1 || filtered[0].Name != "Summer 2026" {
		t.Fatalf("expected 1 result for 'Summer', got %d", len(filtered))
	}

	screen.SetSearchQuery("NonExistent")
	filtered = screen.getFilteredSummaries()
	if len(filtered) != 0 {
		t.Fatalf("expected 0 results, got %d", len(filtered))
	}

	screen.SetSearchQuery("adventures") // in description
	filtered = screen.getFilteredSummaries()
	if len(filtered) != 1 || filtered[0].Name != "Winter 2025" {
		t.Fatalf("expected 1 result matching description, got %d", len(filtered))
	}
}

func TestAlbumScreen_DetailNavigation(t *testing.T) {
	repo := newMockAlbumRepo()
	albumID := int64(1)
	mediaRepo := &mockMediaRepo{
		items: []domain.MediaItem{
			{ID: 101, AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
			{ID: 102, AlbumID: &albumID, CurrentRelativePath: "photos/sunset.png", FileSize: 4096, MimeType: "image/png"},
		},
	}

	screen := New(Options{
		Albums: repo,
		Media:  mediaRepo,
	})

	// Open album
	screen.OpenAlbum(repo.summaries[0])
	if screen.SelectedAlbum() == nil || screen.SelectedAlbum().ID != 1 {
		t.Fatalf("expected selected album 1")
	}

	tt := mui.NewTester(screen.View, 1000, 700)
	waitMembers(t, tt, screen, "beach.jpg")
	if !tt.HasText("Back to Albums") {
		t.Fatalf("expected back button, got: %q", tt.Texts())
	}
	if !tt.HasText("beach.jpg") || !tt.HasText("sunset.png") {
		t.Fatalf("expected member media items, got: %q", tt.Texts())
	}

	// Close detail
	screen.CloseDetail()
	if screen.SelectedAlbum() != nil {
		t.Fatalf("expected selected album to be nil after CloseDetail")
	}

	tt = mui.NewTester(screen.View, 1000, 700)
	waitSummaries(t, tt, screen)
	waitUntil(t, tt, func() bool { return tt.HasText("Summer 2026") && tt.HasText("Winter 2025") })
}

func TestAlbumScreen_CreateModal(t *testing.T) {
	repo := newMockAlbumRepo()
	screen := New(Options{Albums: repo})

	// Open modal
	screen.createOpen = true
	screen.createName = "Invalid/Album?Name"
	screen.createDesc = "Description"

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("Create New Album") {
		t.Fatalf("expected create modal title, got: %q", tt.Texts())
	}

	// Test validation logic directly
	if domain.ValidateAlbumName(screen.createName) {
		t.Fatalf("expected %q to be invalid", screen.createName)
	}

	screen.createName = "Spring 2026"
	if !domain.ValidateAlbumName(screen.createName) {
		t.Fatalf("expected %q to be valid", screen.createName)
	}
}

func TestAlbumScreen_EditModal(t *testing.T) {
	repo := newMockAlbumRepo()
	screen := New(Options{Albums: repo})

	screen.openEditModal(repo.summaries[0])
	if !screen.editOpen || screen.editTarget == nil {
		t.Fatalf("expected edit modal to open")
	}
	if screen.editName != "Summer 2026" {
		t.Fatalf("expected edit name prefilled, got %s", screen.editName)
	}

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("Edit Album Details") {
		t.Fatalf("expected edit modal title, got: %q", tt.Texts())
	}
}

func TestAlbumScreen_DeleteModal(t *testing.T) {
	repo := newMockAlbumRepo()
	screen := New(Options{Albums: repo})

	screen.openDeleteModal(repo.summaries[0])
	if !screen.deleteOpen || screen.deleteTarget == nil {
		t.Fatalf("expected delete modal to open")
	}
	if screen.deleteMode != "soft" {
		t.Fatalf("expected default delete mode soft, got %s", screen.deleteMode)
	}

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("Delete Album: Summer 2026") {
		t.Fatalf("expected delete modal title, got: %q", tt.Texts())
	}
	if !tt.HasText("Keep media files (Soft delete)") || !tt.HasText("Delete media permanently") {
		t.Fatalf("expected radio options in delete modal, got: %q", tt.Texts())
	}
}

func TestAlbumScreen_MergeModal(t *testing.T) {
	repo := newMockAlbumRepo()
	screen := New(Options{Albums: repo})

	screen.openMergeModal(repo.summaries[0])
	if !screen.mergeOpen || screen.mergeSource == nil {
		t.Fatalf("expected merge modal to open")
	}

	tt := mui.NewTester(screen.View, 1000, 700)
	if !tt.HasText("Merge Album \"Summer 2026\"") {
		t.Fatalf("expected merge modal title, got: %q", tt.Texts())
	}
	if !tt.HasText("Destination Target Album:") {
		t.Fatalf("expected destination album selector, got: %q", tt.Texts())
	}
}

func TestAlbumScreen_ConcurrentAccess(t *testing.T) {
	repo := newMockAlbumRepo()
	screen := New(Options{Albums: repo})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%3 == 0 {
				screen.Reload()
			} else if idx%3 == 1 {
				screen.OpenAlbum(repo.summaries[0])
				_ = screen.SelectedAlbum()
				screen.CloseDetail()
			} else {
				screen.SetSearchQuery(fmt.Sprintf("Query %d", idx))
				_ = screen.getFilteredSummaries()
			}
		}(i)
	}
	wg.Wait()
}

func TestAlbumMemberThumbnailsDisabled(t *testing.T) {
	enabled := false
	repo := newMockAlbumRepo()
	albumID := int64(1)
	mediaRepo := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 101, FileHash: "hash-101", AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
	}}
	thumbs := &stubThumbs{}

	screen := New(Options{
		Albums: repo, Media: mediaRepo, Thumbs: thumbs,
		ThumbsEnabled: func() bool { return enabled },
	})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	for i := 0; i < 10; i++ {
		tt.Frame()
	}
	time.Sleep(30 * time.Millisecond)
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

func TestAlbumDetailFilterSortAndSearch(t *testing.T) {
	repo := newMockAlbumRepo()
	albumID := int64(1)
	mediaRepo := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 101, FileHash: "h1", AlbumID: &albumID, CurrentRelativePath: "photos/beach.jpg", FileSize: 2048, MimeType: "image/jpeg"},
		{ID: 102, FileHash: "h2", AlbumID: &albumID, CurrentRelativePath: "videos/waves.mp4", FileSize: 8192, MimeType: "video/mp4"},
	}}

	screen := New(Options{Albums: repo, Media: mediaRepo})
	screen.OpenAlbum(repo.summaries[0])

	tt := mui.NewTester(screen.View, 1000, 700)
	waitMembers(t, tt, screen, "beach.jpg")
	if !tt.HasText("waves.mp4") {
		t.Fatalf("expected both items initially, got: %q", tt.Texts())
	}

	// Filter to Photos
	if err := tt.Click("Photos"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, tt, func() bool {
		mediaRepo.mu.Lock()
		last := mediaRepo.last
		mediaRepo.mu.Unlock()
		return last.Filter == domain.MediaFilterImages && !tt.HasText("waves.mp4")
	})

	// Filter back to All and change sort order
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if err := tt.Click("All"); err != nil {
		t.Fatal(err)
	}
	waitMembers(t, tt, screen, "waves.mp4")

	if err := tt.Click("Newest first"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Name A-Z"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, tt, func() bool {
		mediaRepo.mu.Lock()
		last := mediaRepo.last
		mediaRepo.mu.Unlock()
		return last.SortBy == domain.SortByName && last.SortOrder == domain.SortOrderAsc
	})

	// Debounced search
	screen.detailQuery = "waves"
	tt.Frame()
	time.Sleep(detailDebounceDelay + 30*time.Millisecond)
	waitUntil(t, tt, func() bool {
		mediaRepo.mu.Lock()
		last := mediaRepo.last
		mediaRepo.mu.Unlock()
		return last.Search == "waves" && !tt.HasText("beach.jpg") && tt.HasText("waves.mp4")
	})
}

func TestAlbumDetailHeaderCompactAndActions(t *testing.T) {
	repo := newMockAlbumRepo()
	albumID := int64(3)
	noDesc := domain.AlbumSummary{
		Album:      domain.Album{ID: albumID, Name: "_sami.c", RelativePath: "albums/_sami.c"},
		MediaCount: 1935,
		TotalBytes: 1024 * 1024 * 1024 * 10,
	}
	mediaRepo := &mockMediaRepo{items: []domain.MediaItem{
		{ID: 301, AlbumID: &albumID, CurrentRelativePath: "videos/clip.mp4", FileSize: 2048, MimeType: "video/mp4"},
	}}

	screen := New(Options{Albums: repo, Media: mediaRepo})
	screen.OpenAlbum(noDesc)

	tt := mui.NewTester(screen.View, 700, 600)
	waitMembers(t, tt, screen, "clip.mp4")

	if tt.HasText("No description provided.") {
		t.Fatalf("empty description should not render placeholder text: %q", tt.Texts())
	}
	cardRect, ok := tt.Find("card clip.mp4")
	if !ok {
		t.Fatal("first member card not found")
	}
	if cardRect.Y >= 120 {
		t.Fatalf("member card starts too low at Y=%.1f (compact header should sit under 120px)", cardRect.Y)
	}

	// Verify header action icons are reachable and open their modals.
	if err := tt.Click("Edit album info"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Edit Album Details") {
		t.Fatalf("expected edit modal after clicking header edit action, got: %q", tt.Texts())
	}
}

func TestAlbumGalleryFilterBarAndSort(t *testing.T) {
	repo := newMockAlbumRepo()
	now := time.Now()
	repo.summaries = append(repo.summaries, domain.AlbumSummary{
		Album:      domain.Album{ID: 3, Name: "Empty Archive", RelativePath: "albums/empty", CreatedAt: now.Add(time.Hour)},
		MediaCount: 0,
		TotalBytes: 0,
	})
	screen := New(Options{Albums: repo})

	tt := mui.NewTester(screen.View, 700, 600)
	waitSummaries(t, tt, screen)

	if !tt.HasText("3 albums") {
		t.Fatalf("expected '3 albums' count tag, got: %q", tt.Texts())
	}
	barRect, ok := tt.Find("New Album")
	cardRect, okCard := tt.Find("Empty Archive")
	if !ok || !okCard {
		t.Fatalf("expected toolbar button and album card bounds")
	}
	if cardRect.Y <= barRect.Y+barRect.H {
		t.Fatalf("album card (Y=%.1f) overlaps filter bar (bottom=%.1f)", cardRect.Y, barRect.Y+barRect.H)
	}

	// Filter to Empty albums only
	if err := tt.Click("Empty"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("1 albums") || !tt.HasText("Empty Archive") || tt.HasText("Summer 2026") {
		t.Fatalf("expected only Empty Archive, got: %q", tt.Texts())
	}

	// Filter to With media
	if err := tt.Click("With media"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("2 albums") || tt.HasText("Empty Archive") || !tt.HasText("Summer 2026") {
		t.Fatalf("expected only populated albums, got: %q", tt.Texts())
	}

	// Sort by Most items vs Name A-Z
	if err := tt.Click("Name A-Z"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Most items"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	sorted := screen.getFilteredSummaries()
	if len(sorted) != 2 || sorted[0].Name != "Summer 2026" {
		t.Fatalf("expected Summer 2026 first by Most items, got %+v", sorted)
	}

	// Search with no match shows filtered empty state rather than Create Your First Album
	screen.SetSearchQuery("NonExistentAlbum")
	tt.Frame()
	if !tt.HasText("No matching albums") || tt.HasText("Create Your First Album") {
		t.Fatalf("expected filtered empty state, got: %q", tt.Texts())
	}
}
