// Package discover provides the scanner control bar, live scanning progress,
// virtual staging review table, and consolidation execution modal.
package discover

import (
	"context"
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
	AlbumSource    AlbumSource
	MediaStore     fileops.MediaStore
	AlbumStore     fileops.AlbumStore
	RefreshWindow  func()
}

// Screen coordinates the scanner, staging table, and consolidation modal.
type Screen struct {
	store          *state.Store
	stager         Stager
	scannerFactory ScannerFactory
	albumSource    AlbumSource
	mediaStore     fileops.MediaStore
	albumStore     fileops.AlbumStore
	refreshWindow  func()

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

	return &Screen{
		store:          opts.Store,
		stager:         stager,
		scannerFactory: opts.ScannerFactory,
		albumSource:    opts.AlbumSource,
		mediaStore:     opts.MediaStore,
		albumStore:     opts.AlbumStore,
		refreshWindow:  opts.RefreshWindow,
		albumMap:       make(map[string]int64),
		stagedOps:      stager.ListOperations(),
		summary:        stager.Summary(),
	}
}

// View builds one frame of the discover & staging screen.
func (s *Screen) View(c *mui.Context) {
	uitheme.Apply(c)
	tk := lobetheme.TokensOf(c)

	s.refreshStagedData()

	container := mui.Column(c).Fill().Padding(20, 24).Gap(16)
	container.Children(func() {
		// 1. Scanner controls & live progress bar
		s.scanControls(c, tk)

		// 2. Virtual staging review table
		mui.Column(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			s.stagingTable(c, tk)
		})

		// 3. Consolidate & execute action banner
		s.executePanel(c, tk)

		// 4. Modal confirmation dialog
		s.confirmModal(c, tk)
	})
}

func (s *Screen) refreshStagedData() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stager != nil {
		s.stagedOps = s.stager.ListOperations()
		s.summary = s.stager.Summary()
	}

	// Load albums if empty and albumSource present
	if len(s.albums) == 0 && s.albumSource != nil {
		if summaries, err := s.albumSource.List(context.Background()); err == nil {
			s.albums = make([]string, 0, len(summaries))
			s.albumMap = make(map[string]int64, len(summaries))
			for _, sum := range summaries {
				s.albums = append(s.albums, sum.Album.Name)
				s.albumMap[sum.Album.Name] = sum.Album.ID
			}
		}
	}
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
	if s.stager == nil {
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
	if s.stager != nil {
		s.stager.Clear()
		s.stagedOps = nil
		s.summary = domain.StagingSummary{}
	}
}
