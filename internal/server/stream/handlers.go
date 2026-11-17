package stream

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// cacheImmutable marks hash-addressed thumbnails as cacheable forever.
const cacheImmutable = "public, max-age=31536000, immutable"

// handleStream serves a cataloged media file with full Range support via
// http.ServeContent. Query: path=<drive-relative path>.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	allowCORS(w)

	if s.opts.Resolver == nil {
		http.Error(w, "library resolver not configured", http.StatusServiceUnavailable)
		return
	}
	relPath := r.URL.Query().Get("path")
	// Reject hostile request paths up front: the catalog lookup is keyed
	// on this value, and traversal/absolute forms must never reach the
	// drive sandbox.
	if err := ValidateRelPath(relPath); err != nil {
		resolveError(w, err)
		return
	}
	item, err := s.opts.Resolver.ByPath(r.Context(), relPath)
	if err != nil {
		lookupError(w, err)
		return
	}

	path, err := ResolvePath(s.root(), item.CurrentRelativePath)
	if err != nil {
		resolveError(w, err)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "file not found on drive", http.StatusNotFound)
			return
		}
		http.Error(w, "cannot open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if item.MimeType != "" {
		w.Header().Set("Content-Type", item.MimeType)
	} else if byExt := mime.TypeByExtension(filepath.Ext(path)); byExt != "" {
		w.Header().Set("Content-Type", byExt)
	}
	// ServeContent sets Accept-Ranges, Content-Length and Content-Range,
	// and honors Range, HEAD and conditional requests. A zero modtime
	// suppresses Last-Modified, since staging rewrites can change mtime
	// while content stays hash-identical.
	http.ServeContent(w, r, filepath.Base(path), time.Time{}, file)
}

// handleThumbnail serves the cached WebP thumbnail for mediaId=, with
// immutable caching since thumbnails are content-addressed by file hash.
func (s *Server) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	allowCORS(w)

	if s.opts.Thumbnails == nil || s.opts.Resolver == nil {
		http.Error(w, "thumbnails not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("mediaId"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid mediaId", http.StatusBadRequest)
		return
	}
	item, err := s.opts.Resolver.ByID(r.Context(), id)
	if err != nil {
		lookupError(w, err)
		return
	}
	data, err := s.opts.Thumbnails.Get(r.Context(), *item)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		http.Error(w, "thumbnail unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", cacheImmutable)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

// handleHealth reports server status and the active sandbox root.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if root := s.root(); root != "" {
		_, _ = fmt.Fprintf(w, `{"status":"ok","drive_root":%q,"uptime_seconds":%d}`+"\n",
			root, int64(time.Since(s.started).Seconds()))
		return
	}
	_, _ = fmt.Fprintf(w, `{"status":"ok","drive_root":null,"uptime_seconds":%d}`+"\n",
		int64(time.Since(s.started).Seconds()))
}

// allowCORS lets any local origin (embedded players, web views) read
// responses.
func allowCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// lookupError maps resolver failures to status codes.
func lookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "not in library", http.StatusNotFound)
		return
	}
	http.Error(w, "library lookup failed", http.StatusInternalServerError)
}

// resolveError maps sandbox failures to status codes.
func resolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOutsideRoot):
		http.Error(w, "forbidden path", http.StatusForbidden)
	case errors.Is(err, ErrMissing):
		http.Error(w, "file not found on drive", http.StatusNotFound)
	default:
		http.Error(w, "path resolution failed", http.StatusInternalServerError)
	}
}
