package mobile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"painless/internal/domain"
)

// statusResponse is the /api/status payload used for pairing screens.
type statusResponse struct {
	Status         string            `json:"status"`
	Drive          *domain.DriveInfo `json:"drive"`
	LANAddress     string            `json:"lan_address,omitempty"`
	UptimeSeconds  int64             `json:"uptime_seconds"`
	MaxUploadBytes int64             `json:"max_upload_bytes"`
}

// writeJSON serializes v with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr emits a plain-text error with the given status code.
func writeErr(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

// handlePing answers reachability checks without authentication so a
// phone can confirm the desktop is reachable before pairing.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleStatus reports server health, the active drive, and the LAN
// address for QR pairing.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := statusResponse{
		Status:         "ok",
		UptimeSeconds:  int64(time.Since(s.started).Seconds()),
		MaxUploadBytes: s.opts.MaxUploadBytes,
	}
	if s.opts.DriveInfo != nil {
		resp.Drive = s.opts.DriveInfo()
	}
	if lan, err := LANAddress(s.opts.Port); err == nil {
		resp.LANAddress = lan
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleListAlbums returns all albums with media counts.
func (s *Server) handleListAlbums(w http.ResponseWriter, r *http.Request) {
	if s.opts.Albums == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	albums, err := s.opts.Albums.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "album listing failed")
		return
	}
	if albums == nil {
		albums = []domain.AlbumSummary{}
	}
	writeJSON(w, http.StatusOK, albums)
}

// createAlbumRequest is the POST /api/albums body.
type createAlbumRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// handleCreateAlbum validates and creates a new album.
func (s *Server) handleCreateAlbum(w http.ResponseWriter, r *http.Request) {
	if s.opts.Albums == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	var req createAlbumRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !domain.ValidateAlbumName(req.Name) {
		writeErr(w, http.StatusBadRequest, "invalid album name")
		return
	}
	album, err := s.opts.Albums.Create(r.Context(), domain.CreateAlbumInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "album creation failed")
		return
	}
	writeJSON(w, http.StatusCreated, album)
}

// handleListMedia serves paginated media queries.
// Query: limit, offset, search, album_id, filter, sort_by, sort_order.
func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	if s.opts.Media == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	params, err := parseMediaListParams(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	page, err := s.opts.Media.List(r.Context(), params)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "media listing failed")
		return
	}
	if page.Items == nil {
		page.Items = []domain.MediaItem{}
	}
	writeJSON(w, http.StatusOK, page)
}

// parseMediaListParams converts query parameters into MediaListParams,
// rejecting unknown filter and sort values instead of silently ignoring
// them.
func parseMediaListParams(r *http.Request) (domain.MediaListParams, error) {
	q := r.URL.Query()
	params := domain.MediaListParams{
		Search: strings.TrimSpace(q.Get("search")),
	}

	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 || limit > 500 {
			return params, fmt.Errorf("limit must be 1-500")
		}
		params.Limit = limit
	} else {
		params.Limit = 50
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return params, fmt.Errorf("offset must be >= 0")
		}
		params.Offset = offset
	}
	if raw := q.Get("album_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return params, fmt.Errorf("album_id must be a positive integer")
		}
		params.AlbumID = &id
	}
	if raw := q.Get("filter"); raw != "" {
		switch domain.MediaFilter(raw) {
		case domain.MediaFilterAll, domain.MediaFilterImages, domain.MediaFilterVideos, domain.MediaFilterAudio:
			params.Filter = domain.MediaFilter(raw)
		default:
			return params, fmt.Errorf("unknown filter %q", raw)
		}
	}
	if raw := q.Get("sort_by"); raw != "" {
		switch domain.MediaSortBy(raw) {
		case domain.SortByDate, domain.SortByName, domain.SortBySize:
			params.SortBy = domain.MediaSortBy(raw)
		default:
			return params, fmt.Errorf("unknown sort_by %q", raw)
		}
	}
	if raw := q.Get("sort_order"); raw != "" {
		switch domain.SortOrder(raw) {
		case domain.SortOrderAsc, domain.SortOrderDesc:
			params.SortOrder = domain.SortOrder(raw)
		default:
			return params, fmt.Errorf("unknown sort_order %q", raw)
		}
	}
	return params, nil
}

// handleGetMedia returns one item with parsed metadata.
func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	if s.opts.Media == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid media id")
		return
	}
	item, err := s.opts.Media.GetByID(r.Context(), id)
	if err != nil || item == nil {
		writeErr(w, http.StatusNotFound, "media item not found")
		return
	}
	// Expose parsed metadata under the item's "metadata" JSON key when it
	// deserializes; raw MetadataJSON remains available regardless.
	_, _ = item.ParseMetadata()
	writeJSON(w, http.StatusOK, item)
}
