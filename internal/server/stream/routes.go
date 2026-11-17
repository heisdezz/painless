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

// Handler builds the playback router. The host owns authentication and CORS.
func (s Routes) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", s.handleStream)
	mux.HandleFunc("GET /thumbnail", s.handleThumbnail)
	return mux
}
