package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"painless/internal/domain"
)

// MediaInserter defines the storage interface for registering scanned media items.
type MediaInserter interface {
	InsertBatch(ctx context.Context, items []domain.MediaItem) error
	GetByHash(ctx context.Context, hash string) (*domain.MediaItem, error)
}

// AlbumResolver groups scanned media into albums named after the directory
// they were found in: an existing album name is reused (merged into), a new
// one is created.
type AlbumResolver interface {
	GetByName(ctx context.Context, name string) (*domain.Album, error)
	Create(ctx context.Context, input domain.CreateAlbumInput) (*domain.Album, error)
}

// Config configures the concurrent scanning pipeline.
type Config struct {
	DriveRoot   string
	TargetDir   string
	WorkerCount int
	BatchSize   int
	// Ignore holds custom directory-name patterns from the Discover screen.
	Ignore []string
	// Albums, when set, assigns every scanned item to an album named after
	// its immediate parent directory, creating that album on first use.
	Albums AlbumResolver
}

// Progress tracks real-time progress and discovery statistics during scanning.
type Progress struct {
	FilesScanned int    `json:"files_scanned"`
	MediaFound   int    `json:"media_found"`
	Duplicates   int    `json:"duplicates"`
	BytesScanned int64  `json:"bytes_scanned"`
	CurrentPath  string `json:"current_path"`
	IsPaused     bool   `json:"is_paused"`
	IsRunning    bool   `json:"is_running"`
}

// Scanner coordinates concurrent directory walking, metadata extraction, and database persistence.
type Scanner struct {
	mu         sync.RWMutex
	pauseCond  *sync.Cond
	config     Config
	inserter   MediaInserter
	progress   Progress
	progressCb func(Progress)
	cbMu       sync.Mutex
	paused     bool
	running    bool
	cancelFn   context.CancelFunc

	seenMu     sync.Mutex
	seenHashes map[string]bool

	albumMu  sync.Mutex
	albumIDs map[string]*int64
}

// New creates a new Scanner instance.
func New(cfg Config, inserter MediaInserter, onProgress func(Progress)) *Scanner {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = runtime.NumCPU()
		if cfg.WorkerCount < 2 {
			cfg.WorkerCount = 2
		}
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 200
	}

	s := &Scanner{
		config:     cfg,
		inserter:   inserter,
		progressCb: onProgress,
		seenHashes: make(map[string]bool),
		albumIDs:   make(map[string]*int64),
	}
	s.pauseCond = sync.NewCond(&s.mu)
	return s
}

// Status returns a snapshot of current progress.
func (s *Scanner) Status() Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress
}

// Pause temporarily suspends file processing without terminating the scan.
func (s *Scanner) Pause() {
	s.mu.Lock()
	if s.running && !s.paused {
		s.paused = true
		s.progress.IsPaused = true
	}
	s.mu.Unlock()
	s.notifyProgress()
}

// Resume continues a previously paused scan.
func (s *Scanner) Resume() {
	s.mu.Lock()
	if s.running && s.paused {
		s.paused = false
		s.progress.IsPaused = false
		s.pauseCond.Broadcast()
	}
	s.mu.Unlock()
	s.notifyProgress()
}

// Stop terminates the current scan operation.
func (s *Scanner) Stop() {
	s.mu.Lock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
	if s.paused {
		s.paused = false
		s.pauseCond.Broadcast()
	}
	s.mu.Unlock()
}

func (s *Scanner) checkPause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.paused {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.pauseCond.Wait()
	}
	return ctx.Err()
}

func (s *Scanner) notifyProgress() {
	s.mu.RLock()
	p := s.progress
	cb := s.progressCb
	s.mu.RUnlock()

	if cb != nil {
		s.cbMu.Lock()
		cb(p)
		s.cbMu.Unlock()
	}
}

func (s *Scanner) updateProgress(fn func(*Progress)) {
	s.mu.Lock()
	fn(&s.progress)
	p := s.progress
	cb := s.progressCb
	s.mu.Unlock()

	if cb != nil {
		s.cbMu.Lock()
		cb(p)
		s.cbMu.Unlock()
	}
}

