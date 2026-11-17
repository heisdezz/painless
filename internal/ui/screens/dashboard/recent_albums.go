package dashboard

import (
	"fmt"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) recentAlbumsSection(c *mui.Context, tk *lobetheme.Tokens) {
	s.mu.RLock()
	albums := s.recentAlbums
	loading := s.loading
	s.mu.RUnlock()

	mui.Column(c).Gap(14).Children(func() {
		// 1. Header with View All action
		mui.Row(c).AlignItems(mui.Center).Padding(0, 0, 8).Children(func() {
			mui.Text(c, "RECENT ALBUMS").FontSize(12).FontWeight(800).
				TextColor(tk.TextSecondary).LetterSpacing(0.8)

			mui.Box(c).Grow(1)

			action.Button(c, "View All Albums →", action.ButtonProps{
				Variant: action.Text,
				Size:    lobetheme.Small,
			}).OnClick(func() {
				s.actions.NavigateToAlbums()
			})
		})

		// 2. Loading state
		if loading && len(albums) == 0 {
			mui.Row(c).Padding(32).Center().Children(func() {
				feedback.Spin(c, feedback.SpinProps{
					Size: lobetheme.Middle,
					Tip:  "Loading albums...",
				}, nil)
			})
			return
		}

		// 3. Empty state
		if len(albums) == 0 {
			mui.Column(c).Fill().Center().Padding(32).Radius(16).
				Background(tk.BgContainer).Border(1, tk.BorderSecondary).Gap(8).Children(func() {
				mui.Icon(c, lucide.Folder()).FontSize(32).TextColor(tk.TextTertiary)
				mui.Text(c, "No albums created yet").FontSize(13).FontWeight(600).TextColor(tk.TextSecondary)
				mui.Text(c, "Group your organized media into custom collections in the Albums tab.").
					FontSize(11).TextColor(tk.TextTertiary)
			})
			return
		}

		// 4. Album Cards Grid
		mui.Row(c).Wrap().Gap(16).Children(func() {
			for _, album := range albums {
				s.albumSummaryCard(c, tk, album)
			}
		})
	})
}

func (s *Screen) albumSummaryCard(c *mui.Context, tk *lobetheme.Tokens, album domain.AlbumSummary) {
	c.Key(fmt.Sprintf("recent-album-%d", album.ID))

	displayName := album.Name
	if displayName == "unknown" {
		displayName = "Unsorted Media"
	}

	mui.Row(c).Width(270).Padding(12).Radius(14).
		Background(tk.BgContainer).Border(1, tk.BorderSecondary).
		AlignItems(mui.Center).Gap(12).
		OnClick(func() {
			s.actions.OpenAlbum(album)
		}).Children(func() {
		// Album cover container
		mui.Column(c).Width(52).Height(52).Radius(10).
			Background(tk.BgElevated).Center().Children(func() {
			mui.Icon(c, lucide.Images()).FontSize(24).TextColor(tk.TextTertiary)
		})

		// Details
		mui.Column(c).Grow(1).Gap(3).Children(func() {
			mui.Text(c, displayName).FontSize(13).FontWeight(700).
				TextColor(tk.Text).SingleLine()

			countText := fmt.Sprintf("%d items", album.MediaCount)
			if album.MediaCount == 1 {
				countText = "1 item"
			}
			display.Tag(c, countText, display.TagProps{
				Size: lobetheme.Small,
			})
		})
	})
}
