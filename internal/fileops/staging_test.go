package fileops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"painless/internal/domain"
)

type mockMediaStore struct {
	paths   map[int64]string
	albums  map[int64]int64
	deleted map[int64]bool
}

func newMockMediaStore() *mockMediaStore {
	return &mockMediaStore{
		paths:   make(map[int64]string),
		albums:  make(map[int64]int64),
		deleted: make(map[int64]bool),
	}
}

func (m *mockMediaStore) UpdatePath(ctx context.Context, id int64, newPath string) error {
	m.paths[id] = newPath
	return nil
}

func (m *mockMediaStore) MoveToAlbum(ctx context.Context, targetAlbumID int64, mediaIDs []int64) (int64, error) {
	for _, id := range mediaIDs {
		m.albums[id] = targetAlbumID
	}
	return int64(len(mediaIDs)), nil
}

func (m *mockMediaStore) RemoveFromAlbum(ctx context.Context, mediaIDs []int64) (int64, error) {
	for _, id := range mediaIDs {
		delete(m.albums, id)
	}
	return int64(len(mediaIDs)), nil
}

func (m *mockMediaStore) DeleteBatch(ctx context.Context, mediaIDs []int64) (int64, error) {
	for _, id := range mediaIDs {
		m.deleted[id] = true
	}
	return int64(len(mediaIDs)), nil
}

type mockAlbumStore struct {
	albums map[int64]*domain.Album
}

func (a *mockAlbumStore) GetByID(ctx context.Context, id int64) (*domain.Album, error) {
	return a.albums[id], nil
}

func TestScaffoldLibrary(t *testing.T) {
	tempDir := t.TempDir()

	err := ScaffoldLibrary(tempDir)
	if err != nil {
		t.Fatalf("ScaffoldLibrary failed: %v", err)
	}

	expectedDirs := []string{
		AlbumsDir(tempDir),
		UnknownDir(tempDir),
		ThumbnailsDir(tempDir),
		BackupsDir(tempDir),
		SettingsDir(tempDir),
	}

	for _, d := range expectedDirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}

	albumDir, err := EnsureAlbumDir(tempDir, "Summer Vacation/2026")
	if err != nil {
		t.Fatalf("EnsureAlbumDir failed: %v", err)
	}
	if _, err := os.Stat(albumDir); err != nil {
		t.Errorf("album directory was not created: %v", err)
	}
}

func TestSanitizationAndNaming(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal Name"},
		{"Bad/Slash:Colon*Star?Question", "Bad_Slash_Colon_Star_Question"},
		{"...dots...", "dots"},
		{"CON", "_CON"},
		{"AUX", "_AUX"},
		{"../traversal", "_traversal"},
	}

	for _, tc := range cases {
		got := SanitizePathSegment(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizePathSegment(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
	}

	fn := SanitizeFilename("bad:name?.JPEG")
	if fn != "bad_name_.JPEG" {
		t.Errorf("SanitizeFilename unexpected: got %q", fn)
	}
}

func TestResolveCollision(t *testing.T) {
	tempDir := t.TempDir()

	contentA := []byte("content A")
	hashABytes := sha256.Sum256(contentA)
	hashA := hex.EncodeToString(hashABytes[:])

	contentB := []byte("content B - distinct")
	hashBBytes := sha256.Sum256(contentB)
	hashB := hex.EncodeToString(hashBBytes[:])

	testFile := filepath.Join(tempDir, "image.jpg")
	if err := os.WriteFile(testFile, contentA, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Same hash should detect duplicate
	path, isDup, err := ResolveCollision(testFile, hashA)
	if err != nil || !isDup || path != testFile {
		t.Fatalf("expected duplicate detection for same hash: path=%s, isDup=%v, err=%v", path, isDup, err)
	}

	// 2. Different hash should resolve collision by appending suffix
	path, isDup, err = ResolveCollision(testFile, hashB)
	if err != nil || isDup {
		t.Fatalf("expected collision resolution: isDup=%v, err=%v", isDup, err)
	}
	expectedCandidate := filepath.Join(tempDir, "image_1.jpg")
	if path != expectedCandidate {
		t.Fatalf("expected %s, got %s", expectedCandidate, path)
	}
}

func TestMoveFile_AtomicAndCopyVerify(t *testing.T) {
	tempDir := t.TempDir()

	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "sub", "dest.txt")

	content := []byte("testing move file integrity")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	// Move via MoveFile
	if err := MoveFile(srcFile, dstFile); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// Check dst exists with correct content
	readBack, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed reading dest: %v", err)
	}
	if string(readBack) != string(content) {
		t.Errorf("content mismatch after move: got %q", string(readBack))
	}

	// Check src is gone
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("source file should have been removed")
	}

	// Verify fallback copyAndVerifyMove explicitly
	src2 := filepath.Join(tempDir, "source2.txt")
	dst2 := filepath.Join(tempDir, "dest2.txt")
	if err := os.WriteFile(src2, content, 0644); err != nil {
		t.Fatalf("failed to write source2: %v", err)
	}
	stat2, _ := os.Stat(src2)
	if err := copyAndVerifyMove(src2, dst2, stat2); err != nil {
		t.Fatalf("copyAndVerifyMove failed: %v", err)
	}
	if _, err := os.Stat(src2); !os.IsNotExist(err) {
		t.Errorf("source2 should be removed after copyAndVerifyMove")
	}
}

