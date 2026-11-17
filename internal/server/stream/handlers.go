package stream

import (
	"context"
	"errors"

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
	root := ""
	if s.DriveRoot != nil {
		root = s.DriveRoot()
	}
	if root == "" || s.Resolver == nil {
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
	item, err := s.Resolver.GetByPath(r.Context(), relPath)
	if err != nil {
		http.Error(w, "library lookup failed", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not in library", http.StatusNotFound)
		return
	}

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
	if s.Thumbnails == nil || s.Resolver == nil || s.DriveRoot == nil || s.DriveRoot() == "" {
		http.Error(w, "thumbnails not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("mediaId"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid mediaId", http.StatusBadRequest)
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
