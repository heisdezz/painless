package albums

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	items []domain.MediaItem
}

func (m *mockMediaRepo) List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error) {
	var filtered []domain.MediaItem
	for _, it := range m.items {
		if params.AlbumID != nil && it.AlbumID != nil && *it.AlbumID == *params.AlbumID {
			filtered = append(filtered, it)
		}
	}
	return &domain.MediaListPage{
		Items: filtered,
		Total: len(filtered),
	}, nil
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
	if !tt.HasText("Summer 2026") || !tt.HasText("Winter 2025") {
		t.Fatalf("expected gallery view to be restored")
	}
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