func TestStagingEngine_ExecuteAndRollback(t *testing.T) {
	driveRoot := t.TempDir()
	ctx := context.Background()

	// Create incoming files
	incomingDir := filepath.Join(driveRoot, "incoming")
	_ = os.MkdirAll(incomingDir, 0755)

	file1 := filepath.Join(incomingDir, "photo1.jpg")
	file2 := filepath.Join(incomingDir, "video1.mp4")
	file3 := filepath.Join(incomingDir, "temp.txt")

	_ = os.WriteFile(file1, []byte("photo 1 data"), 0644)
	_ = os.WriteFile(file2, []byte("video 1 data"), 0644)
	_ = os.WriteFile(file3, []byte("delete me"), 0644)

	h1, _ := CalculateFileSHA256(file1)
	h2, _ := CalculateFileSHA256(file2)
	h3, _ := CalculateFileSHA256(file3)

	item1 := domain.MediaItem{ID: 1, FileHash: h1, CurrentRelativePath: "incoming/photo1.jpg"}
	item2 := domain.MediaItem{ID: 2, FileHash: h2, CurrentRelativePath: "incoming/video1.mp4"}
	item3 := domain.MediaItem{ID: 3, FileHash: h3, CurrentRelativePath: "incoming/temp.txt"}

	stager := NewStagingEngine()

	var albumID int64 = 42
	stager.StageMove(item1, &albumID, "Travel")
	stager.StageUnknown(item2)
	stager.StageDelete(item3)

	summary := stager.Summary()
	if summary.TotalOperations != 3 {
		t.Fatalf("expected 3 total operations, got %d", summary.TotalOperations)
	}
	if summary.AlbumMoveCount["Travel"] != 1 {
		t.Errorf("expected 1 travel move")
	}
	if summary.UnknownCount != 1 {
		t.Errorf("expected 1 unknown move")
	}
	if summary.DeleteCount != 1 {
		t.Errorf("expected 1 delete")
	}

	mediaStore := newMockMediaStore()
	mediaStore.paths[1] = "incoming/photo1.jpg"
	mediaStore.paths[2] = "incoming/video1.mp4"
	mediaStore.paths[3] = "incoming/temp.txt"

	albumStore := &mockAlbumStore{
		albums: map[int64]*domain.Album{
			42: {ID: 42, Name: "Travel"},
		},
	}

	progressCalls := 0
	report, err := stager.Execute(ctx, driveRoot, mediaStore, albumStore, func(cur, tot int, op domain.StagedOperation) {
		progressCalls++
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if report.MovedCount != 2 {
		t.Errorf("expected 2 moves, got %d", report.MovedCount)
	}
	if report.DeletedCount != 1 {
		t.Errorf("expected 1 delete, got %d", report.DeletedCount)
	}
	if progressCalls != 3 {
		t.Errorf("expected 3 progress calls, got %d", progressCalls)
	}

	// Verify file locations
	expectedTravelPath := filepath.Join(driveRoot, "albums", "Travel", "photo1.jpg")
	if _, err := os.Stat(expectedTravelPath); err != nil {
		t.Errorf("expected photo1 to be moved to %s: %v", expectedTravelPath, err)
	}
	expectedUnknownPath := filepath.Join(driveRoot, "unknown", "video1.mp4")
	if _, err := os.Stat(expectedUnknownPath); err != nil {
		t.Errorf("expected video1 to be moved to %s: %v", expectedUnknownPath, err)
	}
	if _, err := os.Stat(file3); !os.IsNotExist(err) {
		t.Errorf("file3 should have been deleted")
	}

	// Verify DB updates
	if mediaStore.paths[1] != "albums/Travel/photo1.jpg" {
		t.Errorf("mediaStore path mismatch for 1: got %s", mediaStore.paths[1])
	}
	if mediaStore.albums[1] != 42 {
		t.Errorf("mediaStore album mismatch for 1: got %d", mediaStore.albums[1])
	}
	if mediaStore.paths[2] != "unknown/video1.mp4" {
		t.Errorf("mediaStore path mismatch for 2: got %s", mediaStore.paths[2])
	}
	if !mediaStore.deleted[3] {
		t.Errorf("mediaStore should mark 3 as deleted")
	}

	// Test Rollback of moved files
	if err := stager.Rollback(ctx, driveRoot, mediaStore); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// After rollback, photo1 and video1 should be back in incoming/
	if _, err := os.Stat(file1); err != nil {
		t.Errorf("file1 should be back in incoming: %v", err)
	}
	if _, err := os.Stat(file2); err != nil {
		t.Errorf("file2 should be back in incoming: %v", err)
	}
	if mediaStore.paths[1] != "incoming/photo1.jpg" {
		t.Errorf("mediaStore path should be rolled back to incoming/photo1.jpg, got %s", mediaStore.paths[1])
	}
}
