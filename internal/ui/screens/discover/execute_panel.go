package discover

import (
	"fmt"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) executePanel(c *mui.Context, tk *lobetheme.Tokens) {
	if len(s.stagedOps) == 0 || s.isExecuting {
		return
	}

	total := len(s.stagedOps)
	albumCount := 0
	for _, c := range s.summary.AlbumMoveCount {
		albumCount += c
	}
	unknownCount := s.summary.UnknownCount
	deleteCount := s.summary.DeleteCount

	desc := fmt.Sprintf("%d items queued: %d into albums, %d into unknown/, %d for deletion.",
		total, albumCount, unknownCount, deleteCount)

	mui.Row(c).Shrink(0).Padding(12, 16).Background(tk.BgElevated).
		Radius(8).AlignItems(mui.Center).Children(func() {
		mui.Row(c).Grow(1).Children(func() {
			feedback.Alert(c, "Staging Ready", feedback.AlertProps{
				Type:        feedback.Info,
				Variant:     feedback.Soft,
				Description: desc,
			})
		})

		action.Button(c, "Consolidate & Execute", action.ButtonProps{
			Variant: action.Primary,
			Size:    lobetheme.Middle,
		}).Disabled(s.isScanning).OnClick(func() {
			s.mu.Lock()
			s.modalOpen = true
			s.isExecuting = false
			s.execFinished = false
			s.execReport = nil
			s.execErr = ""
			s.mu.Unlock()
			s.requestRedraw()
		})
	})
}

func (s *Screen) confirmModal(c *mui.Context, tk *lobetheme.Tokens) {
	if !s.modalOpen {
		return
	}

	okText := "Confirm & Execute"
	if s.execFinished {
		okText = "Done"
	}
	if s.isExecuting {
		okText = "Working..."
	}

	confirmed := feedback.Modal(c, &s.modalOpen, feedback.ModalProps{
		Title:      "Consolidate Files on Drive",
		Width:      540,
		OkText:     okText,
		CancelText: "Cancel",
	}, func() {
		mui.Column(c).Padding(8, 0).Gap(14).Children(func() {
			if s.isExecuting {
				var pct float32 = 0
				if s.execTotal > 0 {
					pct = (float32(s.execCurrent) / float32(s.execTotal)) * 100
				}
				feedback.Progress(c, pct, feedback.ProgressProps{
					Size:     lobetheme.Small,
					Color:    tk.Primary,
					Label:    fmt.Sprintf("Relocating item %d of %d...", s.execCurrent, s.execTotal),
					ShowInfo: true,
				})
				mui.Text(c, "Please keep the drive connected while operations are in progress.").
					FontSize(12).TextColor(tk.TextTertiary)
				return
			}

			if s.execFinished && s.execReport != nil {
				feedback.Alert(c, "Consolidation Complete", feedback.AlertProps{
					Type:        feedback.Success,
					Variant:     feedback.Soft,
					Description: fmt.Sprintf("Successfully moved %d files, deleted %d items. Failed: %d.", s.execReport.MovedCount, s.execReport.DeletedCount, s.execReport.FailedCount),
				})
				return
			}

			// Pre-execution confirmation prompt
			mui.Text(c, "You are about to physically reorganize staged media files on your external drive.").
				FontSize(13).FontWeight(500)

			feedback.Alert(c, "Physical Relocation Warning", feedback.AlertProps{
				Type:        feedback.Warning,
				Variant:     feedback.Soft,
				Description: "Files will be moved into structured directories (albums/ and unknown/). Existing filenames with duplicate contents will be preserved, and name collisions will be safely suffixed.",
			})

			mui.Column(c).Gap(4).Children(func() {
				mui.Text(c, fmt.Sprintf("• Total operations: %d", len(s.stagedOps))).FontSize(12)
				mui.Text(c, fmt.Sprintf("• Files destined for named albums: %d", len(s.stagedOps)-s.summary.UnknownCount-s.summary.DeleteCount)).FontSize(12)
				mui.Text(c, fmt.Sprintf("• Files destined for unorganized unknown/: %d", s.summary.UnknownCount)).FontSize(12)
				if s.summary.DeleteCount > 0 {
					mui.Text(c, fmt.Sprintf("• Files scheduled for physical deletion: %d", s.summary.DeleteCount)).FontSize(12).TextColor(tk.Error)
				}
			})

			if s.execErr != "" {
				feedback.Alert(c, s.execErr, feedback.AlertProps{
					Type:    feedback.Error,
					Variant: feedback.Soft,
				})
			}
		})
	})

	if confirmed {
		if s.execFinished {
			s.mu.Lock()
			s.modalOpen = false
			s.execFinished = false
			s.mu.Unlock()
			s.requestRedraw()
			return
		}

		if !s.isExecuting {
			s.runConsolidation()
		}
	}
}

func (s *Screen) runConsolidation() {
	s.mu.Lock()
	if s.closed || s.isExecuting || s.isScanning || len(s.stagedOps) == 0 {
		s.mu.Unlock()
		return
	}
	s.isExecuting = true
	s.execFinished = false
	s.execCurrent = 0
	s.execTotal = len(s.stagedOps)
	s.execErr = ""

	driveRoot := s.driveRoot
	if driveRoot == "" {
		driveRoot = s.targetPath
	}
	s.jobs.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.jobs.Done()
		report, err := s.stager.Execute(s.ctx, driveRoot, s.mediaStore, s.albumStore, func(cur, tot int, _ domain.StagedOperation) {
			s.publish(func() {
				s.execCurrent = cur
				s.execTotal = tot
			})
		})

		s.publish(func() {
			s.isExecuting = false
			s.execFinished = true
			s.execReport = report
			if err != nil {
				s.execErr = err.Error()
			}
			// Completed moves must not be replayed. Leave failed items queued.
			for _, op := range s.stager.ListOperations() {
				if op.Status == domain.StageStatusDone {
					s.stager.Unstage(op.MediaID)
				}
			}
			s.stagedOps = s.stager.ListOperations()
			s.summary = s.stager.Summary()
			if s.store != nil {
				s.store.MediaChanged("")
			}
		})
	}()
}
