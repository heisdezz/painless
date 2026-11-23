// Package stream provides media playback routes for the mobile HTTP server.
package stream

import (
	"context"
	"net/http"

	"painless/internal/domain"
)

// Resolver finds cataloged media. A missing item is returned as nil, nil.
type Resolver interface {
	GetByPath(ctx context.Context, relPath string) (*domain.MediaItem, error)
	GetByID(ctx context.Context, id int64) (*domain.MediaItem, error)
}

// Thumbnailer supplies cached WebP thumbnails for media items.
type Thumbnailer interface {
	Get(ctx context.Context, item domain.MediaItem) ([]byte, error)
}

// Routes serves cataloged media and thumbnails. It does not open a listener.
// DriveRoot supplies the current sandbox root for each playback request.
type Routes struct {
	DriveRoot  func() string
	Resolver   Resolver
	Thumbnails Thumbnailer
}

// Handler builds the playback router; exposed for tests and embedding.
// Besides the query-parameter playback routes, it also serves the
// D-Stream-compatible ID-addressed routes GET /api/media/{id} (original
// file with byte ranges) and GET /api/thumbnail/{id}, plus their bare
// /media/{id} and /thumbnail/{id} forms — the original desktop server
// accepts both prefixes and the D-Stream app calls the bare ones.
func (s Routes) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", s.handleStream)
	mux.HandleFunc("GET /thumbnail", s.handleThumbnail)
	mux.HandleFunc("GET /api/media/{id}", s.handleMediaByID)
	mux.HandleFunc("GET /api/thumbnail/{id}", s.handleThumbnailByID)
	mux.HandleFunc("GET /media/{id}", s.handleMediaByID)
	mux.HandleFunc("GET /thumbnail/{id}", s.handleThumbnailByID)
	return mux
}
