package fileops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"painless/internal/domain"
)

// MediaStore defines persistence operations required during staging execution.
type MediaStore interface {
	UpdatePath(ctx context.Context, id int64, newPath string) error
	MoveToAlbum(ctx context.Context, targetAlbumID int64, mediaIDs []int64) (int64, error)
	RemoveFromAlbum(ctx context.Context, mediaIDs []int64) (int64, error)
	DeleteBatch(ctx context.Context, mediaIDs []int64) (int64, error)
}

// AlbumStore provides album metadata lookups and name-based creation so
// name-only staged moves can merge into an existing album or create one.
type AlbumStore interface {
	GetByID(ctx context.Context, id int64) (*domain.Album, error)
	GetByName(ctx context.Context, name string) (*domain.Album, error)
	Create(ctx context.Context, input domain.CreateAlbumInput) (*domain.Album, error)
}

// JournalEntry tracks a physically executed move for rollback safety.
type JournalEntry struct {
	MediaID         int64
	OldSourcePath   string
	NewDestPath     string
	OldRelativePath string
	NewRelativePath string
	TargetAlbumID   *int64
	Action          domain.StageAction
	ExecutedAt      time.Time
}

// ExecutionReport details the outcome of a "Consolidate & Execute" run.
type ExecutionReport struct {
	TotalOperations int      `json:"total_operations"`
	MovedCount      int      `json:"moved_count"`
	DeletedCount    int      `json:"deleted_count"`
	IgnoredCount    int      `json:"ignored_count"`
	FailedCount     int      `json:"failed_count"`
	Errors          []string `json:"errors,omitempty"`
}

// StagingEngine manages the virtual staging of file movements and their transactional execution.
type StagingEngine struct {
	mu         sync.RWMutex
	operations map[int64]*domain.StagedOperation
	journal    []JournalEntry
}

// NewStagingEngine creates an empty staging engine.
func NewStagingEngine() *StagingEngine {
	return &StagingEngine{
		operations: make(map[int64]*domain.StagedOperation),
		journal:    make([]JournalEntry, 0),
	}
}

// StageMove stages a media item to be relocated to a target album.
func (e *StagingEngine) StageMove(item domain.MediaItem, targetAlbumID *int64, targetAlbumName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.operations[item.ID] = &domain.StagedOperation{
		ID:              fmt.Sprintf("stage-%d-%d", item.ID, time.Now().UnixNano()),
		MediaID:         item.ID,
		FileHash:        item.FileHash,
		SourcePath:      item.CurrentRelativePath,
		TargetAlbumID:   targetAlbumID,
		TargetAlbumName: targetAlbumName,
		Action:          domain.StageActionMove,
		Status:          domain.StageStatusPending,
		CreatedAt:       time.Now(),
	}
}

// StageUnknown stages a media item to be moved to the fallback unknown/ directory.
func (e *StagingEngine) StageUnknown(item domain.MediaItem) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.operations[item.ID] = &domain.StagedOperation{
		ID:            fmt.Sprintf("stage-%d-%d", item.ID, time.Now().UnixNano()),
		MediaID:       item.ID,
		FileHash:      item.FileHash,
		SourcePath:    item.CurrentRelativePath,
		TargetAlbumID: nil,
		Action:        domain.StageActionMove,
		Status:        domain.StageStatusPending,
		CreatedAt:     time.Now(),
	}
}

// StageDelete stages a media item for disk deletion and DB removal.
func (e *StagingEngine) StageDelete(item domain.MediaItem) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.operations[item.ID] = &domain.StagedOperation{
		ID:         fmt.Sprintf("stage-%d-%d", item.ID, time.Now().UnixNano()),
		MediaID:    item.ID,
		FileHash:   item.FileHash,
		SourcePath: item.CurrentRelativePath,
		Action:     domain.StageActionDelete,
		Status:     domain.StageStatusPending,
		CreatedAt:  time.Now(),
	}
}

// StageIgnore marks a media item to remain untouched during consolidation.
func (e *StagingEngine) StageIgnore(item domain.MediaItem) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.operations[item.ID] = &domain.StagedOperation{
		ID:         fmt.Sprintf("stage-%d-%d", item.ID, time.Now().UnixNano()),
		MediaID:    item.ID,
		FileHash:   item.FileHash,
		SourcePath: item.CurrentRelativePath,
		Action:     domain.StageActionIgnore,
		Status:     domain.StageStatusPending,
		CreatedAt:  time.Now(),
	}
}

