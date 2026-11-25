package albums

import (
	"fmt"
	"sort"
	"strings"

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

var galleryFilterOptions = []string{"All", "With media", "Empty"}

var gallerySortOptions = []field.SelectOption{
	{Value: "name_asc", Label: "Name A-Z"},
	{Value: "items_desc", Label: "Most items"},
	{Value: "size_desc", Label: "Largest size"},
	{Value: "newest", Label: "Newest first"},
}

func (s *Screen) galleryView(c *mui.Context, tk *lobetheme.Tokens) {
	mui.Column(c).Grow(1).Basis(0).Gap(12).Children(func() {
		filtered := s.getFilteredSummaries()
		s.galleryFilterBar(c, tk, len(filtered))

		if len(filtered) == 0 {
			if s.albumsLoading && s.summaryCount() == 0 {
				s.gallerySkeleton(c)
				return
			}
			emptyTitle := "No albums yet"
			emptyDesc := "Organize your photos and videos into structured collections on your external drive."
			hasFilter := strings.TrimSpace(s.searchQuery) != "" || s.galleryFilter != 0
			if s.summaryCount() > 0 && hasFilter {
				emptyTitle = "No matching albums"
				emptyDesc = "Try a different search or filter."
			}
			mui.Scroll(c).Grow(1).Basis(0).Children(func() {
				mui.Column(c).Fill().Center().Padding(40).Gap(12).Children(func() {
					display.Empty(c, display.EmptyProps{
						Icon:        lucide.Images(),
						Title:       emptyTitle,
						Description: emptyDesc,
					})
					if !hasFilter && s.summaryCount() == 0 {
						action.Button(c, "Create Your First Album", action.ButtonProps{
							Variant: action.Primary,
							Size:    lobetheme.Middle,
						}).Disabled(s.readOnly).OnClick(func() { s.openCreateModal() })
					}
				})
			})
			return
		}

		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
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
	})
}

// galleryFilterBar renders the album search input, occupancy filter, count tag,
// sort selector, and primary create button in a flat borderless bar.
func (s *Screen) galleryFilterBar(c *mui.Context, tk *lobetheme.Tokens, visibleCount int) {
	mui.Row(c).Shrink(0).Gap(10).AlignItems(mui.Center).Children(func() {
		field.SearchBar(c, &s.searchQuery, field.SearchBarProps{
			Placeholder: "Search albums...",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		}).Grow(1).Basis(200).MinWidth(120).MaxWidth(340)

		action.Segmented(c, &s.galleryFilter, galleryFilterOptions, action.SegmentedProps{
			Size: lobetheme.Small,
		})

		mui.Box(c).Grow(1)

		if s.albumsLoading && s.summaryCount() == 0 {
			display.Tag(c, "Loading…", display.TagProps{
				Size: lobetheme.Small, Color: display.TagBlue,
			}).TextColor(tk.TextSecondary).Shrink(0)
		} else {
			display.Tag(c, fmt.Sprintf("%d albums", visibleCount), display.TagProps{
				Size: lobetheme.Small, Color: display.TagBlue,
			}).TextColor(tk.TextSecondary).Shrink(0)
		}

		selected := "name_asc"
		if s.gallerySort >= 0 && s.gallerySort < len(gallerySortOptions) {
			selected = gallerySortOptions[s.gallerySort].Value
		}
		field.Select(c, &selected, field.SelectProps{
			Options:     gallerySortOptions,
			Placeholder: "Sort by",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		}).Shrink(0)
		for index, option := range gallerySortOptions {
			if option.Value == selected {
				s.gallerySort = index
				break
			}
		}

		action.Button(c, "New Album", action.ButtonProps{
			Variant: action.Primary,
			Size:    lobetheme.Small,
		}).Disabled(s.readOnly).Shrink(0).OnClick(func() { s.openCreateModal() })
	})
}

// gallerySkeleton holds the gallery's place while the first summaries fetch
// loads: album-card-shaped placeholders with a cover block and text rows.
func (s *Screen) gallerySkeleton(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	mui.Scroll(c).Grow(1).Basis(0).Label("albums-loading").Children(func() {
		layout.Grid(c, layout.GridProps{
			MaxItemWidth: 220,
			MaxColumns:   6,
			Gap:          12,
		}).Children(func() {
			for i := 0; i < 12; i++ {
				mui.Column(c).FillWidth().Radius(tk.BorderRadiusLG).Background(tk.BgContainer).
					Border(1, tk.BorderSecondary).Clip().Children(func() {
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
		Border(1, tk.BorderSecondary).Clip()
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
			mui.Row(c).AlignItems(mui.Center).Gap(6).Children(func() {
				mui.Text(c, sum.Name).FontSize(14).FontWeight(600).Grow(1).SingleLine().MinWidth(0).
					OnClick(func() { s.OpenAlbum(sum) })
				display.Tag(c, fmt.Sprintf("%d", sum.MediaCount), display.TagProps{
					Size: lobetheme.Small, Color: display.TagBlue,
				}).Shrink(0)
			})

			subtitle, subtitleColor := "", tk.TextTertiary
			if sum.Description != nil && strings.TrimSpace(*sum.Description) != "" {
				subtitle = strings.TrimSpace(*sum.Description)
				subtitleColor = tk.TextSecondary
			} else if sum.TotalBytes > 0 {
				subtitle = formatBytes(uint64(sum.TotalBytes))
			} else if sum.MediaCount == 0 {
				subtitle = "Empty album"
			} else {
				subtitle = fmt.Sprintf("%d items", sum.MediaCount)
			}
			mui.Text(c, subtitle).FontSize(12).TextColor(subtitleColor).SingleLine()

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

func (s *Screen) summaryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.summaries)
}

func (s *Screen) getFilteredSummaries() []domain.AlbumSummary {
	s.mu.RLock()
	summaries := make([]domain.AlbumSummary, len(s.summaries))
	copy(summaries, s.summaries)
	query := strings.ToLower(strings.TrimSpace(s.searchQuery))
	filter := s.galleryFilter
	sortIdx := s.gallerySort
	s.mu.RUnlock()

	result := make([]domain.AlbumSummary, 0, len(summaries))
	for _, sum := range summaries {
		if filter == 1 && sum.MediaCount == 0 {
			continue
		}
		if filter == 2 && sum.MediaCount > 0 {
			continue
		}
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(sum.Name), query)
			descMatch := sum.Description != nil && strings.Contains(strings.ToLower(*sum.Description), query)
			if !nameMatch && !descMatch {
				continue
			}
		}
		result = append(result, sum)
	}

	switch sortIdx {
	case 1:
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].MediaCount != result[j].MediaCount {
				return result[i].MediaCount > result[j].MediaCount
			}
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	case 2:
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].TotalBytes != result[j].TotalBytes {
				return result[i].TotalBytes > result[j].TotalBytes
			}
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	case 3:
		sort.SliceStable(result, func(i, j int) bool {
			if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
				return result[i].CreatedAt.After(result[j].CreatedAt)
			}
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	default:
		sort.SliceStable(result, func(i, j int) bool {
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
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
