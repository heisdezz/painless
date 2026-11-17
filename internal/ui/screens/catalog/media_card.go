package catalog

import (
	"fmt"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) mediaCard(c *mui.Context, item domain.MediaItem) {
	tk := lobetheme.TokensOf(c)
	selected := s.selection.Has(item.ID)
	card := mui.Column(c).Gap(4).Padding(6).Radius(tk.BorderRadius).AlignSelf(mui.Stretch).
		Label("card " + baseName(item.CurrentRelativePath))
	if selected {
		card.Background(tk.FillSecondary).Border(1, tk.Primary)
	} else {
		card.Border(1, tk.BorderSecondary)
	}
	card.Children(func() {
		if bitmap := s.bitmapFor(item); bitmap != nil {
			mui.Image(c, bitmap).Width(148).Height(110).Fit(mui.Cover).Radius(tk.BorderRadiusSM)
		} else {
			display.Skeleton(c, display.SkeletonProps{Width: 148, Height: 110})
		}
		mui.Text(c, baseName(item.CurrentRelativePath)).FontSize(12).SingleLine()
		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			mui.Text(c, formatSize(item.FileSize)).FontSize(11).TextColor(tk.TextTertiary)
			if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
				display.Tag(c, formatDuration(*item.DurationSeconds), display.TagProps{
					Size: lobetheme.Small, Color: display.TagGeekblue,
				})
			}
		})
	})
	if key := overlay.ContextMenu(c, card, overlay.DropdownMenuProps{Items: contextItems()}); key != "" {
		s.handleContext(key, item)
	}
}

func baseName(path string) string {
	if slash := strings.LastIndexAny(path, `/\\`); slash >= 0 {
		return path[slash+1:]
	}
	return path
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	units := []string{"KB", "MB", "GB", "TB"}
	unit := -1
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("0:%02d", seconds)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