// Unstage removes a media item from staging.
func (e *StagingEngine) Unstage(mediaID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.operations, mediaID)
}

// GetOperation returns the staged operation for a media ID if staged.
func (e *StagingEngine) GetOperation(mediaID int64) (*domain.StagedOperation, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	op, found := e.operations[mediaID]
	if !found {
		return nil, false
	}
	cp := *op
	return &cp, true
}

// ListOperations returns all currently staged operations.
func (e *StagingEngine) ListOperations() []domain.StagedOperation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ops := make([]domain.StagedOperation, 0, len(e.operations))
	for _, op := range e.operations {
		ops = append(ops, *op)
	}
	return ops
}

// Summary calculates a breakdown of all planned operations.
func (e *StagingEngine) Summary() domain.StagingSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()

	summary := domain.StagingSummary{
		TotalOperations: len(e.operations),
		AlbumMoveCount:  make(map[string]int),
	}

	for _, op := range e.operations {
		switch op.Action {
		case domain.StageActionMove:
			if op.TargetAlbumName != "" {
				summary.AlbumMoveCount[op.TargetAlbumName]++
			} else {
				summary.UnknownCount++
			}
		case domain.StageActionDelete:
			summary.DeleteCount++
		case domain.StageActionIgnore:
			summary.IgnoreCount++
		}
	}

	return summary
}

// Clear resets the staging engine.
func (e *StagingEngine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.operations = make(map[int64]*domain.StagedOperation)
	e.journal = make([]JournalEntry, 0)
}

