// Package discover provides the scanner control bar, live scanning progress,
// virtual staging review table, and consolidation execution modal.
package discover

import (
	"context"
	"path/filepath"
	"sync"

	mui "github.com/egoist/mygo/ui"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/fileops"
	"painless/internal/scanner"
	"painless/internal/state"
	uitheme "painless/internal/ui/theme"
)

// ScannerFactory constructs a scanner instance for a target path.
type ScannerFactory func(cfg scanner.Config, onProgress func(scanner.Progress)) Scanner

// Scanner interface controls the background scanning process.
type Scanner interface {
	Scan(ctx context.Context) error
	Pause()
	Resume()
	Stop()
	Status() scanner.Progress
}

// Stager abstracts virtual staging and consolidation execution.
type Stager interface {
	StageMove(item domain.MediaItem, targetAlbumID *int64, targetAlbumName string)
	StageUnknown(item domain.MediaItem)
	StageDelete(item domain.MediaItem)
	StageIgnore(item domain.MediaItem)
	Unstage(mediaID int64)
	ListOperations() []domain.StagedOperation
	Summary() domain.StagingSummary
	Clear()
	Execute(
		ctx context.Context,
		driveRoot string,
		mediaStore fileops.MediaStore,
		albumStore fileops.AlbumStore,
		onProgress func(current, total int, op domain.StagedOperation),
	) (*fileops.ExecutionReport, error)
}

// AlbumSource lists albums for target assignment.
type AlbumSource interface {
	List(ctx context.Context) ([]domain.AlbumSummary, error)
}

// Options configures the discover & staging screen.
type Options struct {
	Store          *state.Store
	Stager         Stager
	ScannerFactory ScannerFactory
	// AlbumScannerFactory indexes in-place library media without staging it.
	AlbumScannerFactory ScannerFactory
	AlbumSource         AlbumSource
	MediaStore          fileops.MediaStore
	AlbumStore          fileops.AlbumStore
	RefreshWindow       func()
	Update              func(func()) // Dispatches worker results to the UI thread.
	DriveRoot           string
	// CatalogCount reports how many items are indexed on the active drive.
	CatalogCount func() int
	// DirectoryPicker opens a native folder chooser. Injected in tests.
	DirectoryPicker func(defaultPath string) (string, error)
}

// Screen coordinates the scanner, staging table, and consolidation modal.
type Screen struct {
	store               *state.Store
	stager              Stager
	scannerFactory      ScannerFactory
	albumScannerFactory ScannerFactory
	albumSource         AlbumSource
	mediaStore          fileops.MediaStore
	albumStore          fileops.AlbumStore
	refreshWindow       func()
	update              func(func())
	driveRoot           string
	albumFolder         string
	ignoreRules         []string
	ignoreInput         string
	catalogCount        func() int
	directoryPicker     func(defaultPath string) (string, error)
	ctx                 context.Context
	cancel              context.CancelFunc
	jobs                sync.WaitGroup
	closed              bool

	// Scanner state
	targetPath  string
	isScanning  bool
	isPaused    bool
	activeScan  Scanner
	scanCancel  context.CancelFunc
	scanProg    scanner.Progress
	lastScanErr string

	// Staging table state
	stagedOps    []domain.StagedOperation
	summary      domain.StagingSummary
	albums       []string
	albumMap     map[string]int64
	selectedItem int64

	// Consolidate & execute modal state
	modalOpen    bool
	isExecuting  bool
	execCurrent  int
	execTotal    int
	execReport   *fileops.ExecutionReport
	execErr      string
	execFinished bool

	mu sync.RWMutex
}

// New creates an initialized discover & stager screen.
func New(opts Options) *Screen {
	stager := opts.Stager
	if stager == nil {
		stager = fileops.NewStagingEngine()
	}

	ctx, cancel := context.WithCancel(context.Background())
	albumFolder := ""
	if opts.DriveRoot != "" {
		albumFolder = filepath.Join(opts.DriveRoot, "media", "albums")
	}
	return &Screen{
		ctx:                 ctx,
		cancel:              cancel,
		update:              opts.Update,
		driveRoot:           opts.DriveRoot,
		albumFolder:         albumFolder,
		targetPath:          opts.DriveRoot,
		catalogCount:        opts.CatalogCount,
		directoryPicker:     opts.DirectoryPicker,
		store:               opts.Store,
		stager:              stager,
		scannerFactory:      opts.ScannerFactory,
		albumScannerFactory: opts.AlbumScannerFactory,
		albumSource:         opts.AlbumSource,
		mediaStore:          opts.MediaStore,
		albumStore:          opts.AlbumStore,
		refreshWindow:       opts.RefreshWindow,
		albumMap:            make(map[string]int64),
		stagedOps:           stager.ListOperations(),
		summary:             stager.Summary(),
	}
}

