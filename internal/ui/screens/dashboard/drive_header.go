package dashboard

import (
	"fmt"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) driveHeader(c *mui.Context, tk *lobetheme.Tokens, drive *domain.DriveInfo) {
	s.mu.RLock()
	stats := s.stats
	s.mu.RUnlock()

	mui.Row(c).Padding(24).Radius(16).Background(tk.BgContainer).
		Border(1, tk.BorderSecondary).AlignItems(mui.Center).Gap(20).Children(func() {
		// 1. Green Drive Squircle Icon
		mui.Column(c).Width(48).Height(48).Radius(14).
			Background(tk.Success).Center().Children(func() {
			if drive.Type == "external" || drive.Type == "removable" {
				mui.Icon(c, lucide.Usb()).FontSize(24).TextColor(tk.White)
			} else {
				mui.Icon(c, lucide.HardDrive()).FontSize(24).TextColor(tk.White)
			}
		})

		// 2. Drive Identity & Album Path
		mui.Column(c).Grow(1).Gap(4).Children(func() {
			driveName := drive.Name
			if driveName == "" {
				driveName = "External Media Volume"
			}
			mui.Text(c, driveName).FontSize(18).FontWeight(800).TextColor(tk.Text)

			subtitle := fmt.Sprintf("%s · %s TOTAL", strings.ToUpper(drive.Path), formatBytes(drive.TotalBytes))
			mui.Text(c, subtitle).FontSize(10).FontWeight(700).
				TextColor(tk.TextTertiary).LetterSpacing(0.5)

			albumLoc := stats.AlbumLocation
			if albumLoc == "" {
				albumLoc = fmt.Sprintf("%s/media/albums", drive.Path)
			}
			mui.Row(c).AlignItems(mui.Center).Gap(6).Children(func() {
				mui.Text(c, "Album folder:").FontSize(11).TextColor(tk.TextSecondary)
				mui.Text(c, albumLoc).FontSize(11).TextColor(tk.Text).SingleLine()
			})
		})

		// 3. Right Action Controls
		mui.Row(c).AlignItems(mui.Center).Gap(10).Children(func() {
			if stats.Scanning {
				mui.Row(c).Padding(6, 12).Radius(8).Background(tk.PrimaryBg).
					Border(1, tk.PrimaryBorder).AlignItems(mui.Center).Gap(8).Children(func() {
					feedback.Spin(c, feedback.SpinProps{
						Size: lobetheme.Small,
					}, nil)
					mui.Column(c).Children(func() {
						mui.Text(c, "INDEXING ACTIVE").FontSize(9).FontWeight(800).
							TextColor(tk.Primary).LetterSpacing(0.5)
						mui.Text(c, fmt.Sprintf("Found %d items...", stats.FoundCount)).
							FontSize(10).TextColor(tk.TextSecondary)
					})
				})
			}

			// Scan Volume button
			action.Button(c, "Scan Volume", action.ButtonProps{
				Variant: action.Default,
				Size:    lobetheme.Middle,
			}).OnClick(func() {
				s.actions.NavigateToDiscover()
			})

			// Refresh action
			action.ActionIcon(c, lucide.RefreshCw(), action.ActionIconProps{
				Size:  lobetheme.Middle,
				Title: "Refresh dashboard",
			}).OnClick(func() {
				s.actions.Refresh()
				s.Reload()
			})

			// Power / Disconnect action
			action.ActionIcon(c, lucide.Power(), action.ActionIconProps{
				Size:   lobetheme.Middle,
				Danger: true,
				Title:  "Disconnect drive",
			}).OnClick(func() {
				s.actions.DisconnectDrive()
			})
		})
	})
}
