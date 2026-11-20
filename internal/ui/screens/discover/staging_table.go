package discover

import (
	"fmt"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) stagingTable(c *mui.Context, tk *lobetheme.Tokens) {
	// With nothing staged the table area is simply empty; no empty state.
	if len(s.stagedOps) == 0 {
		return
	}

	mui.Column(c).Grow(1).Basis(0).Gap(8).Children(func() {
		// 1. Batch toolbar
		mui.Row(c).Shrink(0).Gap(8).AlignItems(mui.Center).Children(func() {
			mui.Text(c, fmt.Sprintf("%d items ready for consolidation", len(s.stagedOps))).
				FontSize(12).TextColor(tk.TextSecondary).FontWeight(500)

			mui.Box(c).Grow(1)

			action.Button(c, "Send All to Unknown", action.ButtonProps{
				Variant: action.Default,
				Size:    lobetheme.Small,
			}).OnClick(func() {
				s.batchAssignUnknown()
			})

			action.Button(c, "Clear Staging", action.ButtonProps{
				Variant: action.Default,
				Size:    lobetheme.Small,
			}).OnClick(func() {
				s.ClearStaging()
				s.requestRedraw()
			})
		})

		// 2. Table Column Headers
		mui.Row(c).Shrink(0).Padding(6, 12).Background(tk.BgElevated).
			Radius(6).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "File Name & Path").FontSize(11).FontWeight(600).
				TextColor(tk.TextSecondary).Width(320)
			mui.Text(c, "Action").FontSize(11).FontWeight(600).
				TextColor(tk.TextSecondary).Width(90)
			mui.Text(c, "Target Destination").FontSize(11).FontWeight(600).
				TextColor(tk.TextSecondary).Grow(1)
			mui.Text(c, "Remove").FontSize(11).FontWeight(600).
				TextColor(tk.TextSecondary).Width(50)
		})

		// 3. Staged Rows List
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mui.Column(c).Gap(4).Children(func() {
				for _, op := range s.stagedOps {
					s.stagedRow(c, tk, op)
				}
			})
		})
	})
}

func (s *Screen) stagedRow(c *mui.Context, tk *lobetheme.Tokens, op domain.StagedOperation) {
	c.Key(op.ID)

	baseName := filepath.Base(op.SourcePath)
	if baseName == "." || baseName == "/" {
		baseName = op.SourcePath
	}

	options := []field.SelectOption{
		{Value: "unknown", Label: "Unknown / Unassigned (unknown/)"},
	}
	for _, a := range s.albums {
		options = append(options, field.SelectOption{
			Value: "album:" + a,
			Label: "Album: " + a,
		})
	}
	options = append(options, field.SelectOption{
		Value: "delete",
		Label: "Mark for Deletion",
	})

	selectedVal := "unknown"
	if op.Action == domain.StageActionDelete {
		selectedVal = "delete"
	} else if op.TargetAlbumName != "" {
		selectedVal = "album:" + op.TargetAlbumName
	}

	mui.Row(c).Padding(6, 12).Radius(6).AlignItems(mui.Center).
		Background(tk.BgContainer).Children(func() {
		// File path & name
		mui.Column(c).Width(320).Gap(2).Children(func() {
			mui.Text(c, baseName).FontSize(13).FontWeight(500).SingleLine()
			mui.Text(c, op.SourcePath).FontSize(11).TextColor(tk.TextTertiary).SingleLine()
		})

		// Action tag
		mui.Row(c).Width(90).Children(func() {
			tagColor := display.TagBlue
			tagText := "MOVE"
			if op.Action == domain.StageActionDelete {
				tagColor = display.TagRed
				tagText = "DELETE"
			}
			display.Tag(c, tagText, display.TagProps{
				Size:  lobetheme.Small,
				Color: tagColor,
			})
		})

		// Destination selector
		mui.Row(c).Grow(1).Children(func() {
			curVal := selectedVal
			field.Select(c, &curVal, field.SelectProps{
				Options: options,
				Size:    lobetheme.Small,
			}).Width(280)

			if curVal != selectedVal {
				s.updateOpDestination(op.MediaID, curVal)
			}
		})

		// Remove icon button
		mui.Row(c).Width(50).Center().Children(func() {
			action.ActionIcon(c, lucide.Trash2(), action.ActionIconProps{
				Size:   lobetheme.Small,
				Danger: true,
				Title:  "Remove from staging",
			}).OnClick(func() {
				s.unstageItem(op.MediaID)
			})
		})
	})
}

func (s *Screen) updateOpDestination(mediaID int64, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stager == nil {
		return
	}

	// Fetch existing op info to recreate with new destination
	ops := s.stager.ListOperations()
	for _, op := range ops {
		if op.MediaID == mediaID {
			item := domain.MediaItem{
				ID:                  op.MediaID,
				FileHash:            op.FileHash,
				CurrentRelativePath: op.SourcePath,
			}
			s.stager.Unstage(mediaID)

			if val == "delete" {
				s.stager.StageDelete(item)
			} else if val == "unknown" {
				s.stager.StageUnknown(item)
			} else if len(val) > 6 && val[:6] == "album:" {
				albumName := val[6:]
				var albumID *int64
				if id, ok := s.albumMap[albumName]; ok {
					albumID = &id
				}
				s.stager.StageMove(item, albumID, albumName)
			}
			break
		}
	}

	s.stagedOps = s.stager.ListOperations()
	s.summary = s.stager.Summary()
	s.requestRedraw()
}

func (s *Screen) unstageItem(mediaID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stager != nil {
		s.stager.Unstage(mediaID)
		s.stagedOps = s.stager.ListOperations()
		s.summary = s.stager.Summary()
	}
	s.requestRedraw()
}

func (s *Screen) batchAssignUnknown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stager == nil {
		return
	}
	ops := s.stager.ListOperations()
	for _, op := range ops {
		item := domain.MediaItem{
			ID:                  op.MediaID,
			FileHash:            op.FileHash,
			CurrentRelativePath: op.SourcePath,
		}
		s.stager.Unstage(op.MediaID)
		s.stager.StageUnknown(item)
	}
	s.stagedOps = s.stager.ListOperations()
	s.summary = s.stager.Summary()
	s.requestRedraw()
}