// Scan runs the recursive directory walk and worker pipeline.
func (s *Scanner) Scan(parentCtx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scanner is already running")
	}
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	s.cancelFn = cancel
	s.running = true
	s.paused = false
	s.progress = Progress{IsRunning: true}
	s.mu.Unlock()

	pauseWaitDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.pauseCond.Broadcast()
			s.mu.Unlock()
		case <-pauseWaitDone:
		}
	}()
	defer close(pauseWaitDone)

	s.seenMu.Lock()
	s.seenHashes = make(map[string]bool)
	s.seenMu.Unlock()

	s.albumMu.Lock()
	s.albumIDs = make(map[string]*int64)
	s.albumMu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.paused = false
		s.progress.IsRunning = false
		s.progress.IsPaused = false
		s.mu.Unlock()
		s.notifyProgress()
	}()

	scanRoot := s.config.DriveRoot
	if s.config.TargetDir != "" {
		scanRoot = s.config.TargetDir
	}

	jobs := make(chan string, s.config.WorkerCount*16)
	results := make(chan domain.MediaItem, s.config.BatchSize*2)
	var writerErr error

	var wgWorkers sync.WaitGroup
	var wgWriter sync.WaitGroup

	// 1. Start single DB batch writer goroutine
	wgWriter.Add(1)
	go func() {
		defer wgWriter.Done()
		batch := make([]domain.MediaItem, 0, s.config.BatchSize)
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		flush := func() {
			if len(batch) == 0 || writerErr != nil || ctx.Err() != nil {
				return
			}
			if s.inserter != nil {
				if err := s.inserter.InsertBatch(ctx, batch); err != nil {
					writerErr = fmt.Errorf("failed writing batch of %d media items to DB: %w", len(batch), err)
					s.Stop()
				}
			}
			batch = batch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				// Keep draining until workers exit so no result sender can remain
				// blocked, whether cancellation came from a write failure or caller.
				for range results {
				}
				return
			case item, ok := <-results:
				if !ok {
					flush()
					return
				}
				batch = append(batch, item)
				if len(batch) >= s.config.BatchSize {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()

	// 2. Start worker pool
	for i := 0; i < s.config.WorkerCount; i++ {
		wgWorkers.Add(1)
		go func() {
			defer wgWorkers.Done()
			for {
				if err := s.checkPause(ctx); err != nil {
					return
				}

				select {
				case <-ctx.Done():
					return
				case filePath, ok := <-jobs:
					if !ok {
						return
					}
					s.processFile(ctx, filePath, results)
				}
			}
		}()
	}

	// 3. Walk directory in caller goroutine
	walkErr := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == scanRoot {
				return err
			}
			return nil // Skip unreadable descendants and continue.
		}

		if err := s.checkPause(ctx); err != nil {
			return err
		}

		if d.IsDir() {
			if path != scanRoot && ShouldIgnoreDirWith(d.Name(), s.config.Ignore) {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.Type().IsRegular() || ShouldIgnoreFile(d.Name()) {
			return nil
		}

		if !IsSupportedExtension(d.Name()) {
			s.updateProgress(func(p *Progress) {
				p.FilesScanned++
			})
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- path:
		}

		return nil
	})

	close(jobs)
	wgWorkers.Wait()
	close(results)
	wgWriter.Wait()

	if writerErr != nil {
		return writerErr
	}

	if err := parentCtx.Err(); err != nil {
		return err
	}
	if walkErr != nil {
		return walkErr
	}

	return nil
}

func (s *Scanner) checkDuplicate(ctx context.Context, hash string) bool {
	s.seenMu.Lock()
	if s.seenHashes[hash] {
		s.seenMu.Unlock()
		return true
	}
	s.seenHashes[hash] = true
	s.seenMu.Unlock()

	if s.inserter != nil {
		existing, err := s.inserter.GetByHash(ctx, hash)
		if err == nil && existing != nil {
			return true
		}
	}

	return false
}

// albumForDir returns the ID of the album named after a directory, creating
// it on first use and caching the result for the scan session. A nil ID means
// the item stays unassigned; album grouping never fails the scan.
func (s *Scanner) albumForDir(ctx context.Context, name string) *int64 {
	if s.config.Albums == nil || !domain.ValidateAlbumName(name) {
		return nil
	}

	s.albumMu.Lock()
	defer s.albumMu.Unlock()
	if id, ok := s.albumIDs[name]; ok {
		return id
	}

	album, err := s.config.Albums.GetByName(ctx, name)
	if err == nil && album == nil {
		album, err = s.config.Albums.Create(ctx, domain.CreateAlbumInput{Name: name})
		if err != nil {
			// Lost a creation race or hit a constraint; the existing album
			// still groups the item.
			album, err = s.config.Albums.GetByName(ctx, name)
		}
	}
	if err != nil || album == nil {
		return nil
	}

	id := album.ID
	s.albumIDs[name] = &id
	return &id
}

func (s *Scanner) processFile(ctx context.Context, filePath string, results chan<- domain.MediaItem) {
	mimeType, isMedia, err := SniffMediaType(filePath)
	if err != nil || !isMedia {
		s.updateProgress(func(p *Progress) {
			p.FilesScanned++
		})
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		s.updateProgress(func(p *Progress) {
			p.FilesScanned++
		})
		return
	}

	hash, err := ComputeFullSHA256(filePath)
	if err != nil {
		s.updateProgress(func(p *Progress) {
			p.FilesScanned++
		})
		return
	}

	// Check duplicates across in-memory scan session and database
	if s.checkDuplicate(ctx, hash) {
		s.updateProgress(func(p *Progress) {
			p.FilesScanned++
			p.Duplicates++
			p.CurrentPath = filePath
		})
		return
	}

	_, durationSecs, metaJSON := ExtractMetadata(filePath, mimeType)

	relPath, err := filepath.Rel(s.config.DriveRoot, filePath)
	if err != nil {
		relPath = filePath
	}
	relPathSlash := filepath.ToSlash(relPath)

	item := domain.MediaItem{
		FileHash:             hash,
		OriginalRelativePath: relPathSlash,
		CurrentRelativePath:  relPathSlash,
		FileSize:             info.Size(),
		MimeType:             mimeType,
		DurationSeconds:      durationSecs,
		MetadataJSON:         metaJSON,
		CreatedAt:            info.ModTime(),
	}

	// Group by the folder the file was found in, relative to the scan root:
	// files directly in the root itself stay unassigned.
	scanRoot := s.config.DriveRoot
	if s.config.TargetDir != "" {
		scanRoot = s.config.TargetDir
	}
	if relToScan, relErr := filepath.Rel(scanRoot, filePath); relErr == nil {
		if dir := path.Dir(filepath.ToSlash(relToScan)); dir != "." && dir != "/" {
			item.AlbumID = s.albumForDir(ctx, path.Base(dir))
		}
	}

	s.updateProgress(func(p *Progress) {
		p.FilesScanned++
		p.MediaFound++
		p.BytesScanned += info.Size()
		p.CurrentPath = filePath
	})

	select {
	case <-ctx.Done():
	case results <- item:
	}
}