// Execute performs the physical moves, name collision resolutions, and database updates.
func (e *StagingEngine) Execute(
	ctx context.Context,
	driveRoot string,
	mediaStore MediaStore,
	albumStore AlbumStore,
	onProgress func(current, total int, op domain.StagedOperation),
) (*ExecutionReport, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ScaffoldLibrary(driveRoot); err != nil {
		return nil, fmt.Errorf("scaffolding drive failed: %w", err)
	}

	report := &ExecutionReport{
		TotalOperations: len(e.operations),
	}

	opsList := make([]*domain.StagedOperation, 0, len(e.operations))
	for _, op := range e.operations {
		opsList = append(opsList, op)
	}

	for idx, op := range opsList {
		if err := ctx.Err(); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("execution canceled: %v", err))
			return report, err
		}

		op.Status = domain.StageStatusExecuting

		switch op.Action {
		case domain.StageActionIgnore:
			op.Status = domain.StageStatusDone
			report.IgnoredCount++

		case domain.StageActionDelete:
			srcAbs := resolveAbsPath(driveRoot, op.SourcePath)
			if err := os.Remove(srcAbs); err != nil && !os.IsNotExist(err) {
				op.Status = domain.StageStatusFailed
				op.ErrorMessage = err.Error()
				report.FailedCount++
				report.Errors = append(report.Errors, fmt.Sprintf("failed deleting %s: %v", srcAbs, err))
			} else {
				if mediaStore != nil {
					_, _ = mediaStore.DeleteBatch(ctx, []int64{op.MediaID})
				}
				op.Status = domain.StageStatusDone
				now := time.Now()
				op.ExecutedAt = &now
				report.DeletedCount++
			}

		case domain.StageActionMove:
			var targetDirName string
			albumName := op.TargetAlbumName

			if albumName == "" && op.TargetAlbumID != nil && albumStore != nil {
				if album, err := albumStore.GetByID(ctx, *op.TargetAlbumID); err == nil && album != nil {
					albumName = album.Name
				}
			}

			// Name-only destinations resolve against the library so the
			// media lands in an existing album (merged) or a fresh one.
			if albumName != "" && op.TargetAlbumID == nil && albumStore != nil {
				if album, err := albumStore.GetByName(ctx, albumName); err == nil && album != nil {
					op.TargetAlbumID = &album.ID
				} else if created, cErr := albumStore.Create(ctx, domain.CreateAlbumInput{Name: albumName}); cErr == nil {
					op.TargetAlbumID = &created.ID
				} else if album, err := albumStore.GetByName(ctx, albumName); err == nil && album != nil {
					op.TargetAlbumID = &album.ID
				}
			}

			if albumName != "" {
				cleanAlbum := SanitizePathSegment(albumName)
				if cleanAlbum == "" {
					cleanAlbum = "unnamed_album"
				}
				targetDirName = filepath.Join(AlbumsDirName, cleanAlbum)
			} else {
				targetDirName = UnknownDirName
			}

			srcAbs := resolveAbsPath(driveRoot, op.SourcePath)
			cleanFilename := SanitizeFilename(filepath.Base(srcAbs))
			destRelPath := filepath.Join(targetDirName, cleanFilename)
			destAbsPath := filepath.Join(driveRoot, destRelPath)

			// 1. Resolve collision
			finalDestAbs, isDup, colErr := ResolveCollision(destAbsPath, op.FileHash)
			if colErr != nil {
				op.Status = domain.StageStatusFailed
				op.ErrorMessage = colErr.Error()
				report.FailedCount++
				report.Errors = append(report.Errors, fmt.Sprintf("collision resolution error for %s: %v", destAbsPath, colErr))
				break
			}

			// If exact duplicate exists at target, we still remove the redundant source or keep target
			var moveErr error
			if isDup && finalDestAbs != srcAbs {
				// Destination identical file exists; safely remove source
				_ = os.Remove(srcAbs)
			} else if finalDestAbs != srcAbs {
				moveErr = MoveFile(srcAbs, finalDestAbs)
			}

			if moveErr != nil {
				op.Status = domain.StageStatusFailed
				op.ErrorMessage = moveErr.Error()
				report.FailedCount++
				report.Errors = append(report.Errors, fmt.Sprintf("move failed from %s to %s: %v", srcAbs, finalDestAbs, moveErr))
				break
			}

			// 2. Compute final relative path from drive root
			finalRel, relErr := filepath.Rel(driveRoot, finalDestAbs)
			if relErr != nil {
				finalRel = destRelPath
			}
			finalRelSlash := filepath.ToSlash(finalRel)

			// 3. Update database
			if mediaStore != nil {
				if updErr := mediaStore.UpdatePath(ctx, op.MediaID, finalRelSlash); updErr != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("failed updating DB path for media %d: %v", op.MediaID, updErr))
				}
				if op.TargetAlbumID != nil {
					_, _ = mediaStore.MoveToAlbum(ctx, *op.TargetAlbumID, []int64{op.MediaID})
				} else {
					_, _ = mediaStore.RemoveFromAlbum(ctx, []int64{op.MediaID})
				}
			}

			// 4. Record Journal for rollback safety
			e.journal = append(e.journal, JournalEntry{
				MediaID:         op.MediaID,
				OldSourcePath:   srcAbs,
				NewDestPath:     finalDestAbs,
				OldRelativePath: op.SourcePath,
				NewRelativePath: finalRelSlash,
				TargetAlbumID:   op.TargetAlbumID,
				Action:          op.Action,
				ExecutedAt:      time.Now(),
			})

			op.Status = domain.StageStatusDone
			now := time.Now()
			op.ExecutedAt = &now
			op.DestinationPath = finalRelSlash
			report.MovedCount++
		}

		if onProgress != nil {
			onProgress(idx+1, len(opsList), *op)
		}
	}

	return report, nil
}

// Rollback reverses executed file movements recorded in the journal.
func (e *StagingEngine) Rollback(ctx context.Context, driveRoot string, mediaStore MediaStore) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var rollbackErrors []error

	// Reverse iterate journal
	for i := len(e.journal) - 1; i >= 0; i-- {
		entry := e.journal[i]
		if entry.Action != domain.StageActionMove {
			continue
		}

		// Move physical file back
		if err := MoveFile(entry.NewDestPath, entry.OldSourcePath); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("failed reverting file from %s to %s: %w", entry.NewDestPath, entry.OldSourcePath, err))
			continue
		}

		// Update database back to old relative path
		if mediaStore != nil {
			if err := mediaStore.UpdatePath(ctx, entry.MediaID, entry.OldRelativePath); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("failed reverting DB path for media %d: %w", entry.MediaID, err))
			}
		}
	}

	// Clear journal after rollback
	e.journal = nil

	if len(rollbackErrors) > 0 {
		return fmt.Errorf("rollback completed with %d errors: %v", len(rollbackErrors), rollbackErrors)
	}

	return nil
}

func resolveAbsPath(driveRoot, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(driveRoot, filepath.FromSlash(p))
}
