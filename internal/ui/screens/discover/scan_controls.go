package discover

import (
	"context"
	"fmt"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/scanner"
)

func (s *Screen) scanControls(c *mui.Context, tk *lobetheme.Tokens) {
	mui.Column(c).Shrink(0).Gap(10).Children(func() {
		// 1. Target path input & control buttons
		mui.Row(c).Gap(12).AlignItems(mui.Center).Children(func() {
			mui.Row(c).Grow(1).Children(func() {
				field.Input(c, &s.targetPath, field.InputProps{
					Placeholder: "Enter folder or drive mount to scan (e.g. /media/drive or D:\\)",
					Size:        lobetheme.Small,
				}).Width(360)
			})

			if !s.isScanning {
				action.Button(c, "Start Scan", action.ButtonProps{
					Variant: action.Primary,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.startScan()
				})
			} else {
				if s.isPaused {
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
		if s.isScanning {
			mui.Column(c).Gap(6).Padding(8, 0).Children(func() {
				var pct float32 = 0
				if s.scanProg.FilesScanned > 0 {
					// Approximate progress pulse
					pct = float32(s.scanProg.FilesScanned%100) + 1
				}

				feedback.Progress(c, pct, feedback.ProgressProps{
					Size:     lobetheme.Small,
					Color:    tk.Primary,
					Label:    "Scanning filesystem...",
					ShowInfo: false,
				})

				// Real-time stat counters
				mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
					display.Tag(c, fmt.Sprintf("%d scanned", s.scanProg.FilesScanned), display.TagProps{
						Size:  lobetheme.Small,
						Color: display.TagBlue,
					})
					display.Tag(c, fmt.Sprintf("%d discovered", s.scanProg.MediaFound), display.TagProps{
						Size:  lobetheme.Small,
						Color: display.TagGreen,
					})
					if s.scanProg.Duplicates > 0 {
						display.Tag(c, fmt.Sprintf("%d duplicates", s.scanProg.Duplicates), display.TagProps{
							Size:  lobetheme.Small,
							Color: display.TagOrange,
						})
					}

					if s.scanProg.CurrentPath != "" {
						base := filepath.Base(s.scanProg.CurrentPath)
						mui.Text(c, base).FontSize(11).TextColor(tk.TextTertiary).SingleLine()
					}
				})
			})
		}

		if s.lastScanErr != "" {
			feedback.Alert(c, s.lastScanErr, feedback.AlertProps{
				Type:    feedback.Error,
				Variant: feedback.Soft,
			})
		}
	})
}

func (s *Screen) startScan() {
	s.mu.Lock()
	if s.isScanning {
		s.mu.Unlock()
		return
	}

	target := s.targetPath
	if target == "" && s.store != nil {
		if drive, ok := s.store.State.ActiveDrive(); ok {
			target = drive.Path
		}
	}
	if target == "" {
		s.lastScanErr = "Please select or enter a target directory to scan."
		s.mu.Unlock()
		s.requestRedraw()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.scanCancel = cancel
	s.isScanning = true
	s.isPaused = false
	s.lastScanErr = ""
	s.scanProg = scanner.Progress{IsRunning: true}

	var sc Scanner
	if s.scannerFactory != nil {
		sc = s.scannerFactory(scanner.Config{
			DriveRoot: target,
			TargetDir: target,
		}, func(p scanner.Progress) {
			s.mu.Lock()
			s.scanProg = p
			s.mu.Unlock()
			s.requestRedraw()
		})
	}
	s.activeScan = sc
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.isScanning = false
			s.isPaused = false
			s.activeScan = nil
			s.mu.Unlock()
			s.requestRedraw()
		}()

		if sc != nil {
			if err := sc.Scan(ctx); err != nil && err != context.Canceled {
				s.mu.Lock()
				s.lastScanErr = fmt.Sprintf("Scan failed: %v", err)
				s.mu.Unlock()
			}
		}
	}()

	s.requestRedraw()
}

func (s *Screen) pauseScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeScan != nil && !s.isPaused {
		s.activeScan.Pause()
		s.isPaused = true
		s.scanProg.IsPaused = true
	}
	s.requestRedraw()
}

func (s *Screen) resumeScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeScan != nil && s.isPaused {
		s.activeScan.Resume()
		s.isPaused = false
		s.scanProg.IsPaused = false
	}
	s.requestRedraw()
}

func (s *Screen) stopScan() {
	s.mu.Lock()
	if s.scanCancel != nil {
		s.scanCancel()
	}
	if s.activeScan != nil {
		s.activeScan.Stop()
	}
	s.isScanning = false
	s.isPaused = false
	s.mu.Unlock()
	s.requestRedraw()
}
