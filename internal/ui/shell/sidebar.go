package shell

import (
	"fmt"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/state"
	uitheme "painless/internal/ui/theme"
)

var navRowTransition = mui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true}

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
		mui.Column(c).Padding(12, 12).Gap(6).Children(func() {
			active := string(s.store.State.Screen())
			for _, item := range navigationItems() {
				if s.navRow(c, tk, item, active == string(item.screen), nav) {
					s.store.Navigate(item.screen)
				}
			}
		})
	})
}

// navRow is a borderless, single-line row in Ink Secondary that brightens on
// hover and takes a Spotlight-toned fill when active. When the rail collapses,
// the row narrows to a centered icon and the label fades with the width.
func (s *Shell) navRow(c *mui.Context, tk *lobetheme.Tokens, item navItem, active bool, nav layout.SideNavState) bool {
	collapse := clamp01(nav.Collapse)
	rail := nav.RailWidth
	if rail <= 0 {
		rail = 58
	}
	selected := active
	row := mui.ButtonBase(c).Label(item.label).Row().Height(32).AlignItems(mui.Center).
		Justify(mui.Start).Gap(lerp(8, 0, collapse)).Radius(tk.BorderRadiusSM).FontSize(tk.FontSizeSM).
		Padding(0, lerp(12, 0, collapse)).
		Margin(0, 0, 0, lerp(0, (rail-32)/2, collapse)).Transition(navRowTransition)

	glyph, ink := tk.TextTertiary, tk.TextSecondary
	switch {
	case selected:
		row.Background(tk.FillSecondary)
		glyph, ink = tk.Text, tk.Text
	case row.Hovered():
		row.Background(tk.FillTertiary)
		glyph, ink = tk.TextLabel, tk.Text
	}
	row.TextColor(ink)

	clicked := false
	row.Children(func() {
		mui.Row(c).Size(18, 18).Shrink(0).Center().TextColor(glyph).Children(func() {
			mui.Icon(c, item.icon()).FontSize(16)
		})
		label := mui.Text(c, item.label).FontSize(tk.FontSizeSM).SingleLine().
			Opacity(1 - collapse)
		if selected {
			label.FontWeight(600)
		}
	})
	clicked = row.Clicked()
	return clicked
}

// storageGauge shows the active drive's usage as a quiet, semantic gauge.
func (s *Shell) storageGauge(c *mui.Context, tk *lobetheme.Tokens, nav layout.SideNavState) {
	drive, ok := s.store.State.ActiveDrive()
	mui.Column(c).Shrink(0).Padding(12, 12).Gap(6).Children(func() {
		mui.Box(c).Height(1).FillWidth().Background(tk.Border).Margin(4, 0)
		if nav.Expanded {
			mui.Text(c, "STORAGE").FontSize(tk.FontSizeSM - 2).FontWeight(800).
				LetterSpacing(1).TextColor(tk.TextTertiary)
		}
		if !ok {
			mui.Text(c, "No drive connected").FontSize(tk.FontSizeSM - 1).
				TextColor(tk.TextTertiary).SingleLine()
			return
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
				FontSize(tk.FontSizeSM - 1).TextColor(tk.TextTertiary).SingleLine()
		}
	})
}

func clamp01(value float32) float32 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func lerp(from, to, t float32) float32 {
	return from + (to-from)*t
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
