package stream

import (
	"context"
	"errors"
	"painless/internal/domain"

	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Thumbnails are addressed by media ID, which may be reused on another drive.
const thumbnailCache = "private, no-cache"

// handleStream serves a cataloged media file with full Range support via
// http.ServeContent. Query: path=<drive-relative path>.
func (s Routes) handleStream(w http.ResponseWriter, r *http.Request) {
	root, ok := s.sandbox(w)
	if !ok {
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
	item, err := s.Resolver.GetByPath(r.Context(), relPath)
	if err != nil {
		http.Error(w, "library lookup failed", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not in library", http.StatusNotFound)
		return
	}
	s.serveItem(w, r, root, item)
}

// handleMediaByID serves the original file for GET /api/media/{id} with
// full Range support (D-Stream companion compatibility).
func (s Routes) handleMediaByID(w http.ResponseWriter, r *http.Request) {
	root, ok := s.sandbox(w)
	if !ok {
		return
	}
	item, ok := s.itemByID(w, r)
	if !ok {
		return
	}
	s.serveItem(w, r, root, item)
}

// sandbox reports the active drive root, writing a 503 when playback is
// not configured. ok=false means the response was written.
func (s Routes) sandbox(w http.ResponseWriter) (string, bool) {
	root := ""
	if s.DriveRoot != nil {
		root = s.DriveRoot()
	}
	if root == "" || s.Resolver == nil {
		http.Error(w, "library resolver not configured", http.StatusServiceUnavailable)
		return "", false
	}
	return root, true
}

// itemByID resolves the {id} path value to a cataloged item, writing the
// error response. ok=false means the response was written.
func (s Routes) itemByID(w http.ResponseWriter, r *http.Request) (*domain.MediaItem, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return nil, false
	}
	item, err := s.Resolver.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "library lookup failed", http.StatusInternalServerError)
		return nil, false
	}
	if item == nil {
		http.Error(w, "not in library", http.StatusNotFound)
		return nil, false
	}
	return item, true
}

// serveItem streams a cataloged item's file from the drive with full
// Range support via http.ServeContent.
func (s Routes) serveItem(w http.ResponseWriter, r *http.Request, root string, item *domain.MediaItem) {
	path, err := ResolvePath(root, item.CurrentRelativePath)
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

// handleThumbnail serves the cached WebP thumbnail for mediaId=.
func (s Routes) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("mediaId"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid mediaId", http.StatusBadRequest)
		return
	}
	s.serveThumbnailID(w, r, id)
}

// handleThumbnailByID serves the cached WebP thumbnail for
// GET /api/thumbnail/{id} (D-Stream companion compatibility).
func (s Routes) handleThumbnailByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return
	}
	s.serveThumbnailID(w, r, id)
}

// serveThumbnailID resolves one media ID and writes its thumbnail bytes.
func (s Routes) serveThumbnailID(w http.ResponseWriter, r *http.Request, id int64) {
	if s.Thumbnails == nil || s.Resolver == nil || s.DriveRoot == nil || s.DriveRoot() == "" {
		http.Error(w, "thumbnails not configured", http.StatusServiceUnavailable)
		return
	}
	item, err := s.Resolver.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "library lookup failed", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not in library", http.StatusNotFound)
		return
	}
	data, err := s.Thumbnails.Get(r.Context(), *item)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		http.Error(w, "thumbnail unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", thumbnailCache)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
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