// View builds one frame of the discover & staging screen.
func (s *Screen) View(c *mui.Context) {
	uitheme.Apply(c)
	tk := lobetheme.TokensOf(c)

	container := mui.Column(c).Fill().Padding(20, 24).Gap(16)
	container.Children(func() {
		// 1. Active drive banner and configurable album folder
		s.driveBanner(c, tk)
		s.scaffoldBanner(c, tk)
		s.albumFolderCard(c, tk)

		// 2. Scanner controls, ignorelist and live telemetry side by side
		mui.Row(c).Shrink(0).Gap(16).AlignItems(mui.Start).Children(func() {
			mui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(16).Children(func() {
				s.scanControls(c, tk)
				s.ignoreList(c, tk)
			})
			mui.Column(c).Width(280).Shrink(0).Children(func() { s.scannerStatus(c, tk) })
		})

		// 3. Virtual staging review table
		mui.Column(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			if !s.isExecuting {
				s.stagingTable(c, tk)
			}
		})

		// 3. Consolidate & execute action banner
		s.executePanel(c, tk)

		// 4. Modal confirmation dialog
		s.confirmModal(c, tk)
	})
}

// ReloadAlbums refreshes the destination list on the UI thread.
func (s *Screen) ReloadAlbums() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.albumSource != nil {
		if summaries, err := s.albumSource.List(s.ctx); err == nil {
			s.albums = make([]string, 0, len(summaries))
			s.albumMap = make(map[string]int64, len(summaries))
			for _, sum := range summaries {
				s.albums = append(s.albums, sum.Album.Name)
				s.albumMap[sum.Album.Name] = sum.Album.ID
			}
		}
	}
}

// Close cancels jobs and prevents queued results from updating this screen.
// The returned channel closes when workers no longer need the drive services.
func (s *Screen) Close() <-chan struct{} {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	scan := s.activeScan
	s.mu.Unlock()
	if scan != nil {
		scan.Stop() // Wake a paused scanner as well as canceling its context.
	}
	done := make(chan struct{})
	go func() {
		s.jobs.Wait()
		close(done)
	}()
	return done
}

func (s *Screen) publish(apply func()) {
	update := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.closed {
			apply()
		}
	}
	if s.update != nil {
		s.update(update)
	} else {
		update()
	}
}

// QueueDiscovered adds committed scan results to virtual staging, not to disk.
func (s *Screen) QueueDiscovered(items []domain.MediaItem) {
	s.publish(func() {
		for _, item := range items {
			s.stager.StageUnknown(item)
		}
		s.stagedOps = s.stager.ListOperations()
		s.summary = s.stager.Summary()
		if len(items) > 0 && s.store != nil {
			s.store.MediaChanged("")
		}
	})
}

// StageDeletion queues catalog items for physical deletion on the active drive.
func (s *Screen) StageDeletion(items []domain.MediaItem) {
	s.publish(func() {
		for _, item := range items {
			s.stager.StageDelete(item)
		}
		s.stagedOps = s.stager.ListOperations()
		s.summary = s.stager.Summary()
	})
}

func (s *Screen) requestRedraw() {
	if s.refreshWindow != nil {
		s.refreshWindow()
	}
}

// SetTargetPath updates the configured scan path.
func (s *Screen) SetTargetPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targetPath = path
}

// TargetPath returns the configured scan path.
func (s *Screen) TargetPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.targetPath
}

// IsScanning returns true if a scanner job is currently active.
func (s *Screen) IsScanning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isScanning
}

// StagedCount returns the number of staged operations.
func (s *Screen) StagedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.stagedOps)
}

// StageItem allows adding a media item to staging.
func (s *Screen) StageItem(item domain.MediaItem, albumName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.isExecuting {
		return
	}
	if albumName != "" {
		var albumID *int64
		if id, ok := s.albumMap[albumName]; ok {
			albumID = &id
		}
		s.stager.StageMove(item, albumID, albumName)
	} else {
		s.stager.StageUnknown(item)
	}
	s.stagedOps = s.stager.ListOperations()
	s.summary = s.stager.Summary()
}

// ClearStaging clears all staged operations.
func (s *Screen) ClearStaging() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && !s.isExecuting {
		s.stager.Clear()
		s.stagedOps = nil
		s.summary = domain.StagingSummary{}
	}
}
