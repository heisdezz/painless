package shell

import (
	"context"
	"testing"

	"painless/internal/domain"
	"painless/internal/storage"
)

func TestDashboardStatsCountMatchesDatabase(t *testing.T) {
	root := makeLibraryDrive(t, "stats")
	db, err := storage.Open(root, storage.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	media := storage.NewMediaRepo(db)
	source := dashboardSource{media: media, albums: storage.NewAlbumRepo(db)}
	stats, err := source.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalMedia != 1 {
		t.Fatalf("TotalMedia = %d, want 1", stats.TotalMedia)
	}

	_, err = media.InsertBatch(context.Background(), []domain.MediaItem{{
		FileHash: "hash-two", CurrentRelativePath: "photos/two.jpg", MimeType: "image/jpeg", FileSize: 1024,
	}})
	if err != nil {
		t.Fatal(err)
	}
	stats, err = source.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalMedia != 2 {
		t.Fatalf("TotalMedia after insert = %d, want 2", stats.TotalMedia)
	}
}
