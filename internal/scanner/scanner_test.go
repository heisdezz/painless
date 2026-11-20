package scanner

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"painless/internal/domain"
)

type mockInserter struct {
	mu       sync.Mutex
	items    []domain.MediaItem
	byHash   map[string]domain.MediaItem
	batchLog [][]domain.MediaItem
}

func newMockInserter() *mockInserter {
	return &mockInserter{
		byHash: make(map[string]domain.MediaItem),
	}
}

func (m *mockInserter) InsertBatch(ctx context.Context, items []domain.MediaItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]domain.MediaItem, len(items))
	copy(copied, items)
	m.batchLog = append(m.batchLog, copied)
	for _, item := range items {
		m.items = append(m.items, item)
		m.byHash[item.FileHash] = item
	}
	return nil
}

func (m *mockInserter) GetByHash(ctx context.Context, hash string) (*domain.MediaItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, found := m.byHash[hash]
	if !found {
		return nil, nil
	}
	return &item, nil
}

func TestIgnoreRules(t *testing.T) {
	ignoredDirs := []string{".git", ".svn", "node_modules", ".media_thumbnails", ".Trashes", ".hidden_dir"}
	for _, d := range ignoredDirs {
		if !ShouldIgnoreDir(d) {
			t.Errorf("expected %s to be ignored as directory", d)
		}
	}
	if ShouldIgnoreDir("albums") || ShouldIgnoreDir("Vacation 2026") {
		t.Errorf("regular folders should not be ignored")
	}

	ignoredFiles := []string{".DS_Store", "._photo.jpg", ".media_library.db", ".media_library.db-wal"}
	for _, f := range ignoredFiles {
		if !ShouldIgnoreFile(f) {
			t.Errorf("expected %s to be ignored as file", f)
		}
	}
	if ShouldIgnoreFile("photo.jpg") || ShouldIgnoreFile("movie.mp4") {
		t.Errorf("regular media files should not be ignored")
	}
}

func TestMimeSniffing(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create valid PNG image
	pngPath := filepath.Join(tempDir, "sample.png")
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)
	_ = os.WriteFile(pngPath, pngBuf.Bytes(), 0644)

	mime, isMedia, err := SniffMediaType(pngPath)
	if err != nil || !isMedia || mime != "image/png" {
		t.Fatalf("expected image/png, got mime=%q, isMedia=%v, err=%v", mime, isMedia, err)
	}

	// 2. Non-media file
	txtPath := filepath.Join(tempDir, "readme.txt")
	_ = os.WriteFile(txtPath, []byte("plain text"), 0644)

	_, isMedia, _ = SniffMediaType(txtPath)
	if isMedia {
		t.Errorf("readme.txt should not be recognized as media")
	}
}

func TestHasher(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.dat")
	data := []byte("deterministic content for hashing test")
	_ = os.WriteFile(filePath, data, 0644)

	h1, err := ComputeFullSHA256(filePath)
	if err != nil || h1 == "" {
		t.Fatalf("ComputeFullSHA256 failed: %v", err)
	}

	h2, err := ComputeFullSHA256(filePath)
	if err != nil || h1 != h2 {
		t.Fatalf("hash mismatch between runs: %s vs %s", h1, h2)
	}

	// Test sparse hash on small file falls back to full hash
	fastHash, err := ComputeFastHash(filePath, int64(len(data)))
	if err != nil || fastHash != h1 {
		t.Fatalf("expected fast hash to equal full hash on small file: got %s, want %s", fastHash, h1)
	}
}

func TestMetadataExtraction(t *testing.T) {
	tempDir := t.TempDir()
	jpegPath := filepath.Join(tempDir, "test.jpg")

	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	f, _ := os.Create(jpegPath)
	_ = jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	f.Close()

	meta, dur, metaJSON := ExtractMetadata(jpegPath, "image/jpeg")
	if meta.Width != 640 || meta.Height != 480 {
		t.Errorf("expected 640x480, got %dx%d", meta.Width, meta.Height)
	}
	if dur != nil {
		t.Errorf("image should not have video duration")
	}
	if metaJSON == "" || metaJSON == "{}" {
		t.Errorf("expected non-empty JSON metadata, got %s", metaJSON)
	}
}

func TestScannerPipeline_ScanAndDuplicates(t *testing.T) {
	driveRoot := t.TempDir()
	ctx := context.Background()

	// Create test files
	subDir := filepath.Join(driveRoot, "vacation")
	_ = os.MkdirAll(subDir, 0755)

	// Create 3 images (2 distinct, 1 exact duplicate)
	img1 := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		img1.Set(x, x, color.RGBA{R: 255, A: 255})
	}
	var buf1 bytes.Buffer
	_ = png.Encode(&buf1, img1)

	img2 := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		img2.Set(x, x, color.RGBA{G: 255, A: 255})
	}
	var buf2 bytes.Buffer
	_ = png.Encode(&buf2, img2)

	p1 := filepath.Join(subDir, "pic1.png")
	p2 := filepath.Join(subDir, "pic2.png")
	pDup := filepath.Join(driveRoot, "duplicate.png")
	pTxt := filepath.Join(driveRoot, "notes.txt")

	_ = os.WriteFile(p1, buf1.Bytes(), 0644)
	_ = os.WriteFile(p2, buf2.Bytes(), 0644)
	_ = os.WriteFile(pDup, buf1.Bytes(), 0644) // exact byte-for-byte duplicate of p1
	_ = os.WriteFile(pTxt, []byte("not media"), 0644)

	mockDB := newMockInserter()

	var pMu sync.Mutex
	progressCount := 0
	scanner := New(Config{
		DriveRoot:   driveRoot,
		WorkerCount: 4,
		BatchSize:   2,
	}, mockDB, func(p Progress) {
		pMu.Lock()
		progressCount++
		pMu.Unlock()
	})

	err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("scanner.Scan failed: %v", err)
	}

	status := scanner.Status()
	if status.MediaFound != 2 {
		t.Errorf("expected 2 unique media found, got %d", status.MediaFound)
	}
	if status.Duplicates != 1 {
		t.Errorf("expected 1 duplicate detected, got %d", status.Duplicates)
	}
	if len(mockDB.items) != 2 {
		t.Errorf("expected 2 items inserted into DB, got %d", len(mockDB.items))
	}

	pMu.Lock()
	defer pMu.Unlock()
	if progressCount == 0 {
		t.Errorf("expected progress callbacks to fire")
	}
}

