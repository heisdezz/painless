package albums

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/fileops"
)

func (s *Screen) createModal(c *mui.Context, tk *lobetheme.Tokens) {
	if s.readOnly || !s.createOpen {
		return
	}

	confirmed := feedback.Modal(c, &s.createOpen, feedback.ModalProps{
		Title:      "Create New Album",
		Width:      480,
		OkText:     "Create Album",
		CancelText: "Cancel",
	}, func() {
		mui.Column(c).Padding(8, 0).Gap(12).Children(func() {
			mui.Text(c, "Album Name").FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
			field.Input(c, &s.createName, field.InputProps{
				Placeholder: "e.g. Summer Vacation 2026",
				Size:        lobetheme.Small,
			})

			mui.Text(c, "Description (optional)").FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
			field.TextArea(c, &s.createDesc, field.TextAreaProps{
				Placeholder: "Add notes or a description for this collection...",
				MinRows:     3,
				MaxRows:     5,
			})

			if s.createErr != "" {
				feedback.Alert(c, s.createErr, feedback.AlertProps{
					Type:    feedback.Error,
					Variant: feedback.Soft,
				})
			}
		})
	})

	if confirmed {
		trimmedName := strings.TrimSpace(s.createName)
		if !domain.ValidateAlbumName(trimmedName) {
			s.mu.Lock()
			s.createErr = "Invalid album name. Prohibited characters: / \\ : * ? \" < > |"
			s.mu.Unlock()
			s.requestRedraw()
			return
		}

		ctx := context.Background()
		if s.albums != nil {
			_, err := s.albums.Create(ctx, trimmedName, s.createDesc)
			if err != nil {
				s.mu.Lock()
				s.createErr = err.Error()
				s.mu.Unlock()
				s.requestRedraw()
				return
			}
		}

		if s.driveRoot != "" {
			_, _ = fileops.EnsureAlbumDir(s.driveRoot, trimmedName)
		}

		s.mu.Lock()
		s.createOpen = false
		s.createName = ""
		s.createDesc = ""
		s.createErr = ""
		s.mu.Unlock()

		s.Reload()
		s.requestRedraw()
	}
}

func (s *Screen) editModal(c *mui.Context, tk *lobetheme.Tokens) {
	if s.readOnly || !s.editOpen || s.editTarget == nil {
		return
	}

	confirmed := feedback.Modal(c, &s.editOpen, feedback.ModalProps{
		Title:      "Edit Album Details",
		Width:      480,
		OkText:     "Save Changes",
		CancelText: "Cancel",
	}, func() {
		mui.Column(c).Padding(8, 0).Gap(12).Children(func() {
			mui.Text(c, "Album Name").FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
			field.Input(c, &s.editName, field.InputProps{
				Size: lobetheme.Small,
			})

			mui.Text(c, "Description").FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
			field.TextArea(c, &s.editDesc, field.TextAreaProps{
				MinRows: 3,
				MaxRows: 5,
			})

			if s.editErr != "" {
				feedback.Alert(c, s.editErr, feedback.AlertProps{
					Type:    feedback.Error,
					Variant: feedback.Soft,
				})
			}
		})
	})

	if confirmed {
		trimmedName := strings.TrimSpace(s.editName)
		if !domain.ValidateAlbumName(trimmedName) {
			s.mu.Lock()
			s.editErr = "Invalid album name. Prohibited characters: / \\ : * ? \" < > |"
			s.mu.Unlock()
			s.requestRedraw()
			return
		}

		ctx := context.Background()
		if s.albums != nil {
			album := s.editTarget.Album
			album.Name = trimmedName
			album.Description = &s.editDesc
			album.RelativePath = filepath.Join(fileops.AlbumsDirName, fileops.SanitizePathSegment(trimmedName))

			if err := s.albums.Update(ctx, album); err != nil {
				s.mu.Lock()
				s.editErr = err.Error()
				s.mu.Unlock()
				s.requestRedraw()
				return
			}
		}

		// Rename physical directory on disk if name changed
		if s.driveRoot != "" && s.editTarget.Name != trimmedName {
			oldDir := filepath.Join(fileops.AlbumsDir(s.driveRoot), fileops.SanitizePathSegment(s.editTarget.Name))
			newDir := filepath.Join(fileops.AlbumsDir(s.driveRoot), fileops.SanitizePathSegment(trimmedName))
			if _, err := os.Stat(oldDir); err == nil {
				_ = os.Rename(oldDir, newDir)
			}
		}

		s.mu.Lock()
		s.editOpen = false
		s.editTarget = nil
		s.editErr = ""
		s.mu.Unlock()

		s.Reload()
		s.requestRedraw()
	}
}

