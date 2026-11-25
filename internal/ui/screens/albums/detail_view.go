package albums

import (
	"fmt"
	"path/filepath"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/nav"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/ui/components/mediacard"
	"painless/internal/ui/components/mediagrid"
)

var detailSortOptions = []field.SelectOption{
	{Value: "date_desc", Label: "Newest first"},
	{Value: "date_asc", Label: "Oldest first"},
	{Value: "name", Label: "Name A-Z"},
	{Value: "size", Label: "File size"},
}

var detailFilterOptions = []string{"All", "Photos", "Videos"}

func (s *Screen) detailView(c *mui.Context, tk *lobetheme.Tokens) {
	if s.selectedAlbum == nil {
		return
	}
	album := *s.selectedAlbum
	s.ensureDetailItems(album.ID)

	mui.Column(c).Grow(1).Basis(0).Gap(10).Children(func() {
		// Compact borderless album header: navigation, identity, and quiet actions
		// share one row, separated from the media controls by a 1px hairline.
		mui.Column(c).Shrink(0).Gap(4).Children(func() {
			mui.Row(c).Shrink(0).Gap(10).AlignItems(mui.Center).Children(func() {
				action.Button(c, "Back to Albums", action.ButtonProps{
					Variant: action.Text, Size: lobetheme.Small,
				}).Shrink(0).OnClick(func() { s.CloseDetail() })

				mui.Box(c).Width(1).Height(14).Shrink(0).Background(tk.BorderSecondary)

				mui.Row(c).Grow(1).Basis(0).MinWidth(0).AlignItems(mui.Center).Gap(8).Children(func() {
					mui.Text(c, album.Name).FontSize(16).FontWeight(800).SingleLine().Shrink(1).MinWidth(0)
					if s.detailLoading && s.detailTotal == 0 {
						display.Tag(c, "Loading…", display.TagProps{
							Size: lobetheme.Small, Color: display.TagBlue,
						}).Shrink(0)
					} else {
						display.Tag(c, fmt.Sprintf("%d items", s.detailTotal), display.TagProps{
							Size: lobetheme.Small, Color: display.TagBlue,
						}).Shrink(0)
					}
					if album.TotalBytes > 0 {
						display.Tag(c, formatBytes(uint64(album.TotalBytes)), display.TagProps{
							Size: lobetheme.Small, Color: display.TagGray,
						}).Shrink(0)
					}
				})

				if !s.readOnly {
					mui.Row(c).Shrink(0).Gap(4).AlignItems(mui.Center).Children(func() {
						action.ActionIcon(c, lucide.Pencil(), action.ActionIconProps{
							Size: lobetheme.Small, Title: "Edit album info",
						}).OnClick(func() { s.openEditModal(album) })
						action.ActionIcon(c, lucide.GitMerge(), action.ActionIconProps{
							Size: lobetheme.Small, Title: "Merge into another album",
						}).OnClick(func() { s.openMergeModal(album) })
						action.ActionIcon(c, lucide.Trash2(), action.ActionIconProps{
							Size: lobetheme.Small, Danger: true, Title: "Delete album",
						}).OnClick(func() { s.openDeleteModal(album) })
					})
				}
			})

			if album.Description != nil {
				if desc := strings.TrimSpace(*album.Description); desc != "" {
					mui.Text(c, desc).FontSize(12).TextColor(tk.TextSecondary).SingleLine()
				}
			}

			mui.Box(c).Height(1).FillWidth().Background(tk.BorderSecondary).Margin(4, 0, 0)
		})

		s.detailFilterBar(c)

		if len(s.detailItems) == 0 {
			if s.detailLoading {
				s.memberSkeleton(c)
				return
			}
			emptyTitle := "No media in this album"
			emptyDesc := "Assign items to this album from the Media Library or Discover screen."
			if s.detailCommittedQuery != "" || s.detailFilter != 0 {
				emptyTitle = "No matching media"
				emptyDesc = "Try a different search or filter."
			}
			mui.Scroll(c).Grow(1).Basis(0).Children(func() {
				mui.Column(c).Fill().Center().Padding(40).Children(func() {
					display.Empty(c, display.EmptyProps{
						Icon:        lucide.Images(),
						Title:       emptyTitle,
						Description: emptyDesc,
					})
				})
			})
			return
		}

		cards := make([]mediagrid.Item, 0, len(s.detailItems))
		for index, item := range s.detailItems {
			card := s.memberCard(item)
			card.Reveal, card.RevealIndex = s.detailReveal, index
			cards = append(cards, card)
		}
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mediagrid.Strip(c, cards)
		})
		s.memberPager(c)
	})
}

