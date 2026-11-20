// Package mobile serves the authenticated REST and upload API consumed
// by mobile companion apps on the local Wi-Fi network, including playback
// and thumbnails for cataloged media.
package mobile

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"painless/internal/domain"
	"painless/internal/server/stream"
	"painless/internal/storage"
)

// DefaultPort is the port the mobile sync server binds to unless
// configured otherwise.
const DefaultPort = 51790

// DefaultMaxUploadBytes caps a single upload at 2 GiB.
const DefaultMaxUploadBytes = int64(2) << 30

// ThumbnailWarmer schedules thumbnail generation for freshly uploaded
// media. *thumbnail.Service satisfies it.
type ThumbnailWarmer interface {
	Get(ctx context.Context, item domain.MediaItem) ([]byte, error)
}

// Options configures the mobile sync server.
type Options struct {
	// Port to listen on. Zero selects DefaultPort.
	Port int
	// Bind address, e.g. "0.0.0.0". Empty binds all interfaces so phones
	// on the LAN can reach the API.
	Bind string
	// Token is the pairing token; write requests must present it as a
	// Bearer credential. Empty disables writes (503) rather than leaving
	// the drive writable by anyone.
	Token string
	// ProtectReads also requires the token for GET requests. Ping stays
	// open either way so clients can verify reachability before pairing.
	ProtectReads bool
	// DriveRoot is the active external drive; uploads land on it. It can
	// be changed at runtime with SetDriveRoot.
	DriveRoot string
	// Albums and Media are the library repositories backing the API.
	Albums *storage.AlbumRepo
	Media  *storage.MediaRepo
	// Thumbnails, when set, serves thumbnails and warms them in the
	// background for newly uploaded files.
	Thumbnails ThumbnailWarmer
	// DriveInfo, when set, supplies the active drive snapshot reported by
	// /api/status.
	DriveInfo func() *domain.DriveInfo
	// MaxUploadBytes caps one upload. Zero selects DefaultMaxUploadBytes.
	MaxUploadBytes int64
}

// Server exposes the mobile companion REST API over HTTP.
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
	if opts.MaxUploadBytes <= 0 {
		opts.MaxUploadBytes = DefaultMaxUploadBytes
	}
	return &Server{
		opts:      opts,
		driveRoot: opts.DriveRoot,
		started:   time.Now(),
	}
}

// SetDriveRoot switches the target drive when the active drive changes.
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

// cors sets permissive CORS headers (the API is consumed by companion
// web views) and short-circuits preflight requests.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range, X-File-Name, X-Album-ID")
		h.Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Length, Content-Range")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler builds the routing table; exposed for tests and embedding.
//
// Alongside the native Painless API it serves the D-Stream Mobile LAN
// compatibility surface: full album/media CRUD under /api/albums/{id}
// and /api/media/{id}, ID-based streaming and thumbnails, and uploads
// via POST /api/media with D-Stream's multipart/raw transports.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	guard := s.authGuard()
	mux.Handle("GET /api/status", guard(s.handleStatus))
	mux.HandleFunc("GET /api/ping", s.handlePing)
	mux.Handle("GET /api/albums", guard(s.handleListAlbums))
	mux.Handle("POST /api/albums", guard(s.handleCreateAlbum))
	mux.Handle("GET /api/albums/{id}", guard(s.handleGetAlbum))
	mux.Handle("PATCH /api/albums/{id}", guard(s.handleUpdateAlbum))
	mux.Handle("DELETE /api/albums/{id}", guard(s.handleDeleteAlbum))
	mux.Handle("GET /api/media", guard(s.handleListMedia))
	mux.Handle("GET /api/media/{id}/details", guard(s.handleGetMediaDetails))
	mux.Handle("POST /api/media", guard(s.handleCreateMedia))
	mux.Handle("PATCH /api/media/{id}", guard(s.handleMoveMedia))
	mux.Handle("DELETE /api/media/{id}", guard(s.handleDeleteMedia))
	mux.Handle("POST /api/upload", guard(s.handleUpload))

	playback := stream.Routes{DriveRoot: s.root, Thumbnails: s.opts.Thumbnails}
	if s.opts.Media != nil {
		playback.Resolver = s.opts.Media
	}
	mediaHandler := playback.Handler()
	// Playback routes live in the stream package: path-based /stream,
	// query-based /thumbnail, and the ID-addressed /api/media/{id} file
	// stream and /api/thumbnail/{id}.
	mux.Handle("GET /stream", guard(mediaHandler.ServeHTTP))
	mux.Handle("GET /thumbnail", guard(mediaHandler.ServeHTTP))
	mux.Handle("GET /api/media/{id}", guard(mediaHandler.ServeHTTP))
	mux.Handle("GET /api/thumbnail/{id}", guard(mediaHandler.ServeHTTP))

	return s.cors(mux)
}

// Start binds the configured address and serves in a background
// goroutine. The port is bound before Start returns.
func (s *Server) Start() error {
	addr := net.JoinHostPort(s.opts.Bind, strconv.Itoa(s.opts.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("mobile server listen %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler: s.Handler(),
		// Generous write timeout: 2 GiB uploads over Wi-Fi are slow.
		WriteTimeout: 10 * time.Minute,
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
// to finish or ctx to expire. Safe to call more than once.
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

// LANAddress returns the first private non-loopback IPv4 address of this
// host joined with port, for QR pairing and on-screen instructions.
func LANAddress(port int) (string, error) {
	if port <= 0 {
		port = DefaultPort
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("enumerate interfaces: %w", err)
	}
	fallback := ""
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLoopback() {
			continue
		}
		candidate := net.JoinHostPort(ipnet.IP.String(), strconv.Itoa(port))
		// Prefer the common RFC1918 home-subnet ranges first.
		if ipnet.IP.IsPrivate() {
			return candidate, nil
		}
		if fallback == "" {
			fallback = candidate
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no non-loopback IPv4 address available")
}
