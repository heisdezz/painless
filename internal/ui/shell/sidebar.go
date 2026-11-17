package shell

import (
	"fmt"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/state"
	uitheme "painless/internal/ui/theme"
)

type navItem struct {
	screen state.Screen
	label  string
	icon   func() *mui.SVG
}

// navigationItems returns the sidebar entries in navigation order.
func navigationItems() []navItem {
	return []navItem{
		{state.ScreenDashboard, "Home", lucide.House},
		{state.ScreenCatalog, "Library", lucide.FolderHeart},
		{state.ScreenAlbums, "Albums", lucide.Images},
		{state.ScreenDiscover, "Discover", lucide.ScanSearch},
		{state.ScreenSettings, "Settings", lucide.Settings},
		{state.ScreenLogs, "Logs", lucide.ScrollText},
	}
}

func (s *Shell) sidebar(c *mui.Context, tk *lobetheme.Tokens) {
	layout.DraggableSideNav(c, &s.navWidth, &s.navExpanded, layout.DraggableSideNavProps{
		MinWidth:                58,
		MaxWidth:                320,
		HighlightHandle:         true,
		ShowHandleWhenCollapsed: true,
		Background:              tk.BgLayout,
		Footer:                  func(nav layout.SideNavState) { s.storageGauge(c, tk, nav) },
	}, func(nav layout.SideNavState) {
		mui.Column(c).Padding(14, 12).Children(func() {
			items := navigationItems()
			rows := make([]display.ListItem, 0, len(items))
			for _, item := range items {
				rows = append(rows, display.ListItem{
					Key:   string(item.screen),
					Label: item.label,
					Icon:  item.icon(),
				})
			}
			if clicked := display.List(c, display.ListProps{
				Items:     rows,
				Active:    string(s.store.State.Screen()),
				Collapse:  nav.Collapse,
				RailWidth: nav.RailWidth - 24,
			}); clicked != "" {
				s.store.Navigate(state.Screen(clicked))
			}
		})
	})
}

// storageGauge shows the active drive's usage and free space in the rail.
func (s *Shell) storageGauge(c *mui.Context, tk *lobetheme.Tokens, nav layout.SideNavState) {
	drive, ok := s.store.State.ActiveDrive()
	mui.Column(c).Shrink(0).Padding(12, 12).Gap(6).Children(func() {
		if !ok {
			mui.Text(c, "No drive connected").FontSize(12).TextColor(tk.TextTertiary).SingleLine()
			return
		}
		if nav.Expanded {
			mui.Text(c, "Storage").FontSize(12).TextColor(tk.TextSecondary)
		}
		feedback.Progress(c, float32(drive.UsedPercentage), feedback.ProgressProps{
			Size:     lobetheme.Small,
			Color:    uitheme.UsageColor(drive.UsedPercentage, tk),
			Label:    drive.Name,
			ShowInfo: nav.Expanded,
		})
		if nav.Expanded {
			mui.Text(c, fmt.Sprintf("%s free of %s",
				formatBytes(drive.FreeBytes), formatBytes(drive.TotalBytes))).
				FontSize(11).TextColor(tk.TextTertiary).SingleLine()
		}
	})
}

// formatBytes renders a byte count in human-readable binary units.
func formatBytes(bytes uint64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	unit := -1
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
