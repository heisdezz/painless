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

func (s *Screen) drivePicker(c *mui.Context, tk *lobetheme.Tokens) {
	s.mu.RLock()
	drives := s.drives
	loading := s.loading
	s.mu.RUnlock()

	// 1. Header with title and refresh action
	mui.Row(c).AlignItems(mui.Center).Children(func() {
		mui.Text(c, "SELECT A CONNECTED DRIVE").
			FontSize(14).FontWeight(800).TextColor(tk.TextSecondary).LetterSpacing(1.0)

		mui.Box(c).Grow(1)

		action.Button(c, "Refresh Drives", action.ButtonProps{
			Variant: action.Text,
			Size:    lobetheme.Small,
		}).OnClick(func() {
			s.actions.Refresh()
			s.Reload()
		})
	})

	// 2. Drive list or Empty state
	if len(drives) == 0 {
		if loading {
			mui.Column(c).Fill().Center().Padding(60).Gap(12).Children(func() {
				feedback.Spin(c, feedback.SpinProps{
					Size: lobetheme.Large,
					Tip:  "Scanning for connected drives...",
				}, nil)
			})
			return
		}

		mui.Column(c).Fill().Center().Padding(60).Gap(16).Children(func() {
			display.Empty(c, display.EmptyProps{
				Icon:        lucide.HardDrive(),
				Title:       "No Drives Connected",
				Description: "Connect an external storage drive, SSD, or USB volume to begin cataloging.",
			})

			action.Button(c, "Scan for Drives", action.ButtonProps{
				Variant: action.Primary,
				Size:    lobetheme.Middle,
			}).OnClick(func() {
				s.actions.Refresh()
				s.Reload()
			})
		})
		return
	}

	// 3. Drive Cards Grid
	mui.Row(c).Wrap().Gap(20).Children(func() {
		for _, drive := range drives {
			s.driveCard(c, tk, drive)
		}
	})
}

func (s *Screen) driveCard(c *mui.Context, tk *lobetheme.Tokens, drive domain.DriveInfo) {
	c.Key(fmt.Sprintf("drive-%s", drive.ID))

	isMounted := drive.Status == "mounted" || drive.Path != ""

	mui.Column(c).Width(320).Padding(20).Radius(16).
		Background(tk.BgContainer).Border(1, tk.BorderSecondary).
		OnClick(func() {
			if isMounted {
				s.actions.SelectDrive(drive.ID)
			}
		}).Children(func() {
		// Top row: Drive Icon, Name, and Mount Tag
		mui.Row(c).AlignItems(mui.Center).Gap(12).Children(func() {
			// Drive squircle icon container
			mui.Column(c).Width(42).Height(42).Radius(12).
				Background(tk.SuccessBg).Center().Children(func() {
				if drive.Type == "external" || drive.Type == "removable" {
					mui.Icon(c, lucide.Usb()).FontSize(20).TextColor(tk.Success)
				} else {
					mui.Icon(c, lucide.HardDrive()).FontSize(20).TextColor(tk.Success)
				}
			})

			// Drive Name & Size
			mui.Column(c).Grow(1).Gap(2).Children(func() {
				driveName := drive.Name
				if driveName == "" {
					driveName = drive.Path
				}
				mui.Text(c, driveName).FontSize(14).FontWeight(700).TextColor(tk.Text).SingleLine()
				mui.Text(c, formatBytes(drive.TotalBytes)).FontSize(11).TextColor(tk.TextTertiary)
			})

			// Mount status tag
			if isMounted {
				display.Tag(c, "Mounted", display.TagProps{
					Size:    lobetheme.Small,
					Color:   display.TagSuccess,
					Variant: display.TagFilled,
				})
			} else {
				display.Tag(c, "Unmounted", display.TagProps{
					Size: lobetheme.Small,
				})
			}
		})

		// Storage capacity bar
		mui.Column(c).Padding(16, 0, 0).Gap(6).Children(func() {
			usedPercent := float32(drive.UsedPercentage)
			if usedPercent == 0 && drive.TotalBytes > 0 && drive.FreeBytes > 0 {
				usedPercent = float32(float64(drive.TotalBytes-drive.FreeBytes) / float64(drive.TotalBytes) * 100)
			}

			status := feedback.ProgressNormal
			if usedPercent > 80 {
				status = feedback.ProgressException
			} else if usedPercent > 50 {
				status = feedback.ProgressActive
			}

			feedback.Progress(c, usedPercent, feedback.ProgressProps{
				Size:     lobetheme.Small,
				Status:   status,
				ShowInfo: false,
			})

			mui.Row(c).Children(func() {
				mui.Text(c, fmt.Sprintf("%.0f%% used", usedPercent)).
					FontSize(10).TextColor(tk.TextTertiary)
				mui.Box(c).Grow(1)
				mui.Text(c, fmt.Sprintf("%s total", formatBytes(drive.TotalBytes))).
					FontSize(10).TextColor(tk.TextTertiary)
			})
		})
	})
}
