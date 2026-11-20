package albums

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/ui/components/mediacard"
)

func (s *Screen) galleryView(c *mui.Context, tk *lobetheme.Tokens) {
	// The gallery scrolls under the toolbar: a bar in normal flow would clip
	// the cards at the scroll's edge and leave nothing behind the glass.
	mui.Box(c).Grow(1).Basis(0).Children(func() {
		filtered := s.getFilteredSummaries()
		if len(filtered) == 0 {
			if s.albumsLoading && len(s.summaries) == 0 {
				s.gallerySkeleton(c)
			} else {
				mui.Scroll(c).Fill().Padding(64, 0, 0).Children(func() {
					mui.Column(c).Fill().Center().Gap(12).Children(func() {
						display.Empty(c, display.EmptyProps{
							Icon:        lucide.Images(),
							Title:       "No albums yet",
							Description: "Organize your photos and videos into structured collections on your external drive.",
						})
						action.Button(c, "Create Your First Album", action.ButtonProps{
							Variant: action.Primary,
							Size:    lobetheme.Middle,
						}).Disabled(s.readOnly).OnClick(func() { s.openCreateModal() })
					})
				})
			}
		} else {
			mui.Scroll(c).Fill().Padding(64, 0, 0).ScrollbarInsets(64, 0, 0).Children(func() {
				// Responsive equal columns: cards stretch to fill the content width.
				layout.Grid(c, layout.GridProps{
					MaxItemWidth: 220,
					MaxColumns:   6,
					Gap:          12,
				}).Children(func() {
					for index, sum := range filtered {
						s.albumCard(c, tk, sum, index)
					}
				})
			})
		}

		// Toolbar: quiet search, grow spacer, one primary action. Built last so
		// it paints above the cards passing behind the liquid-glass pane;
		// padding gives the controls room inside its bezel.
		mui.Row(c).Absolute().Top(0).Left(0).Right(0).Gap(12).AlignItems(mui.Center).
			Padding(10, 12).Radius(tk.BorderRadius).Material(glass.Glass{}).Children(func() {
			field.SearchBar(c, &s.searchQuery, field.SearchBarProps{
				Placeholder: "Filter albums...",
				Variant:     field.InputFilled,
				Size:        lobetheme.Small,
			}).Width(280)
			mui.Box(c).Grow(1)
			action.Button(c, "New Album", action.ButtonProps{
				Variant: action.Primary,
				Size:    lobetheme.Small,
			}).Disabled(s.readOnly).OnClick(func() { s.openCreateModal() })
		})
	})
}

// gallerySkeleton holds the gallery's place while the first summaries fetch
// loads: album-card-shaped placeholders with a cover block and text rows.
func (s *Screen) gallerySkeleton(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	mui.Scroll(c).Fill().Padding(64, 0, 0).Label("albums-loading").Children(func() {
		layout.Grid(c, layout.GridProps{
			MaxItemWidth: 220,
			MaxColumns:   6,
			Gap:          12,
		}).Children(func() {
			for i := 0; i < 12; i++ {
				mui.Column(c).FillWidth().Radius(tk.BorderRadiusLG).Background(tk.BgContainer).
					Border(1, tk.BorderSecondary).Children(func() {
					display.Skeleton(c, display.SkeletonProps{Height: 130})
					mui.Column(c).Padding(12).Gap(8).Children(func() {
						display.Skeleton(c, display.SkeletonProps{Width: 110, Height: 14})
						display.Skeleton(c, display.SkeletonProps{Width: 150, Height: 12})
						display.Skeleton(c, display.SkeletonProps{Width: 64, Height: 11})
					})
				})
			}
		})
	})
}

