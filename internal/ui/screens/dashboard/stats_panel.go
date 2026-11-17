package dashboard

import (
	"fmt"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) statsPanel(c *mui.Context, tk *lobetheme.Tokens, drive *domain.DriveInfo) {
	s.mu.RLock()
	stats := s.stats
	s.mu.RUnlock()

	mui.Row(c).Gap(20).Children(func() {
		// 1. Total Media Items Card
		mui.Row(c).Grow(1).Basis(0).Padding(20).Radius(16).
			Background(tk.BgContainer).Border(1, tk.BorderSecondary).
			AlignItems(mui.Center).Gap(16).Children(func() {
			mui.Column(c).Width(42).Height(42).Radius(12).
				Background(tk.SuccessBg).Center().Children(func() {
				mui.Icon(c, lucide.Image()).FontSize(20).TextColor(tk.Success)
			})

			mui.Column(c).Gap(2).Children(func() {
				mui.Text(c, "TOTAL MEDIA ITEMS").FontSize(10).FontWeight(800).
					TextColor(tk.TextTertiary).LetterSpacing(0.5)
				mui.Text(c, fmt.Sprintf("%d", stats.TotalMedia)).
					FontSize(22).FontWeight(800).TextColor(tk.Text)
			})
		})

		// 2. Total Albums Card
		mui.Row(c).Grow(1).Basis(0).Padding(20).Radius(16).
			Background(tk.BgContainer).Border(1, tk.BorderSecondary).
			AlignItems(mui.Center).Gap(16).Children(func() {
			mui.Column(c).Width(42).Height(42).Radius(12).
				Background(tk.SuccessBg).Center().Children(func() {
				mui.Icon(c, lucide.Folder()).FontSize(20).TextColor(tk.Success)
			})

			mui.Column(c).Gap(2).Children(func() {
				mui.Text(c, "TOTAL ALBUMS").FontSize(10).FontWeight(800).
					TextColor(tk.TextTertiary).LetterSpacing(0.5)
				mui.Text(c, fmt.Sprintf("%d", stats.TotalAlbums)).
					FontSize(22).FontWeight(800).TextColor(tk.Text)
			})
		})

		// 3. Storage Used Card
		mui.Column(c).Grow(1).Basis(0).Padding(20).Radius(16).
			Background(tk.BgContainer).Border(1, tk.BorderSecondary).
			Gap(10).Children(func() {
			usedPercent := stats.UsedPercent
			if usedPercent == 0 && drive.UsedPercentage > 0 {
				usedPercent = drive.UsedPercentage
			} else if usedPercent == 0 && drive.TotalBytes > 0 && drive.FreeBytes > 0 {
				usedPercent = float64(drive.TotalBytes-drive.FreeBytes) / float64(drive.TotalBytes) * 100
			}

			mui.Row(c).AlignItems(mui.Center).Children(func() {
				mui.Text(c, "STORAGE USED").FontSize(10).FontWeight(800).
					TextColor(tk.TextTertiary).LetterSpacing(0.5)
				mui.Box(c).Grow(1)
				mui.Text(c, fmt.Sprintf("%.0f%%", usedPercent)).
					FontSize(11).FontWeight(700).TextColor(tk.Text)
			})

			status := feedback.ProgressNormal
			if usedPercent > 80 {
				status = feedback.ProgressException
			} else if usedPercent > 50 {
				status = feedback.ProgressActive
			}

			feedback.Progress(c, float32(usedPercent), feedback.ProgressProps{
				Size:     lobetheme.Small,
				Status:   status,
				ShowInfo: false,
			})

			totalCap := stats.StorageTotal
			if totalCap == 0 {
				totalCap = drive.TotalBytes
			}
			mui.Text(c, fmt.Sprintf("%s total capacity", formatBytes(totalCap))).
				FontSize(10).TextColor(tk.TextTertiary)
		})
	})
}
