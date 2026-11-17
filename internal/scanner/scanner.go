package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
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

// Config configures the concurrent scanning pipeline.
type Config struct {
	DriveRoot   string
	TargetDir   string
	WorkerCount int
	BatchSize   int
	// Ignore holds custom directory-name patterns from the Discover screen.
	Ignore []string
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

	s.seenMu.Lock()
	s.seenHashes = make(map[string]bool)
	s.seenMu.Unlock()

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
	errChan := make(chan error, 1)

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
			if len(batch) == 0 {
				return
			}
			if s.inserter != nil {
				if err := s.inserter.InsertBatch(ctx, batch); err != nil {
					select {
					case errChan <- fmt.Errorf("failed writing batch to DB: %w", err):
					default:
					}
				}
			}
			batch = batch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				flush()
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

	select {
	case err := <-errChan:
		return err
	default:
	}

	if walkErr != nil && walkErr != context.Canceled {
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
