// Package thumbnail generates, caches and throttles media previews.
package thumbnail

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"painless/internal/domain"
	"painless/internal/state"
)

var (
	ErrNoHash          = errors.New("media item has no file hash")
	ErrUnsupportedType = errors.New("unsupported media type for thumbnail")
)

// Options configures a Service. Zero values get sensible defaults.
type Options struct {
	DriveRoot      string
	Store          *state.Store // gates Get when the user disables thumbnails
	FFmpeg         string
	MaxDimension   int
	Quality        int
	Seek           time.Duration
	MemoryCapacity int
	Concurrency    int
}

// Service resolves thumbnails from memory, then the drive cache, then by
// generating a WebP preview. It is safe for concurrent use.
type Service struct {
	opts     Options
	disk     *diskCache
	mem      *memoryCache
	throttle *Throttler
	webp     *webpPool
	sem      chan struct{}
}

// New creates a thumbnail service for one drive root.
func New(opts Options) *Service {
	if opts.MaxDimension <= 0 {
		opts.MaxDimension = 320
	}
	if opts.Quality <= 0 {
		opts.Quality = 80
	}
	if opts.Seek <= 0 {
		opts.Seek = time.Second
	}
	if opts.MemoryCapacity <= 0 {
		opts.MemoryCapacity = 256
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 2
	}
	if opts.FFmpeg == "" {
		opts.FFmpeg = "ffmpeg"
	}
	throttle := NewThrottler()
	return &Service{
		opts:     opts,
		disk:     newDiskCache(opts.DriveRoot),
		mem:      newMemoryCache(opts.MemoryCapacity),
		throttle: throttle,
		webp:     newWebPPool(opts.Concurrency, opts.FFmpeg, opts.Quality, throttle),
		sem:      make(chan struct{}, opts.Concurrency),
	}
}

// Close stops the persistent WebP encoders. Call it when the service is no
// longer needed.
func (s *Service) Close() {
	s.webp.close()
}

// Throttler returns the process gate used by the UI to pause generation
// while a video plays.
func (s *Service) Throttler() *Throttler { return s.throttle }

// CacheRoot is the on-disk thumbnail directory for the drive.
func (s *Service) CacheRoot() string { return s.disk.root }

// Get returns WebP bytes for item, generating and caching them if needed.
func (s *Service) Get(ctx context.Context, item domain.MediaItem) ([]byte, error) {
	if s.opts.Store != nil && !s.opts.Store.ThumbnailsEnabled() {
		return nil, nil
	}
	if item.FileHash == "" {
		return nil, ErrNoHash
	}
	if data, ok := s.cached(item.FileHash); ok {
		return data, nil
	}

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := s.throttle.Wait(ctx); err != nil {
		return nil, err
	}
	// Another call may have filled the cache while we waited for a slot.
	if data, ok := s.cached(item.FileHash); ok {
		return data, nil
	}

	data, err := s.generate(ctx, item)
	if err != nil {
		return nil, err
	}
	if err := s.disk.put(item.FileHash, data); err != nil {
		return nil, fmt.Errorf("cache thumbnail: %w", err)
	}
	s.mem.put(item.FileHash, data)
	return data, nil
}

// GetPath is Get with the path of the cached WebP file.
func (s *Service) GetPath(ctx context.Context, item domain.MediaItem) (string, error) {
	if _, err := s.Get(ctx, item); err != nil {
		return "", err
	}
	return s.disk.path(item.FileHash), nil
}

// Invalidate drops an item from both cache levels.
func (s *Service) Invalidate(fileHash string) {
	s.mem.remove(fileHash)
	_ = s.disk.remove(fileHash)
}

func (s *Service) cached(hash string) ([]byte, bool) {
	if data, ok := s.mem.get(hash); ok {
		return data, true
	}
	if data, ok := s.disk.get(hash); ok {
		s.mem.put(hash, data)
		return data, true
	}
	return nil, false
}

func (s *Service) generate(ctx context.Context, item domain.MediaItem) ([]byte, error) {
	source, err := s.sourcePath(item)
	if err != nil {
		return nil, err
	}
	switch item.Type() {
	case domain.MediaTypeImage:
		img, err := decodeImage(source)
		if err != nil {
			return nil, err
		}
		resized := resizeImage(img, s.opts.MaxDimension)
		if data, err := s.webp.encode(ctx, resized); err == nil {
			return data, nil
		}
		// The persistent encoder may have died; a one-shot encode keeps
		// thumbnails working until the service is recreated.
		return encodeWebP(ctx, s.opts.FFmpeg, resized, s.opts.Quality, s.throttle)
	case domain.MediaTypeVideo:
		return extractVideoFrame(ctx, s.opts.FFmpeg, source, s.opts.Seek, s.opts.MaxDimension, s.opts.Quality, s.throttle)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedType, item.MimeType)
	}
}

// sourcePath resolves a media item's path inside the drive root, rejecting
// absolute or traversing paths.
func (s *Service) sourcePath(item domain.MediaItem) (string, error) {
	relative := item.CurrentRelativePath
	if relative == "" {
		relative = item.OriginalRelativePath
	}
	if relative == "" {
		return "", errors.New("media item has no file path")
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("media path must be relative: %q", relative)
	}
	root := filepath.Clean(s.opts.DriveRoot)
	path := filepath.Clean(filepath.Join(root, relative))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("media path escapes drive root: %q", relative)
	}
	return path, nil
}
