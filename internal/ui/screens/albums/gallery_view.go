package albums

import (
	"fmt"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) galleryView(c *mui.Context, tk *lobetheme.Tokens) {
	// 1. Top action bar
	mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
		field.SearchBar(c, &s.searchQuery, field.SearchBarProps{
			Placeholder: "Filter albums...",
			Size:        lobetheme.Small,
		}).Width(260)

		mui.Box(c).Grow(1)

		action.Button(c, "New Album", action.ButtonProps{
			Variant: action.Primary,
			Size:    lobetheme.Small,
		}).OnClick(func() {
			s.mu.Lock()
			s.createOpen = true
			s.createName = ""
			s.createDesc = ""
			s.createErr = ""
			s.mu.Unlock()
			s.requestRedraw()
		})
	})

	filtered := s.getFilteredSummaries()

	// 2. Empty state
	if len(filtered) == 0 {
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mui.Column(c).Fill().Center().Padding(48).Gap(16).Children(func() {
				display.Empty(c, display.EmptyProps{
					Icon:        lucide.Images(),
					Title:       "No albums yet",
					Description: "Organize your photos and videos into structured collections on your external drive.",
				})

				action.Button(c, "Create Your First Album", action.ButtonProps{
					Variant: action.Primary,
					Size:    lobetheme.Middle,
				}).OnClick(func() {
					s.mu.Lock()
					s.createOpen = true
					s.createName = ""
					s.createDesc = ""
					s.createErr = ""
					s.mu.Unlock()
					s.requestRedraw()
				})
			})
		})
		return
	}

	// 3. Album Cards Grid
	mui.Scroll(c).Grow(1).Basis(0).Children(func() {
		mui.Row(c).Wrap().Gap(16).Children(func() {
			for _, sum := range filtered {
				s.albumCard(c, tk, sum)
			}
		})
	})
}

func (s *Screen) albumCard(c *mui.Context, tk *lobetheme.Tokens, sum domain.AlbumSummary) {
	c.Key(fmt.Sprintf("album-%d", sum.ID))

	mui.Column(c).Width(240).Radius(10).Background(tk.BgContainer).
		Border(1, tk.BorderSecondary).Children(func() {
		// Cover area
		mui.Column(c).Height(130).Background(tk.BgElevated).Center().
			OnClick(func() {
				s.OpenAlbum(sum)
			}).Children(func() {
			mui.Icon(c, lucide.Images()).FontSize(36).TextColor(tk.TextTertiary)
		})

		// Details area
		mui.Column(c).Padding(12).Gap(8).Children(func() {
			mui.Row(c).AlignItems(mui.Center).Children(func() {
				mui.Text(c, sum.Name).FontSize(14).FontWeight(600).Grow(1).SingleLine().
					OnClick(func() {
						s.OpenAlbum(sum)
					})

				display.Tag(c, fmt.Sprintf("%d", sum.MediaCount), display.TagProps{
					Size:  lobetheme.Small,
					Color: display.TagBlue,
				})
			})

			desc := "No description"
			if sum.Description != nil && *sum.Description != "" {
				desc = *sum.Description
			}
			mui.Text(c, desc).FontSize(12).TextColor(tk.TextSecondary).SingleLine()

			// Card action buttons
			mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
				action.Button(c, "Open", action.ButtonProps{
					Variant: action.Default,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.OpenAlbum(sum)
				})

				mui.Box(c).Grow(1)

				// Edit button
				action.ActionIcon(c, lucide.Pencil(), action.ActionIconProps{
					Size:  lobetheme.Small,
					Title: "Edit album info",
				}).OnClick(func() {
					s.openEditModal(sum)
				})

				// Merge button
				action.ActionIcon(c, lucide.GitMerge(), action.ActionIconProps{
					Size:  lobetheme.Small,
					Title: "Merge into another album",
				}).OnClick(func() {
					s.openMergeModal(sum)
				})

				// Delete button
				action.ActionIcon(c, lucide.Trash2(), action.ActionIconProps{
					Size:   lobetheme.Small,
					Danger: true,
					Title:  "Delete album",
				}).OnClick(func() {
					s.openDeleteModal(sum)
				})
			})
		})
	})
}

func (s *Screen) getFilteredSummaries() []domain.AlbumSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.TrimSpace(s.searchQuery) == "" {
		return s.summaries
	}

	q := strings.ToLower(strings.TrimSpace(s.searchQuery))
	var res []domain.AlbumSummary
	for _, sum := range s.summaries {
		if strings.Contains(strings.ToLower(sum.Name), q) {
			res = append(res, sum)
		} else if sum.Description != nil && strings.Contains(strings.ToLower(*sum.Description), q) {
			res = append(res, sum)
		}
	}
	return res
}

func (s *Screen) openEditModal(sum domain.AlbumSummary) {
	s.mu.Lock()
	s.editOpen = true
	s.editTarget = &sum
	s.editName = sum.Name
	s.editDesc = ""
	if sum.Description != nil {
		s.editDesc = *sum.Description
	}
	s.editErr = ""
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) openMergeModal(sum domain.AlbumSummary) {
	s.mu.Lock()
	s.mergeOpen = true
	s.mergeSource = &sum
	s.mergeTarget = ""
	s.mergeErr = ""
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) openDeleteModal(sum domain.AlbumSummary) {
	s.mu.Lock()
	s.deleteOpen = true
	s.deleteTarget = &sum
	s.deleteMode = "soft"
	s.deleteErr = ""
	s.mu.Unlock()
	s.requestRedraw()
}