func (s *Screen) albumCard(c *mui.Context, tk *lobetheme.Tokens, sum domain.AlbumSummary, index int) {
	c.Key(fmt.Sprintf("album-%d", sum.ID))

	card := mui.Column(c).FillWidth().Radius(tk.BorderRadiusLG).Background(tk.BgContainer).
		Border(1, tk.BorderSecondary)
	card.Children(func() {
		// Cover preview from the album's first indexed item; the icon is the
		// fallback until a thumbnail has been cached.
		mui.Column(c).Height(130).Background(tk.BgElevated).Center().Clip().
			OnClick(func() { s.OpenAlbum(sum) }).Children(func() {
			if sum.PreviewItem != nil {
				if bitmap := s.bitmapFor(*sum.PreviewItem); bitmap != nil {
					mui.Image(c, bitmap).FillWidth().Height(130).Fit(mui.Cover)
					return
				}
			}
			mui.Icon(c, lucide.FolderClosed()).FontSize(36).TextColor(tk.TextTertiary)
		})

		mui.Column(c).Padding(12).Gap(8).Children(func() {
			mui.Row(c).AlignItems(mui.Center).Children(func() {
				mui.Text(c, sum.Name).FontSize(14).FontWeight(600).Grow(1).SingleLine().
					OnClick(func() { s.OpenAlbum(sum) })
				display.Tag(c, fmt.Sprintf("%d", sum.MediaCount), display.TagProps{
					Size: lobetheme.Small, Color: display.TagBlue,
				})
			})

			desc := "No description"
			if sum.Description != nil && *sum.Description != "" {
				desc = *sum.Description
			}
			mui.Text(c, desc).FontSize(12).TextColor(tk.TextSecondary).SingleLine()

			mui.Box(c).Height(1).FillWidth().Background(tk.BorderSecondary)
			mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
				action.Button(c, "Open", action.ButtonProps{
					Variant: action.Text, Size: lobetheme.Small,
				}).OnClick(func() { s.OpenAlbum(sum) })
				mui.Box(c).Grow(1)
				if s.readOnly {
					return
				}
				action.ActionIcon(c, lucide.Pencil(), action.ActionIconProps{
					Size: lobetheme.Small, Title: "Edit album info",
				}).OnClick(func() { s.openEditModal(sum) })
				action.ActionIcon(c, lucide.GitMerge(), action.ActionIconProps{
					Size: lobetheme.Small, Title: "Merge into another album",
				}).OnClick(func() { s.openMergeModal(sum) })
				action.ActionIcon(c, lucide.Trash2(), action.ActionIconProps{
					Size: lobetheme.Small, Danger: true, Title: "Delete album",
				}).OnClick(func() { s.openDeleteModal(sum) })
			})
		})
	})
	mediacard.ApplyReveal(c, card, s.galleryReveal, index)
}

func (s *Screen) getFilteredSummaries() []domain.AlbumSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.TrimSpace(s.searchQuery) == "" {
		return s.summaries
	}
	query := strings.ToLower(strings.TrimSpace(s.searchQuery))
	var result []domain.AlbumSummary
	for _, sum := range s.summaries {
		if strings.Contains(strings.ToLower(sum.Name), query) {
			result = append(result, sum)
		} else if sum.Description != nil && strings.Contains(strings.ToLower(*sum.Description), query) {
			result = append(result, sum)
		}
	}
	return result
}

func (s *Screen) openCreateModal() {
	s.mu.Lock()
	s.createOpen, s.createName, s.createDesc, s.createErr = true, "", "", ""
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) openEditModal(sum domain.AlbumSummary) {
	s.mu.Lock()
	s.editOpen, s.editTarget, s.editName, s.editErr = true, &sum, sum.Name, ""
	s.editDesc = ""
	if sum.Description != nil {
		s.editDesc = *sum.Description
	}
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) openMergeModal(sum domain.AlbumSummary) {
	s.mu.Lock()
	s.mergeOpen, s.mergeSource, s.mergeTarget, s.mergeErr = true, &sum, "", ""
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) openDeleteModal(sum domain.AlbumSummary) {
	s.mu.Lock()
	s.deleteOpen, s.deleteTarget, s.deleteMode, s.deleteErr = true, &sum, "soft", ""
	s.mu.Unlock()
	s.requestRedraw()
}
