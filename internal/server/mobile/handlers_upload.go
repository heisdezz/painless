package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"painless/internal/domain"
	"painless/internal/fileops"
	"painless/internal/scanner"
)

// warmTimeout bounds background thumbnail generation for uploads.
const warmTimeout = 2 * time.Minute

// uploadResponse is the POST /api/upload result payload.
type uploadResponse struct {
	Duplicate bool              `json:"duplicate"`
	Item      *domain.MediaItem `json:"item"`
}

// handleUpload accepts one media file as a streaming multipart part or a
// raw body, hashing and storing it on the external drive without
// buffering the payload in memory, then catalogs it in SQLite.
//
// Multipart mode: Content-Type multipart/form-data, first file part.
// Raw mode: any other content type with X-File-Name header.
// Both modes honor optional X-Album-ID to target an album directory.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.opts.Media == nil || s.opts.Albums == nil {
		writeErr(w, http.StatusServiceUnavailable, "library not configured")
		return
	}
	root := s.root()
	if root == "" {
		writeErr(w, http.StatusServiceUnavailable, "no active drive")
		return
	}

	albumID, album, ok := s.resolveUploadAlbum(w, r)
	if !ok {
		return
	}

	// Cap the whole request body just above the per-file limit to leave
	// room for multipart framing overhead.
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxUploadBytes+64<<10)

	src, filename, cleanup, err := s.openUploadSource(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	saneName := fileops.SanitizeFilename(filename)
	if saneName == "" {
		writeErr(w, http.StatusBadRequest, "invalid file name")
		return
	}

	item, duplicate, err := s.persistUpload(r, root, src, saneName, albumID, album)
	if err != nil {
		writeErr(w, uploadErrorStatus(err), err.Error())
		return
	}
	if duplicate {
		writeJSON(w, http.StatusOK, uploadResponse{Duplicate: true, Item: item})
		return
	}

	s.warmThumbnail(item)
	writeJSON(w, http.StatusCreated, uploadResponse{Item: item})
}

// resolveUploadAlbum validates the optional X-Album-ID header, returning
// the parsed ID and album; ok=false means the response was written.
func (s *Server) resolveUploadAlbum(w http.ResponseWriter, r *http.Request) (*int64, *domain.Album, bool) {
	raw := strings.TrimSpace(r.Header.Get("X-Album-ID"))
	if raw == "" {
		return nil, nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "X-Album-ID must be a positive integer")
		return nil, nil, false
	}
	album, err := s.opts.Albums.GetByID(r.Context(), id)
	if err != nil || album == nil {
		writeErr(w, http.StatusNotFound, "album not found")
		return nil, nil, false
	}
	return &id, album, true
}

// openUploadSource yields the payload reader and its filename. For
// multipart requests the first file part is used; otherwise the raw body
// with the X-File-Name header.
func (s *Server) openUploadSource(r *http.Request) (src io.Reader, filename string, cleanup func(), err error) {
	ct := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(ct)

	if strings.HasPrefix(mediaType, "multipart/") {
		mr, mErr := r.MultipartReader()
		if mErr != nil {
			return nil, "", nil, errors.New("invalid multipart body")
		}
		for {
			part, pErr := mr.NextPart()
			if errors.Is(pErr, io.EOF) {
				return nil, "", nil, errors.New("multipart body contains no file part")
			}
			if pErr != nil {
				return nil, "", nil, errors.New("invalid multipart body")
			}
			if part.FileName() != "" {
				name := strings.TrimSpace(r.Header.Get("X-File-Name"))
				if name == "" {
					name = part.FileName()
				}
				return part, name, func() { _ = part.Close() }, nil
			}
			_ = part.Close()
		}
	}

	name := strings.TrimSpace(r.Header.Get("X-File-Name"))
	if name == "" {
		return nil, "", nil, errors.New("X-File-Name header required for raw uploads")
	}
	return r.Body, name, nil, nil
}

// uploadError is a handler-level failure carrying its HTTP status.
type uploadError struct {
	status int
	msg    string
}

func (e *uploadError) Error() string { return e.msg }

