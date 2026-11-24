package catalog

import (
	"fmt"
	"strings"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/ui/components/mediagrid"
)

func (s *Screen) mediaCard(i int, item domain.MediaItem) mediagrid.Item {
	selected := s.selection.Has(item.ID)
	duration := ""
	if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
		duration = formatDuration(*item.DurationSeconds)
	}
	return mediagrid.Item{
		Key:           item.ID,
		Label:         "card " + baseName(item.CurrentRelativePath),
		Name:          baseName(item.CurrentRelativePath),
		SizeLabel:     formatSize(item.FileSize),
		DurationLabel: duration,
		Bitmap:        s.bitmapFor(item),
		Selected:      selected,
		Menu:          contextItems(selected),
		OnClick:       func(mods mui.Modifiers) { s.cardClicked(i, item, mods) },
		OnContext:     func(key string) { s.handleContext(key, item) },
	}
}

// cardClicked mirrors the grid's click semantics: while a selection is active
// (select mode) clicks toggle items into it; otherwise a plain click opens the
// viewer. Cmd toggles and Shift extends the range in both modes.
func (s *Screen) cardClicked(i int, item domain.MediaItem, mods mui.Modifiers) {
	switch {
	case mods&mui.Shift != 0:
		s.extendSelection(i)
	case mods&mui.Cmd != 0:
		s.toggleSelection(i)
	case s.selection.Len() > 0:
		s.toggleSelection(i)
	default:
		s.selected, s.pivot = i, i
		s.actions.OpenInViewer(item, s.page.Items)
	}
}

func (s *Screen) toggleSelection(i int) {
	key := s.itemKey(i)
	if s.selection.Has(key) {
		s.selection.Remove(key)
	} else {
		s.selection.Add(key)
	}
	s.selected, s.pivot = i, i
}

func (s *Screen) extendSelection(i int) {
	if s.pivot < 0 || s.pivot >= len(s.page.Items) {
		s.toggleSelection(i)
		return
	}
	s.selection.Clear()
	for j := min(s.pivot, i); j <= max(s.pivot, i); j++ {
		s.selection.Add(s.itemKey(j))
	}
	s.selected = i
}

func (s *Screen) itemKey(i int) int64 {
	if key, ok := s.grid.Key(i).(int64); ok {
		return key
	}
	return 0
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