func TestScanner_PauseResumeCancel(t *testing.T) {
	tempDir := t.TempDir()

	// Create 20 sample files
	for i := 0; i < 20; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 20, 20))
		img.Set(i, i, color.RGBA{B: uint8(i * 10), A: 255})
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		_ = os.WriteFile(filepath.Join(tempDir, filepath.Base(tempDir)+string(rune('a'+i))+".png"), buf.Bytes(), 0644)
	}

	mockDB := newMockInserter()
	scanner := New(Config{
		DriveRoot:   tempDir,
		WorkerCount: 2,
		BatchSize:   5,
	}, mockDB, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(10 * time.Millisecond)
		scanner.Pause()
		time.Sleep(20 * time.Millisecond)
		scanner.Resume()
	}()

	err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("scan with pause/resume failed: %v", err)
	}

	if scanner.Status().MediaFound != 20 {
		t.Errorf("expected all 20 media files found, got %d", scanner.Status().MediaFound)
	}
}

// memoryAlbums is a fake AlbumResolver recording creations and lookups.
type memoryAlbums struct {
	mu      sync.Mutex
	names   map[string]int64
	nextID  int64
	created []string
}

func newMemoryAlbums() *memoryAlbums {
	return &memoryAlbums{names: make(map[string]int64), nextID: 1}
}

func (a *memoryAlbums) GetByName(ctx context.Context, name string) (*domain.Album, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if id, ok := a.names[name]; ok {
		return &domain.Album{ID: id, Name: name}, nil
	}
	return nil, nil
}

func (a *memoryAlbums) Create(ctx context.Context, input domain.CreateAlbumInput) (*domain.Album, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if id, ok := a.names[input.Name]; ok {
		return &domain.Album{ID: id, Name: input.Name}, nil
	}
	id := a.nextID
	a.nextID++
	a.names[input.Name] = id
	a.created = append(a.created, input.Name)
	return &domain.Album{ID: id, Name: input.Name}, nil
}

func writeTestPNG(t *testing.T, path string, seed byte) {
	t.Helper()
	// Vary a pixel so successive files are not byte-identical duplicates.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: seed, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode failed: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write %s failed: %v", path, err)
	}
}

// TestScannerAssignsAlbumsByDirectory pins the directory-to-album rule: media
// found under a folder joins the album named after that folder (created on
// first use, merged afterwards); files at the scan root stay unassigned.
func TestScannerAssignsAlbumsByDirectory(t *testing.T) {
	driveRoot := t.TempDir()
	ctx := context.Background()

	vacation := filepath.Join(driveRoot, "Vacation")
	beach := filepath.Join(driveRoot, "Vacation", "Beach")
	if err := os.MkdirAll(beach, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	writeTestPNG(t, filepath.Join(vacation, "a.png"), 10)
	writeTestPNG(t, filepath.Join(beach, "b.png"), 20)
	writeTestPNG(t, filepath.Join(driveRoot, "root.png"), 30)

	albums := newMemoryAlbums()
	mockDB := newMockInserter()
	scanner := New(Config{
		DriveRoot:   driveRoot,
		WorkerCount: 2,
		Albums:      albums,
	}, mockDB, nil)

	if err := scanner.Scan(ctx); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	byName := make(map[string]domain.MediaItem)
	for _, item := range mockDB.items {
		byName[filepath.Base(item.CurrentRelativePath)] = item
	}

	if got := byName["a.png"].AlbumID; got == nil || *got != albums.names["Vacation"] {
		t.Errorf("a.png should join album 'Vacation' (id %d), got %v", albums.names["Vacation"], got)
	}
	// Current folder only: Beach/b.png belongs to 'Beach', not 'Vacation'.
	if got := byName["b.png"].AlbumID; got == nil || *got != albums.names["Beach"] {
		t.Errorf("b.png should join album 'Beach' (id %d), got %v", albums.names["Beach"], got)
	}
	if got := byName["root.png"].AlbumID; got != nil {
		t.Errorf("root.png sits in the scan root and should stay unassigned, got %v", *got)
	}
	if len(albums.created) != 2 {
		t.Errorf("expected albums created for exactly 2 new names, got %v", albums.created)
	}

	// A rescan merges into the existing albums instead of recreating them.
	albums.created = nil
	writeTestPNG(t, filepath.Join(vacation, "c.png"), 40)
	if err := scanner.Scan(ctx); err != nil {
		t.Fatalf("rescan failed: %v", err)
	}
	if len(albums.created) != 0 {
		t.Errorf("existing albums must be merged, not recreated; created %v", albums.created)
	}
	var assigned int
	for _, item := range mockDB.items {
		if filepath.Base(item.CurrentRelativePath) == "c.png" && item.AlbumID != nil && *item.AlbumID == albums.names["Vacation"] {
			assigned = 1
		}
	}
	if assigned != 1 {
		t.Errorf("c.png should have merged into existing album 'Vacation'")
	}
}
