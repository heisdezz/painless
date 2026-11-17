package dashboard

import (
	"context"
	"fmt"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
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
			mui.Row(c).Padding(32).Center().Children(func() {
				feedback.Spin(c, feedback.SpinProps{
					Size:   lobetheme.Middle,
					Tip: "Querying catalog database...",
				}, nil)
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

		// 4. Horizontal preview grid (8 items)
		mui.Row(c).Wrap().Gap(14).Children(func() {
			for _, item := range media {
				s.mediaPreviewCard(c, tk, item, media)
			}
		})
	})
}

func (s *Screen) mediaPreviewCard(c *mui.Context, tk *lobetheme.Tokens, item domain.MediaItem, allItems []domain.MediaItem) {
	c.Key(fmt.Sprintf("recent-media-%d", item.ID))

	bitmap := s.bitmapFor(item)

	mui.Column(c).Width(125).Height(125).Radius(12).
		Background(tk.BgElevated).Border(1, tk.BorderSecondary).
		OnClick(func() {
			s.actions.OpenViewer(item, allItems)
		}).Children(func() {
		// Image thumbnail container
		mui.Box(c).Grow(1).Center().Children(func() {
			if bitmap != nil {
				display.Image(c, bitmap, display.ImageProps{
					Width:   123,
					Height:  95,
					Variant: display.ImageFilled,
				})
			} else {
				mui.Icon(c, lucide.Image()).FontSize(28).TextColor(tk.TextTertiary)
			}
		})

		// Bottom tag pill with filename/folder
		mui.Row(c).Height(26).Padding(0, 6).AlignItems(mui.Center).
			Background(tk.BgContainer).Children(func() {
			name := filepath.Base(item.CurrentRelativePath)
			mui.Icon(c, lucide.Folder()).FontSize(11).TextColor(tk.Success)
			mui.Box(c).Width(4)
			mui.Text(c, name).FontSize(10).FontWeight(600).
				TextColor(tk.TextSecondary).SingleLine().Grow(1)
		})
	})
}

func (s *Screen) bitmapFor(item domain.MediaItem) *mui.Bitmap {
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
