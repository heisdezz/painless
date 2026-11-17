package storage

import (
	"context"
	"fmt"
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