func (s *Screen) deleteModal(c *mui.Context, tk *lobetheme.Tokens) {
	if s.readOnly || !s.deleteOpen || s.deleteTarget == nil {
		return
	}

	targetName := s.deleteTarget.Name

	confirmed := feedback.Modal(c, &s.deleteOpen, feedback.ModalProps{
		Title:      fmt.Sprintf("Delete Album: %s", targetName),
		Width:      480,
		OkText:     "Confirm Delete",
		CancelText: "Cancel",
	}, func() {
		mui.Column(c).Padding(8, 0).Gap(14).Children(func() {
			feedback.Alert(c, "Album Deletion Warning", feedback.AlertProps{
				Type:        feedback.Warning,
				Variant:     feedback.Soft,
				Description: fmt.Sprintf("You are about to remove album %q containing %d items.", targetName, s.deleteTarget.MediaCount),
			})

			mui.Text(c, "Select Deletion Policy:").FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)

			choice.Radio(c, &s.deleteMode, "soft", "Keep media files (Soft delete)", choice.RadioProps{})
			mui.Text(c, "Removes the album assignment. Files stay where they are.").
				FontSize(11).TextColor(tk.TextTertiary).Padding(0, 24)

			choice.Radio(c, &s.deleteMode, "permanent", "Delete media permanently", choice.RadioProps{})
			mui.Text(c, "Removes the media records from the catalog. Files stay on the drive.").
				FontSize(11).TextColor(tk.Error).Padding(0, 24)

			if s.deleteErr != "" {
				feedback.Alert(c, s.deleteErr, feedback.AlertProps{
					Type:    feedback.Error,
					Variant: feedback.Soft,
				})
			}
		})
	})

	if confirmed {
		ctx := context.Background()
		softDelete := s.deleteMode != "permanent"

		if s.albums != nil {
			if err := s.albums.Delete(ctx, s.deleteTarget.ID, softDelete); err != nil {
				s.mu.Lock()
				s.deleteErr = err.Error()
				s.mu.Unlock()
				s.requestRedraw()
				return
			}
		}

		s.mu.Lock()
		s.deleteOpen = false
		if s.selectedAlbum != nil && s.selectedAlbum.ID == s.deleteTarget.ID {
			s.selectedAlbum = nil
			s.detailItems = nil
		}
		s.deleteTarget = nil
		s.deleteErr = ""
		s.mu.Unlock()

		s.Reload()
		s.requestRedraw()
	}
}

func (s *Screen) mergeModal(c *mui.Context, tk *lobetheme.Tokens) {
	if s.readOnly || !s.mergeOpen || s.mergeSource == nil {
		return
	}

	source := *s.mergeSource

	options := make([]field.SelectOption, 0)
	for _, sum := range s.summaries {
		if sum.ID != source.ID {
			options = append(options, field.SelectOption{
				Value: strconv.FormatInt(sum.ID, 10),
				Label: sum.Name,
			})
		}
	}

	confirmed := feedback.Modal(c, &s.mergeOpen, feedback.ModalProps{
		Title:      fmt.Sprintf("Merge Album %q", source.Name),
		Width:      480,
		OkText:     "Merge Albums",
		CancelText: "Cancel",
	}, func() {
		mui.Column(c).Padding(8, 0).Gap(14).Children(func() {
			feedback.Alert(c, "Consolidation Notice", feedback.AlertProps{
				Type:        feedback.Info,
				Variant:     feedback.Soft,
				Description: fmt.Sprintf("All media items from %q will be merged into the target album, and %q will be removed.", source.Name, source.Name),
			})

			mui.Text(c, "Destination Target Album:").FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)

			field.Select(c, &s.mergeTarget, field.SelectProps{
				Options:     options,
				Placeholder: "Select destination album...",
				Size:        lobetheme.Small,
			})

			if s.mergeErr != "" {
				feedback.Alert(c, s.mergeErr, feedback.AlertProps{
					Type:    feedback.Error,
					Variant: feedback.Soft,
				})
			}
		})
	})

	if confirmed {
		targetID, err := strconv.ParseInt(s.mergeTarget, 10, 64)
		if err != nil || targetID == 0 {
			s.mu.Lock()
			s.mergeErr = "Please select a valid destination album."
			s.mu.Unlock()
			s.requestRedraw()
			return
		}

		ctx := context.Background()
		if s.albums != nil {
			if err := s.albums.Merge(ctx, source.ID, targetID); err != nil {
				s.mu.Lock()
				s.mergeErr = err.Error()
				s.mu.Unlock()
				s.requestRedraw()
				return
			}
		}

		s.mu.Lock()
		s.mergeOpen = false
		if s.selectedAlbum != nil && s.selectedAlbum.ID == source.ID {
			s.selectedAlbum = nil
			s.detailItems = nil
		}
		s.mergeSource = nil
		s.mergeTarget = ""
		s.mergeErr = ""
		s.mu.Unlock()

		s.Reload()
		s.requestRedraw()
	}
}
