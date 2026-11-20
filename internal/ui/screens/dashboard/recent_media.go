package dashboard

import (
	"context"
	"fmt"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/ui/components/mediagrid"
)

func (s *Screen) recentMediaSection(c *mui.Context, tk *lobetheme.Tokens) {
	s.mu.RLock()
	media := s.recentMedia
	loading := s.loading
	s.mu.RUnlock()

	mui.Column(c).Gap(14).Children(func() {
		// 1. Header with View All action
		mui.Row(c).AlignItems(mui.Center).Padding(0, 0, 8).Children(func() {
			mui.Text(c, "RECENTLY ADDED MEDIA").FontSize(12).FontWeight(800).
				TextColor(tk.TextSecondary).LetterSpacing(0.8)

			mui.Box(c).Grow(1)

			action.Button(c, "View All Media →", action.ButtonProps{
				Variant: action.Text,
				Size:    lobetheme.Small,
			}).OnClick(func() {
				s.actions.NavigateToMedia()
			})
		})

		// 2. Loading state
		if loading && len(media) == 0 {
			mui.Column(c).Padding(32).Center().Gap(8).Children(func() {
				feedback.Spin(c, feedback.SpinProps{Size: lobetheme.Middle}, nil)
				mui.Text(c, "Querying catalog database...").FontSize(12).TextColor(tk.TextTertiary)
			})
			return
		}

		// 3. Empty state
		if len(media) == 0 {
			mui.Column(c).Fill().Center().Padding(40).Radius(16).
				Background(tk.BgContainer).Border(1, tk.BorderSecondary).Gap(12).Children(func() {
				mui.Icon(c, lucide.Image()).FontSize(36).TextColor(tk.TextTertiary)
				mui.Text(c, "No Media Found").FontSize(14).FontWeight(700).TextColor(tk.Text)
				mui.Text(c, "This storage volume has not been cataloged yet. Switch to Discover to scan files.").
					FontSize(12).TextColor(tk.TextTertiary)

				action.Button(c, "Scan Folder", action.ButtonProps{
					Variant: action.Primary,
					Size:    lobetheme.Small,
				}).OnClick(func() {
					s.actions.NavigateToDiscover()
				})
			})
			return
		}

		// 4. Wrapped strip of fixed-size tiles, identical to the library grid.
		cards := make([]mediagrid.Item, 0, len(media))
		for _, item := range media {
			cards = append(cards, s.mediaPreviewCard(item, media))
		}
		mediagrid.Strip(c, cards)
	})
}

func (s *Screen) mediaPreviewCard(item domain.MediaItem, allItems []domain.MediaItem) mediagrid.Item {
	name := filepath.Base(item.CurrentRelativePath)
	duration := ""
	if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
		duration = formatMediaDuration(*item.DurationSeconds)
	}

	return mediagrid.Item{
		Key:           fmt.Sprintf("recent-media-%d", item.ID),
		Label:         "card " + name,
		Name:          name,
		SizeLabel:     formatBytes(uint64(item.FileSize)),
		DurationLabel: duration,
		Bitmap:        s.bitmapFor(item),
		Menu: []overlay.MenuItem{
			{Key: "open", Label: "Open in Viewer", Icon: lucide.Image()},
			{Key: "library", Label: "Go to Library", Icon: lucide.Library()},
		},
		OnClick: func(mui.Modifiers) {
			s.actions.OpenViewer(item, allItems)
		},
		OnContext: func(key string) {
			switch key {
			case "open":
				s.actions.OpenViewer(item, allItems)
			case "library":
				s.actions.NavigateToMedia()
			}
		},
	}
}

func formatMediaDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("0:%02d", seconds)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func (s *Screen) bitmapFor(item domain.MediaItem) *mui.Bitmap {
	if s.thumbsDisabled() {
		return nil
	}
	s.mu.Lock()
	if s.bitmaps == nil {
		s.bitmaps = make(map[string]*mui.Bitmap)
	}
	if bitmap, ok := s.bitmaps[item.FileHash]; ok {
		s.mu.Unlock()
		return bitmap
	}
	if s.pendingBitmaps == nil {
		s.pendingBitmaps = make(map[string]bool)
	}
	if s.pendingBitmaps[item.FileHash] || s.thumbs == nil {
		s.mu.Unlock()
		return nil
	}
	s.pendingBitmaps[item.FileHash] = true
	thumbs := s.thumbs
	s.mu.Unlock()

	go func(hash string, media domain.MediaItem) {
		defer func() {
			s.mu.Lock()
			delete(s.pendingBitmaps, hash)
			s.mu.Unlock()
		}()
		data, err := thumbs.Get(context.Background(), media)
		if err == nil && len(data) > 0 {
			if bmp, decErr := mui.DecodeBitmap(data); decErr == nil {
				s.mu.Lock()
				s.bitmaps[hash] = bmp
				s.mu.Unlock()
				s.requestRedraw()
			}
		}
	}(item.FileHash, item)

	return nil
}

// thumbsDisabled reports whether the display-thumbnails setting is off.
func (s *Screen) thumbsDisabled() bool {
	return s.thumbsEnabled != nil && !s.thumbsEnabled()
}