func uploadErrorStatus(err error) int {
	var ue *uploadError
	if errors.As(err, &ue) {
		return ue.status
	}
	return http.StatusInternalServerError
}

// persistUpload streams src to a temp file on the drive while hashing,
// then moves it to its destination and catalogs it. duplicate reports a
// byte-identical copy already present in the library.
func (s *Server) persistUpload(r *http.Request, root string, src io.Reader, filename string, albumID *int64, album *domain.Album) (*domain.MediaItem, bool, error) {
	tmp, err := createUploadTemp(root)
	if err != nil {
		return nil, false, err
	}
	tmpName := tmp.Name()
	removeTmp := func() { _ = os.Remove(tmpName) }

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(src, s.opts.MaxUploadBytes+1))
	if err != nil {
		_ = tmp.Close()
		removeTmp()
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, false, &uploadError{http.StatusRequestEntityTooLarge, "upload exceeds size limit"}
		}
		return nil, false, &uploadError{http.StatusBadRequest, "upload transfer failed"}
	}
	if size > s.opts.MaxUploadBytes {
		_ = tmp.Close()
		removeTmp()
		return nil, false, &uploadError{http.StatusRequestEntityTooLarge, "upload exceeds size limit"}
	}
	if size == 0 {
		_ = tmp.Close()
		removeTmp()
		return nil, false, &uploadError{http.StatusBadRequest, "empty upload"}
	}
	if err := tmp.Close(); err != nil {
		removeTmp()
		return nil, false, err
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	mimeType := detectUploadMime(tmpName, filename)

	ctx := r.Context()

	// Library dedupe: identical content already cataloged.
	if existing, gErr := s.opts.Media.GetByHash(ctx, hash); gErr == nil && existing != nil {
		removeTmp()
		return existing, true, nil
	}

	// Destination directory: album dir or unknown/.
	destDir := fileops.UnknownDir(root)
	if albumID != nil && album != nil {
		destDir, err = fileops.EnsureAlbumDir(root, album.Name)
		if err != nil {
			removeTmp()
			return nil, false, err
		}
	} else if err := os.MkdirAll(destDir, 0o755); err != nil {
		removeTmp()
		return nil, false, err
	}

	finalPath, diskDup, err := fileops.ResolveCollision(filepath.Join(destDir, filename), hash)
	if err != nil {
		removeTmp()
		return nil, false, err
	}
	if diskDup {
		removeTmp()
	} else if err := fileops.MoveFile(tmpName, finalPath); err != nil {
		removeTmp()
		return nil, false, err
	}

	rel, err := filepath.Rel(root, finalPath)
	if err != nil {
		return nil, false, err
	}

	item := domain.MediaItem{
		FileHash:             hash,
		OriginalRelativePath: rel,
		CurrentRelativePath:  rel,
		FileSize:             size,
		MimeType:             mimeType,
		AlbumID:              albumID,
	}
	if _, err := s.opts.Media.InsertBatch(ctx, []domain.MediaItem{item}); err != nil {
		return nil, false, err
	}
	stored, err := s.opts.Media.GetByHash(ctx, hash)
	if err != nil || stored == nil {
		stored = &item
	}
	return stored, false, nil
}

// createUploadTemp creates a hidden temp file directly on the drive so
// the final move stays within one filesystem.
func createUploadTemp(root string) (*os.File, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return os.CreateTemp(root, ".upload_*.tmp")
}

// detectUploadMime sniffs magic bytes of the staged file, falling back to
// the filename extension and finally a generic type.
func detectUploadMime(stagedPath, filename string) string {
	if m, ok, _ := scanner.SniffMediaType(stagedPath); ok && m != "" {
		return m
	}
	if m := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); m != "" {
		return m
	}
	return "application/octet-stream"
}

// warmThumbnail schedules background thumbnail generation for a newly
// cataloged item; failures are non-fatal and retried on next view.
func (s *Server) warmThumbnail(item *domain.MediaItem) {
	if s.opts.Thumbnails == nil || item == nil {
		return
	}
	warm := *item
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), warmTimeout)
		defer cancel()
		_, _ = s.opts.Thumbnails.Get(ctx, warm)
	}()
}
