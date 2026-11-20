package mobile

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strconv"

	"painless/internal/server/stream"
)

// moveMediaRequest is the PATCH /api/media/{id} body (D-Stream
// compatibility): move the file into an album, or null to unassign it
// back to Unsorted Media.
type moveMediaRequest struct {
	AlbumID *int64 `json:"albumId"`
}

// handleMoveMedia moves a media item into an album. The catalog record
// is updated only, matching the desktop's virtual album assignment;
// physical relocation stays a staging operation.
func (s *Server) handleMoveMedia(w http.ResponseWriter, r *http.Request) {
	if s.opts.Media == nil || s.opts.Albums == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	id, ok := mediaID(w, r)
	if !ok {
		return
	}
	var req moveMediaRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	ctx := r.Context()
	if req.AlbumID == nil {
		if _, err := s.opts.Media.RemoveFromAlbum(ctx, []int64{id}); err != nil {
			writeErr(w, http.StatusInternalServerError, "media unassign failed")
			return
		}
	} else {
		album, err := s.opts.Albums.GetByID(ctx, *req.AlbumID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "album lookup failed")
			return
		}
		if album == nil {
			writeErr(w, http.StatusNotFound, "album not found")
			return
		}
		if _, err := s.opts.Media.MoveToAlbum(ctx, *req.AlbumID, []int64{id}); err != nil {
			writeErr(w, http.StatusInternalServerError, "media move failed")
			return
		}
	}

	item, err := s.opts.Media.GetByID(ctx, id)
	if err != nil || item == nil {
		writeErr(w, http.StatusNotFound, "media item not found")
		return
	}
	_, _ = item.ParseMetadata()
	writeJSON(w, http.StatusOK, item)
}

// handleDeleteMedia removes a media item's record and its physical file
// (D-Stream compatibility). A file already missing from the drive still
// yields a successful record deletion; only paths inside the drive
// sandbox are touched.
func (s *Server) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	if s.opts.Media == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	id, ok := mediaID(w, r)
	if !ok {
		return
	}
	item, err := s.opts.Media.GetByID(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "media lookup failed")
		return
	}
	if item == nil {
		writeErr(w, http.StatusNotFound, "media item not found")
		return
	}

	if rel := item.CurrentRelativePath; rel != "" {
		if root := s.root(); root != "" {
			if path, perr := stream.ResolvePath(root, rel); perr == nil {
				if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
					writeErr(w, http.StatusInternalServerError, "file deletion failed")
					return
				}
			}
		}
	}

	if _, err := s.opts.Media.DeleteBatch(r.Context(), []int64{id}); err != nil {
		writeErr(w, http.StatusInternalServerError, "media deletion failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// mediaID parses the {id} path value shared by the media routes.
func mediaID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid media id")
		return 0, false
	}
	return id, true
}
