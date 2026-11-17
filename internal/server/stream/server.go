// Package stream serves media files over local HTTP with byte-range
// support, sandboxed to the active drive root and the media library
// database.
package stream

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"painless/internal/domain"
	"painless/internal/storage"
)

// DefaultPort is the port the streaming server binds to unless configured
// otherwise.
const DefaultPort = 51789

// Resolver looks up cataloged media items for request validation.
type Resolver interface {
	// ByPath returns the item cataloged at relPath (relative to the drive
	// root), or an error wrapping ErrNotFound.
	ByPath(ctx context.Context, relPath string) (*domain.MediaItem, error)
	// ByID returns the item with the given database ID, or an error
	// wrapping ErrNotFound.
	ByID(ctx context.Context, id int64) (*domain.MediaItem, error)
}

// Thumbnailer supplies cached WebP thumbnails for media items.
type Thumbnailer interface {
	Get(ctx context.Context, item domain.MediaItem) ([]byte, error)
}

// ErrNotFound reports that no cataloged item matches a lookup. Handlers
// map it to 404; path sandbox violations map to 403.
var ErrNotFound = errors.New("media item not found")

// Options configures the streaming server.
type Options struct {
	// Port to listen on. Zero selects DefaultPort.
	Port int
	// Bind address, e.g. "127.0.0.1". Empty binds all interfaces so LAN
	// clients and mobile devices can stream.
	Bind string
	// DriveRoot is the sandbox root for stream paths. It can be changed
	// at runtime with SetDriveRoot when the active drive switches.
	DriveRoot string
	// Resolver validates requested paths and IDs against the library.
	Resolver Resolver
	// Thumbnails serves /thumbnail. Optional; without it the endpoint
	// reports 503.
	Thumbnails Thumbnailer
	// ReadTimeout and WriteTimeout guard stuck connections. Zero values
	// select generous defaults suited to slow media reads.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Server streams media from the active drive over HTTP.
type Server struct {
	opts Options

	mu        sync.RWMutex
	driveRoot string
	listener  net.Listener
	http      *http.Server

	started time.Time
}

// New creates a server; call Start to begin serving.
func New(opts Options) *Server {
	if opts.Port <= 0 {
		opts.Port = DefaultPort
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = 30 * time.Second
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 60 * time.Second
	}
	return &Server{
		opts:      opts,
		driveRoot: opts.DriveRoot,
		started:   time.Now(),
	}
}

// SetDriveRoot switches the sandbox root when the active drive changes.
// New requests immediately validate against the new root.
func (s *Server) SetDriveRoot(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.driveRoot = root
}

func (s *Server) root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.driveRoot
}

// Handler builds the routing table; exposed for tests and embedding.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", s.handleStream)
	mux.HandleFunc("HEAD /stream", s.handleStream)
	mux.HandleFunc("GET /thumbnail", s.handleThumbnail)
	mux.HandleFunc("GET /health", s.handleHealth)
	return mux
}

// Start binds the configured address and serves in a background
// goroutine. The port is bound before Start returns, so callers can read
// Addr immediately.
func (s *Server) Start() error {
	addr := net.JoinHostPort(s.opts.Bind, strconv.Itoa(s.opts.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("stream server listen %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler:      s.Handler(),
		ReadTimeout:  s.opts.ReadTimeout,
		WriteTimeout: s.opts.WriteTimeout,
	}
	s.mu.Lock()
	s.listener = ln
	s.http = srv
	s.mu.Unlock()
	// Serve the local copy: Shutdown may nil s.http concurrently.
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

// Addr returns the bound address, empty before Start.
func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Shutdown stops accepting connections and waits for in-flight requests
// to finish or ctx to expire. It is safe to call more than once.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.http
	ln := s.listener
	s.http, s.listener = nil, nil
	s.mu.Unlock()

	if srv == nil {
		return nil
	}
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}

// RepoResolver adapts the storage layer to Resolver with the exact-path
// lookup the repositories do not expose directly.
type RepoResolver struct {
	DB *storage.DB
}

const lookupColumns = "id, file_hash, current_relative_path, file_size, mime_type"

// ByPath fetches the item whose current_relative_path matches exactly.
func (r RepoResolver) ByPath(ctx context.Context, relPath string) (*domain.MediaItem, error) {
	return scanItem(r.DB.Raw().QueryRowContext(ctx,
		"SELECT "+lookupColumns+" FROM media_items WHERE current_relative_path = ? LIMIT 1", relPath), relPath)
}

// ByID fetches the item by primary key.
func (r RepoResolver) ByID(ctx context.Context, id int64) (*domain.MediaItem, error) {
	return scanItem(r.DB.Raw().QueryRowContext(ctx,
		"SELECT "+lookupColumns+" FROM media_items WHERE id = ?", id), fmt.Sprint(id))
}

func scanItem(row *sql.Row, key string) (*domain.MediaItem, error) {
	item := &domain.MediaItem{}
	err := row.Scan(&item.ID, &item.FileHash, &item.CurrentRelativePath, &item.FileSize, &item.MimeType)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("%w: %q", ErrNotFound, key)
	case err != nil:
		return nil, fmt.Errorf("lookup media %q: %w", key, err)
	}
	return item, nil
}
