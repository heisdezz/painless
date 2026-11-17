package discover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/scanner"
)

func (s *Screen) scanControls(c *mui.Context, tk *lobetheme.Tokens) {
	s.mu.RLock()
	scanning, paused := s.isScanning, s.isPaused
	progress, scanErr := s.scanProg, s.lastScanErr
	s.mu.RUnlock()

	mui.Column(c).Shrink(0).Gap(10).Children(func() {
		// 1. Target path input & control buttons
		mui.Row(c).Gap(12).AlignItems(mui.Center).Children(func() {
			mui.Row(c).Grow(1).Children(func() {
				field.Input(c, &s.targetPath, field.InputProps{
					Placeholder: "Enter folder or drive mount to scan (e.g. /media/drive or D:\\)",
					Size:        lobetheme.Small,
				}).Width(360)
			})

			if !scanning {
				action.Button(c, "Start Scan", action.ButtonProps{
					Variant: action.Primary,
					Size:    lobetheme.Small,
				}).Disabled(s.scannerFactory == nil || s.isExecuting).OnClick(func() {
					s.startScan()
				})
			} else {
				if paused {
					action.Button(c, "Resume", action.ButtonProps{
						Variant: action.Default,
						Size:    lobetheme.Small,
					}).OnClick(func() {
						s.resumeScan()
					})
				} else {
					action.Button(c, "Pause", action.ButtonProps{
						Variant: action.Default,
						Size:    lobetheme.Small,
					}).OnClick(func() {
						s.pauseScan()
					})
				}

				action.Button(c, "Stop", action.ButtonProps{
					Variant: action.Default,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.stopScan()
				})
			}
		})

		// 2. Active scan progress bar and status tags
		if scanning {
			mui.Column(c).Gap(6).Padding(8, 0).Children(func() {
				var pct float32 = 0
				if progress.FilesScanned > 0 {
					// Approximate progress pulse
					pct = float32(progress.FilesScanned%100) + 1
				}

				feedback.Progress(c, pct, feedback.ProgressProps{
					Size:     lobetheme.Small,
					Color:    tk.Primary,
					Label:    "Scanning filesystem...",
					ShowInfo: false,
				})

				// Real-time stat counters
				mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
					display.Tag(c, fmt.Sprintf("%d scanned", progress.FilesScanned), display.TagProps{
						Size:  lobetheme.Small,
						Color: display.TagBlue,
					})
					display.Tag(c, fmt.Sprintf("%d discovered", progress.MediaFound), display.TagProps{
						Size:  lobetheme.Small,
						Color: display.TagGreen,
					})
					if progress.Duplicates > 0 {
						display.Tag(c, fmt.Sprintf("%d duplicates", s.scanProg.Duplicates), display.TagProps{
							Size:  lobetheme.Small,
							Color: display.TagOrange,
						})
					}

					if progress.CurrentPath != "" {
						base := filepath.Base(progress.CurrentPath)
						mui.Text(c, base).FontSize(11).TextColor(tk.TextTertiary).SingleLine()
					}
				})
			})
		}

		if scanErr != "" {
			feedback.Alert(c, scanErr, feedback.AlertProps{
				Type:    feedback.Error,
				Variant: feedback.Soft,
			})
		}
	})
}

func (s *Screen) startScan() {
	s.mu.Lock()
	s.startScanLocked(s.targetPath, s.scannerFactory, false)
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) startScanLocked(target string, factory ScannerFactory, albumScan bool) {
	if s.closed || s.isScanning || s.isExecuting || factory == nil {
		return
	}
	if target == "" && s.store != nil {
		if drive, ok := s.store.State.ActiveDrive(); ok {
			target = drive.Path
		}
	}
	if target == "" {
		s.lastScanErr = "Please select or enter a target directory to scan."
		return
	}

	root := s.driveRoot
	if root == "" {
		root = target
	} else {
		var err error
		target, err = scanDirectory(root, target)
		if err != nil {
			s.lastScanErr = err.Error()
			return
		}
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.scanCancel = cancel
	s.isScanning = true
	s.isPaused = false
	s.lastScanErr = ""
	s.scanProg = scanner.Progress{IsRunning: true}

	var lastProgress time.Time
	sc := factory(scanner.Config{
		DriveRoot: root,
		TargetDir: target,
		Ignore:    s.ignorePatterns(),
	}, func(p scanner.Progress) {
		// A scan can report once per file; don't flood the window queue.
		if p.IsRunning && time.Since(lastProgress) < 50*time.Millisecond {
			return
		}
		lastProgress = time.Now()
		s.publish(func() { s.scanProg = p })
	})
	s.activeScan = sc
	s.jobs.Add(1)

	go func() {
		defer s.jobs.Done()
		err := sc.Scan(ctx)
		stopped := ctx.Err() != nil
		cancel()
		s.publish(func() {
			s.isScanning = false
			s.isPaused = false
			s.activeScan = nil
			s.scanCancel = nil
			if err != nil && !stopped {
				s.lastScanErr = fmt.Sprintf("Scan failed: %v", err)
			}
			if albumScan && s.store != nil {
				// A canceled or failed scan can still have committed earlier batches.
				s.store.MediaChanged("")
			}
		})
	}()
}

func scanDirectory(root, target string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("Cannot open the active drive: %w", err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("Cannot open scan folder: %w", err)
	}
	rel, err := filepath.Rel(canonicalRoot, canonicalTarget)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return "", errors.New("Choose a scan folder inside the active drive.")
	}
	info, err := os.Stat(canonicalTarget)
	if err != nil || !info.IsDir() {
		return "", errors.New("Choose an existing directory to scan.")
	}
	return filepath.Join(root, rel), nil
}

func (s *Screen) pauseScan() {
	s.mu.Lock()
	scan := s.activeScan
	pause := scan != nil && !s.isPaused && s.scanCancel != nil
	if pause {
		s.isPaused = true
	}
	s.mu.Unlock()
	if pause {
		scan.Pause()
	}
	s.requestRedraw()
}

func (s *Screen) resumeScan() {
	s.mu.Lock()
	scan := s.activeScan
	resume := scan != nil && s.isPaused && s.scanCancel != nil
	if resume {
		s.isPaused = false
	}
	s.mu.Unlock()
	if resume {
		scan.Resume()
	}
	s.requestRedraw()
}

func (s *Screen) stopScan() {
	s.mu.Lock()
	if s.scanCancel != nil {
		s.scanCancel()
	}
	scan := s.activeScan
	s.scanCancel = nil
	s.isPaused = false
	s.mu.Unlock()
	if scan != nil {
		scan.Stop()
	}
	s.requestRedraw()
}