// detailFilterBar renders the search input, media-type filter, and sort selector
// for the album's content list, matching the catalog filter bar.
func (s *Screen) detailFilterBar(c *mui.Context) {
	mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
		field.SearchBar(c, &s.detailQuery, field.SearchBarProps{
			Placeholder: "Search album media",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		}).Grow(1).Basis(220).MinWidth(140).MaxWidth(360)
		action.Segmented(c, &s.detailFilter, detailFilterOptions, action.SegmentedProps{Size: lobetheme.Small})

		mui.Box(c).Grow(1)

		selected := "date_desc"
		if s.detailSort >= 0 && s.detailSort < len(detailSortOptions) {
			selected = detailSortOptions[s.detailSort].Value
		}
		field.Select(c, &selected, field.SelectProps{
			Options:     detailSortOptions,
			Placeholder: "Sort by",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		}).Shrink(0)
		for index, option := range detailSortOptions {
			if option.Value == selected {
				s.detailSort = index
				break
			}
		}
	})
}

// memberSkeleton holds the member strip's place while the album's members
// load: a wrapped row of the shared card-shaped skeleton tiles.
func (s *Screen) memberSkeleton(c *mui.Context) {
	mui.Scroll(c).Grow(1).Basis(0).Label("album-members-loading").Children(func() {
		mui.Row(c).Wrap().Gap(8).Children(func() {
			for i := 0; i < 24; i++ {
				mediacard.SkeletonTile(c)
			}
		})
	})
}

// memberPager pages through the album's members below the member grid.
func (s *Screen) memberPager(c *mui.Context) {
	if s.detailTotal <= memberPageSize {
		return
	}
	mui.Row(c).Shrink(0).AlignItems(mui.Center).Children(func() {
		nav.Pagination(c, &s.memberPage, nav.PaginationProps{
			Total:            s.detailTotal,
			PageSize:         memberPageSize,
			HideOnSinglePage: true,
			Size:             lobetheme.Small,
		})
	})
}

// memberCard reuses the library's fixed-size media tile for album members.
func (s *Screen) memberCard(item domain.MediaItem) mediagrid.Item {
	name := filepath.Base(item.CurrentRelativePath)
	if name == "." || name == "/" {
		name = item.CurrentRelativePath
	}
	duration := ""
	if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
		seconds := *item.DurationSeconds
		if seconds < 60 {
			duration = fmt.Sprintf("0:%02d", seconds)
		} else {
			duration = fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
		}
	}
	return mediagrid.Item{
		Key:           fmt.Sprintf("media-%d", item.ID),
		Label:         "card " + name,
		Name:          name,
		SizeLabel:     formatBytes(uint64(item.FileSize)),
		DurationLabel: duration,
		FormatLabel:   mimeSubtype(item.MimeType),
		Bitmap:        s.bitmapFor(item),
		OnClick: func(mui.Modifiers) {
			if s.onOpenItem != nil {
				all := append([]domain.MediaItem(nil), s.detailItems...)
				s.onOpenItem(item, all)
			}
		},
	}
}

// mimeSubtype reports the uppercase format identity for the signal-blue tag.
func mimeSubtype(mime string) string {
	if slash := strings.IndexByte(mime, '/'); slash >= 0 && slash+1 < len(mime) {
		return mime[slash+1:]
	}
	return mime
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
