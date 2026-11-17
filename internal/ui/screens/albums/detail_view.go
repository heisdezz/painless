package albums

import (
	"fmt"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) detailView(c *mui.Context, tk *lobetheme.Tokens) {
	if s.selectedAlbum == nil {
		return
	}
	album := *s.selectedAlbum

	mui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
		// 1. Back navigation and action header
		mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
			action.Button(c, "Back to Albums", action.ButtonProps{
				Variant: action.Default,
				Size:    lobetheme.Small,
			}).OnClick(func() {
				s.CloseDetail()
			})

			mui.Box(c).Grow(1)

			if !s.readOnly {
				action.Button(c, "Edit Album", action.ButtonProps{
					Variant: action.Default,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.openEditModal(album)
				})

				action.Button(c, "Merge Into...", action.ButtonProps{
					Variant: action.Default,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.openMergeModal(album)
				})

				action.Button(c, "Delete Album", action.ButtonProps{
					Variant: action.Default,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.openDeleteModal(album)
				})
			}
		})

		// 2. Album Banner
		mui.Column(c).Shrink(0).Padding(16, 20).Background(tk.BgElevated).
			Radius(10).Gap(8).Children(func() {
			mui.Row(c).AlignItems(mui.Center).Gap(12).Children(func() {
				mui.Text(c, album.Name).FontSize(20).FontWeight(700)

				display.Tag(c, fmt.Sprintf("%d items", s.detailTotal), display.TagProps{
					Size:  lobetheme.Small,
					Color: display.TagBlue,
				})

				if album.TotalBytes > 0 {
					display.Tag(c, formatBytes(uint64(album.TotalBytes)), display.TagProps{
						Size:  lobetheme.Small,
						Color: display.TagGreen,
					})
				}
			})

			desc := "No description provided."
			if album.Description != nil && *album.Description != "" {
				desc = *album.Description
			}
			mui.Text(c, desc).FontSize(13).TextColor(tk.TextSecondary)
		})

		// 3. Member Media Grid
		if len(s.detailItems) == 0 {
			mui.Scroll(c).Grow(1).Basis(0).Children(func() {
				mui.Column(c).Fill().Center().Padding(48).Children(func() {
					display.Empty(c, display.EmptyProps{
						Icon:        lucide.Images(),
						Title:       "No media in this album",
						Description: "Assign items to this album from the Media Library or Discover screen.",
					})
				})
			})
			return
		}

		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mui.Row(c).Wrap().Gap(12).Children(func() {
				for _, item := range s.detailItems {
					s.memberMediaCard(c, tk, item)
				}
			})
		})
	})
}

func (s *Screen) memberMediaCard(c *mui.Context, tk *lobetheme.Tokens, item domain.MediaItem) {
	c.Key(fmt.Sprintf("media-%d", item.ID))

	baseName := filepath.Base(item.CurrentRelativePath)
	if baseName == "." || baseName == "/" {
		baseName = item.CurrentRelativePath
	}

	mui.Column(c).Width(160).Radius(8).Background(tk.BgContainer).
		Border(1, tk.BorderSecondary).Children(func() {
		// Thumbnail placeholder
		mui.Column(c).Height(110).Background(tk.BgElevated).Center().Children(func() {
			mui.Icon(c, lucide.Image()).FontSize(28).TextColor(tk.TextTertiary)
		})

		// Meta info
		mui.Column(c).Padding(8).Gap(4).Children(func() {
			mui.Text(c, baseName).FontSize(12).FontWeight(500).SingleLine()
			mui.Row(c).AlignItems(mui.Center).Children(func() {
				mui.Text(c, formatBytes(uint64(item.FileSize))).
					FontSize(10).TextColor(tk.TextTertiary)

				mui.Box(c).Grow(1)

				display.Tag(c, item.MimeType, display.TagProps{
					Size:  lobetheme.Small,
					Color: display.TagBlue,
				})
			})
		})
	})
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
