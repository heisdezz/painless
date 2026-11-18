package albums

import (
	"fmt"
	"path/filepath"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/ui/components/mediagrid"
)

func (s *Screen) detailView(c *mui.Context, tk *lobetheme.Tokens) {
	if s.selectedAlbum == nil {
		return
	}
	album := *s.selectedAlbum

	mui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
		// Borderless toolbar: navigation on the left, quiet actions on the right.
		mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
			action.Button(c, "Back to Albums", action.ButtonProps{
				Variant: action.Text, Size: lobetheme.Small,
			}).OnClick(func() { s.CloseDetail() })

			mui.Box(c).Grow(1)
			if !s.readOnly {
				action.Button(c, "Edit Album", action.ButtonProps{
					Variant: action.Text, Size: lobetheme.Small,
				}).OnClick(func() { s.openEditModal(album) })
				action.Button(c, "Merge Into...", action.ButtonProps{
					Variant: action.Text, Size: lobetheme.Small,
				}).OnClick(func() { s.openMergeModal(album) })
				action.Button(c, "Delete Album", action.ButtonProps{
					Variant: action.Text, Size: lobetheme.Small,
				}).OnClick(func() { s.openDeleteModal(album) })
			}
		})

		// Album identity: one tonal step, a hairline, no shadow.
		mui.Column(c).Shrink(0).Padding(16, 20).Background(tk.BgContainer).
			Border(1, tk.BorderSecondary).Radius(tk.BorderRadiusLG).Gap(8).Children(func() {
			mui.Row(c).AlignItems(mui.Center).Gap(12).Children(func() {
				mui.Text(c, album.Name).FontSize(18).FontWeight(800)
				display.Tag(c, fmt.Sprintf("%d items", s.detailTotal), display.TagProps{
					Size: lobetheme.Small, Color: display.TagBlue,
				})
				if album.TotalBytes > 0 {
					display.Tag(c, formatBytes(uint64(album.TotalBytes)), display.TagProps{
						Size: lobetheme.Small, Color: display.TagGray,
					})
				}
			})
			desc := "No description provided."
			if album.Description != nil && *album.Description != "" {
				desc = *album.Description
			}
			mui.Text(c, desc).FontSize(13).TextColor(tk.TextSecondary)
		})

		if len(s.detailItems) == 0 {
			mui.Scroll(c).Grow(1).Basis(0).Children(func() {
				mui.Column(c).Fill().Center().Padding(40).Children(func() {
					display.Empty(c, display.EmptyProps{
						Icon:        lucide.Images(),
						Title:       "No media in this album",
						Description: "Assign items to this album from the Media Library or Discover screen.",
					})
				})
			})
			return
		}

		cards := make([]mediagrid.Item, 0, len(s.detailItems))
		for _, item := range s.detailItems {
			cards = append(cards, s.memberCard(item))
		}
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mediagrid.Strip(c, cards)
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
				s.onOpenItem(item)
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
