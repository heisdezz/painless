package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"painless/internal/domain"
)

func TestAlbumRepoCRUD(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewAlbumRepo(db)
	ctx := context.Background()

	// 1. Create
	album, err := repo.Create(ctx, domain.CreateAlbumInput{
		Name: "Vacation 2026",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if album.ID == 0 || album.RelativePath != "albums/Vacation 2026" {
		t.Fatalf("unexpected album output: %+v", album)
	}

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, album.ID)
	if err != nil || fetched == nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Name != "Vacation 2026" {
		t.Errorf("expected 'Vacation 2026', got '%s'", fetched.Name)
	}

	// 3. Update
	newDesc := "Trip to the beach"
	err = repo.Update(ctx, domain.EditAlbumInput{
		ID:          album.ID,
		NewName:     "Summer 2026",
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := repo.GetByID(ctx, album.ID)
	if updated.Name != "Summer 2026" || updated.RelativePath != "albums/Summer 2026" || *updated.Description != newDesc {
		t.Fatalf("unexpected updated album: %+v", updated)
	}

	// 4. List
	summaries, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "Summer 2026" {
		t.Fatalf("expected 1 album summary, got %+v", summaries)
	}

	// 5. Delete
	affected, err := repo.Delete(ctx, album.ID, false)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if affected != 0 {
		t.Errorf("expected 0 affected items for empty album delete, got %d", affected)
	}

	afterDelete, _ := repo.GetByID(ctx, album.ID)
	if afterDelete != nil {
		t.Errorf("expected album to be deleted, got %+v", afterDelete)
	}
}

func TestAlbumMerge(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	albumRepo := NewAlbumRepo(db)
	mediaRepo := NewMediaRepo(db)
	ctx := context.Background()

	target, _ := albumRepo.Create(ctx, domain.CreateAlbumInput{Name: "Target"})
	src1, _ := albumRepo.Create(ctx, domain.CreateAlbumInput{Name: "Src1"})
	src2, _ := albumRepo.Create(ctx, domain.CreateAlbumInput{Name: "Src2"})

	// Add items to src1 and src2
	items := []domain.MediaItem{
		{FileHash: "h1", OriginalRelativePath: "p1", CurrentRelativePath: "albums/Src1/p1", FileSize: 100, MimeType: "image/jpeg", AlbumID: &src1.ID},
		{FileHash: "h2", OriginalRelativePath: "p2", CurrentRelativePath: "albums/Src2/p2", FileSize: 200, MimeType: "image/png", AlbumID: &src2.ID},
	}
	_, err = mediaRepo.InsertBatch(ctx, items)
	if err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	moved, err := albumRepo.Merge(ctx, target.ID, []int64{src1.ID, src2.ID})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	if moved != 2 {
		t.Errorf("expected 2 moved items, got %d", moved)
	}

	// Check that src1 and src2 are deleted
	s1, _ := albumRepo.GetByID(ctx, src1.ID)
	if s1 != nil {
		t.Errorf("expected src1 to be deleted")
	}

	// Check items are in target
	targetItems, _ := mediaRepo.List(ctx, domain.MediaListParams{AlbumID: &target.ID})
	if targetItems.Total != 2 {
		t.Errorf("expected 2 items in target album, got %d", targetItems.Total)
	}
}

func TestMediaRepoSortByAdded(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	items := []domain.MediaItem{
		{FileHash: "a", OriginalRelativePath: "photos/a.jpg", CurrentRelativePath: "photos/a.jpg", FileSize: 1, MimeType: "image/jpeg", CreatedAt: now},
		{FileHash: "b", OriginalRelativePath: "photos/b.jpg", CurrentRelativePath: "photos/b.jpg", FileSize: 2, MimeType: "image/jpeg", CreatedAt: now.Add(-2 * time.Hour)},
		{FileHash: "c", OriginalRelativePath: "photos/c.jpg", CurrentRelativePath: "photos/c.jpg", FileSize: 3, MimeType: "image/jpeg", CreatedAt: now.Add(-time.Hour)},
	}
	if _, err := repo.InsertBatch(ctx, items); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	page, err := repo.List(ctx, domain.MediaListParams{
		Limit:     10,
		SortBy:    domain.SortByAdded,
		SortOrder: domain.SortOrderDesc,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(page.Items))
	}
	// Newest by insertion: c, b, a — even though a has the newest created_at.
	want := []string{"photos/c.jpg", "photos/b.jpg", "photos/a.jpg"}
	for i, item := range page.Items {
		if item.CurrentRelativePath != want[i] {
			t.Errorf("item %d = %q, want %q", i, item.CurrentRelativePath, want[i])
		}
	}
}

func TestMediaRepoQueryAndPagination(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	// Insert 25 items
	var items []domain.MediaItem
	for i := 1; i <= 25; i++ {
		mime := "image/jpeg"
		if i%3 == 0 {
			mime = "video/mp4"
		}
		items = append(items, domain.MediaItem{
			FileHash:             fmt.Sprintf("hash_%03d", i),
			OriginalRelativePath: fmt.Sprintf("DCIM/IMG_%03d.JPG", i),
			CurrentRelativePath:  fmt.Sprintf("unknown/IMG_%03d.JPG", i),
			FileSize:             int64(i * 1000),
			MimeType:             mime,
			CreatedAt:            time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	inserted, err := repo.InsertBatch(ctx, items)
	if err != nil || inserted != 25 {
		t.Fatalf("InsertBatch failed: %v (inserted: %d)", err, inserted)
	}

	// Test pagination page 1 (limit 10)
	page1, err := repo.List(ctx, domain.MediaListParams{
		Limit:     10,
		Offset:    0,
		SortBy:    domain.SortByName,
		SortOrder: domain.SortOrderAsc,
	})
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}
	if page1.Total != 25 || len(page1.Items) != 10 {
		t.Fatalf("expected total 25 and 10 items, got total %d, items %d", page1.Total, len(page1.Items))
	}
	if page1.Items[0].CurrentRelativePath != "unknown/IMG_001.JPG" {
		t.Errorf("expected IMG_001.JPG first, got %s", page1.Items[0].CurrentRelativePath)
	}

	// Test video filter
	videos, err := repo.List(ctx, domain.MediaListParams{
		Filter: domain.MediaFilterVideos,
	})
	if err != nil {
		t.Fatalf("List videos failed: %v", err)
	}
	if videos.Total != 8 { // 3, 6, 9, 12, 15, 18, 21, 24
		t.Errorf("expected 8 videos, got %d", videos.Total)
	}

	// Test search
	searched, err := repo.List(ctx, domain.MediaListParams{
		Search: "IMG_01",
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	// Matches IMG_010.JPG through IMG_019.JPG + IMG_001.JPG (does not match 001) -> 10 matches (010..019)
	if searched.Total != 10 {
		t.Errorf("expected 10 search results for 'IMG_01', got %d", searched.Total)
	}

	// Test Navigation (next / prev)
	firstItem := page1.Items[0]
	item, prevID, nextID, err := repo.GetItemWithNavigation(ctx, firstItem.ID, nil)
	if err != nil {
		t.Fatalf("GetItemWithNavigation failed: %v", err)
	}
	if item == nil || prevID != nil || nextID == nil {
		t.Errorf("expected first item to have no prevID and valid nextID, got prev: %v, next: %v", prevID, nextID)
	}
}

func BenchmarkMediaList10kRows(b *testing.B) {
	db, err := OpenMemory()
	if err != nil {
		b.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	// Seed 10,000 rows
	var batch []domain.MediaItem
	now := time.Now().UTC()
	for i := 1; i <= 10000; i++ {
		batch = append(batch, domain.MediaItem{
			FileHash:             fmt.Sprintf("hash_%06d", i),
			OriginalRelativePath: fmt.Sprintf("DCIM/IMG_%06d.JPG", i),
			CurrentRelativePath:  fmt.Sprintf("unknown/IMG_%06d.JPG", i),
			FileSize:             int64(i * 1024),
			MimeType:             "image/jpeg",
			CreatedAt:            now.Add(time.Duration(i) * time.Second),
		})
		if len(batch) >= 1000 {
			if _, err := repo.InsertBatch(ctx, batch); err != nil {
				b.Fatalf("InsertBatch failed: %v", err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		_, _ = repo.InsertBatch(ctx, batch)
	}

	params := domain.MediaListParams{
		Limit:     50,
		Offset:    2500,
		SortBy:    domain.SortByDate,
		SortOrder: domain.SortOrderDesc,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		page, err := repo.List(ctx, params)
		if err != nil || len(page.Items) != 50 {
			b.Fatalf("benchmark list failed: %v", err)
		}
	}
}

func TestMediaRepoSearchIndexed(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	items := []domain.MediaItem{
		{FileHash: "h1", OriginalRelativePath: "photos/vacation-beach.jpg", CurrentRelativePath: "photos/vacation-beach.jpg", FileSize: 100, MimeType: "image/jpeg", CreatedAt: time.Now()},
		{FileHash: "h2", OriginalRelativePath: "videos/hiking.mp4", CurrentRelativePath: "videos/hiking.mp4", FileSize: 200, MimeType: "video/mp4", CreatedAt: time.Now().Add(-time.Minute)},
		{FileHash: "h3", OriginalRelativePath: "photos/IMG_0001.JPG", CurrentRelativePath: "photos/IMG_0001.JPG", FileSize: 50, MimeType: "image/jpeg", MetadataJSON: `{"camera":"Sony"}`, CreatedAt: time.Now().Add(-2 * time.Minute)},
	}
	if _, err := repo.InsertBatch(ctx, items); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	cases := []struct {
		name   string
		search string
		want   []string
	}{
		{"path substring", "beach", []string{"photos/vacation-beach.jpg"}},
		{"long path substring", "vacation-beach", []string{"photos/vacation-beach.jpg"}},
		{"metadata substring", "Sony", []string{"photos/IMG_0001.JPG"}},
		{"case insensitive", "VAC", []string{"photos/vacation-beach.jpg"}},
		{"no match", "zebra", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := repo.List(ctx, domain.MediaListParams{Limit: 10, Search: tc.search})
			if err != nil {
				t.Fatalf("List search=%q: %v", tc.search, err)
			}
			got := make([]string, 0, len(page.Items))
			for _, item := range page.Items {
				got = append(got, item.CurrentRelativePath)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("search=%q items = %v, want %v", tc.search, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("search=%q items = %v, want %v", tc.search, got, tc.want)
				}
			}
		})
	}

	// Short searches fall back to the LIKE path and keep matching.
	page, err := repo.List(ctx, domain.MediaListParams{Limit: 10, Search: "ac"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 { // "vacation-beach.jpg" contains "ac"
		t.Fatalf("short search total = %d, want 1", page.Total)
	}
}

// benchmarkMediaRepoSearch seeds a representative library and measures the
// search box: "IMG_0" matches by path, "note" matches by metadata.
func benchmarkMediaRepoSearch(b *testing.B, search string) {
	db, err := OpenMemory()
	if err != nil {
		b.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	const total = 10000
	var items []domain.MediaItem
	for i := 0; i < total; i++ {
		items = append(items, domain.MediaItem{
			FileHash:             fmt.Sprintf("hash-%d", i),
			OriginalRelativePath: filepath.Join("library", fmt.Sprintf("sub%02d", i%50), fmt.Sprintf("IMG_%05d.JPG", i)),
			CurrentRelativePath:  filepath.Join("photos", fmt.Sprintf("sub%02d", i%50), fmt.Sprintf("IMG_%05d.JPG", i)),
			FileSize:             int64(1024 + i%1000),
			MimeType:             "image/jpeg",
			CreatedAt:            time.Now(),
		})
	}
	if _, err := repo.InsertBatch(ctx, items); err != nil {
		b.Fatalf("seed: %v", err)
	}
	for i := 0; i < total/10; i++ {
		if _, err := db.Raw().ExecContext(ctx,
			"UPDATE media_items SET metadata_json = ? WHERE id = ?",
			fmt.Sprintf(`{"camera":"Canon","note":"photo number %d"}`, i), i+1); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		page, err := repo.List(ctx, domain.MediaListParams{Limit: 10, Search: search})
		if err != nil || page.Total == 0 {
			b.Fatalf("benchmark search %q failed: err=%v total=%d", search, err, page.Total)
		}
	}
}

func BenchmarkMediaRepoSearchPath(b *testing.B)     { benchmarkMediaRepoSearch(b, "IMG_0") }
func BenchmarkMediaRepoSearchMetadata(b *testing.B) { benchmarkMediaRepoSearch(b, "note") }

func TestInsertBatchDuplicateHashDedup(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	item := domain.MediaItem{FileHash: "hash-dup", OriginalRelativePath: "incoming/a.jpg", CurrentRelativePath: "incoming/a.jpg", FileSize: 10, MimeType: "image/jpeg", CreatedAt: time.Now()}
	if _, err := repo.InsertBatch(ctx, []domain.MediaItem{item}); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	dup := item
	dup.CurrentRelativePath = "incoming/a-copy.jpg"
	if _, err := repo.InsertBatch(ctx, []domain.MediaItem{dup}); err != nil {
		t.Fatalf("duplicate insert reported an error: %v", err)
	}
	if n := countMediaRows(t, db, "hash-dup"); n != 1 {
		t.Fatalf("rows for hash-dup = %d, want 1", n)
	}

	other := domain.MediaItem{FileHash: "hash-other", OriginalRelativePath: "incoming/b.jpg", CurrentRelativePath: "incoming/b.jpg", FileSize: 20, MimeType: "image/jpeg", CreatedAt: time.Now()}
	if _, err := repo.InsertBatch(ctx, []domain.MediaItem{other}); err != nil {
		t.Fatalf("insert of a new hash failed: %v", err)
	}
	if n := countMediaRows(t, db, "hash-other"); n != 1 {
		t.Fatalf("rows for hash-other = %d, want 1", n)
	}
}

func TestInsertBatchConcurrentDuplicateHash(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer db.Close()

	repo := NewMediaRepo(db)
	ctx := context.Background()

	item := domain.MediaItem{FileHash: "hash-conc", OriginalRelativePath: "incoming/a.jpg", CurrentRelativePath: "incoming/a.jpg", FileSize: 10, MimeType: "image/jpeg", CreatedAt: time.Now()}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dup := item
			dup.CurrentRelativePath = fmt.Sprintf("incoming/c%d.jpg", i)
			if _, err := repo.InsertBatch(ctx, []domain.MediaItem{dup}); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent duplicate insert: %v", err)
	}
	if n := countMediaRows(t, db, item.FileHash); n != 1 {
		t.Fatalf("rows for %q = %d, want 1", item.FileHash, n)
	}
}

func TestMigrationDedupSQLLogic(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE media_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_hash TEXT NOT NULL,
		current_relative_path TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	// Pre-existing duplicates: two rows for hash "h1", one for "h2".
	for _, tc := range []struct {
		hash string
		path string
	}{
		{"h1", "a.jpg"},
		{"h1", "a-dup.jpg"},
		{"h2", "b.jpg"},
	} {
		if _, err := db.Exec("INSERT INTO media_items(file_hash, current_relative_path) VALUES (?, ?)", tc.hash, tc.path); err != nil {
			t.Fatal(err)
		}
	}

	// v3 statements: dedupe keeping the earliest row per hash, then
	// enforce the unique index.
	if _, err := db.Exec("DELETE FROM media_items WHERE id NOT IN (SELECT MIN(id) FROM media_items GROUP BY file_hash)"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM media_items").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows after dedup = %d, want 2", n)
	}
	var kept string
	if err := db.QueryRow("SELECT current_relative_path FROM media_items WHERE file_hash = 'h1'").Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if kept != "a.jpg" {
		t.Fatalf("kept row for h1 = %q, want a.jpg (earliest per hash)", kept)
	}
}

func countMediaRows(t *testing.T, db *DB, hash string) int {
	t.Helper()
	var n int
	if err := db.Raw().QueryRow("SELECT COUNT(*) FROM media_items WHERE file_hash = ?", hash).Scan(&n); err != nil {
		t.Fatalf("count media by hash: %v", err)
	}
	return n
}
