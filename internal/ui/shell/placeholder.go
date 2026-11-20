package shell

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/state"
)

type screenInfo struct {
	title       string
	description string
	emptyTitle  string
	icon        func() *mui.SVG
}

func screenInfos() map[state.Screen]screenInfo {
	return map[state.Screen]screenInfo{
		state.ScreenDashboard: {
			"Dashboard",
			"Overview of the active drive and recently added media.",
			"No drive selected",
			lucide.House,
		},
		state.ScreenCatalog: {
			"Media library",
			"Browse everything indexed on the active drive.",
			"No media indexed yet",
			lucide.FolderHeart,
		},
		state.ScreenAlbums: {
			"Albums",
			"Group media into albums and keep them organized on disk.",
			"No albums yet",
			lucide.Images,
		},
		state.ScreenDiscover: {
			"Discover & scan",
			"Scan a drive and review the staged changes before applying them.",
			"Nothing staged",
			lucide.ScanSearch,
		},
		state.ScreenSettings: {
			"Settings",
			"Backups, mobile pairing and application preferences.",
			"Nothing to configure",
			lucide.Settings,
		},
		state.ScreenLogs: {
			"System logs",
			"Diagnostics from the scanner, staging engine and services.",
			"No log entries",
			lucide.ScrollText,
		},
		state.ScreenViewer: {
			"Viewer",
			"Inspect a photo or play a video from the library.",
			"Nothing open",
			lucide.Image,
		},
	}
}

// content hosts the active screen. Screen packages are rendered once implemented,
// or show a titled placeholder until their task lands.
func (s *Shell) content(c *mui.Context, tk *lobetheme.Tokens) {
	screen := s.store.State.Screen()
	if !screen.Valid() {
		screen = state.ScreenDashboard
	}
	info := screenInfos()[screen]
	mui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
		if screen != state.ScreenAlbums && screen != state.ScreenDashboard && screen != state.ScreenViewer {
			mui.Column(c).Shrink(0).Padding(24, 28, 12).Gap(4).Children(func() {
				mui.Text(c, info.title).FontSize(22).FontWeight(600)
				mui.Text(c, info.description).FontSize(13).TextColor(tk.TextSecondary)
			})
		}
		// These screens paint their own headers (or none), so they take the
		// whole content area without the generic screen heading.
		if screen == state.ScreenAlbums && s.albums != nil {
			mui.Column(c).Grow(1).Basis(0).Children(func() { s.albums.View(c) })
			return
		}
		if screen == state.ScreenDashboard && s.dashboard != nil {
			mui.Column(c).Grow(1).Basis(0).Children(func() { s.dashboard.View(c) })
			return
		}
		if screen == state.ScreenViewer && s.viewer != nil {
			// Keyed so the stage is never mistaken for another screen's
			// element: leaving the viewer unmounts it and stops playback.
			mui.Column(c).Key("screen-viewer").Grow(1).Basis(0).Children(func() { s.viewer.View(c) })
			return
		}
		if screen == state.ScreenCatalog && s.catalog != nil {
			mui.Column(c).Grow(1).Basis(0).Children(func() { s.catalog.View(c) })
			return
		}
		if screen == state.ScreenDiscover && s.discover != nil {
			mui.Column(c).Grow(1).Basis(0).Children(func() { s.discover.View(c) })
			return
		}
		if screen == state.ScreenSettings && s.settings != nil {
			mui.Column(c).Grow(1).Basis(0).Children(func() { s.settings.View(c) })
			return
		}
		if screen == state.ScreenLogs && s.settings != nil {
			s.settings.ShowLogsTab()
			mui.Column(c).Grow(1).Basis(0).Children(func() { s.settings.View(c) })
			return
		}
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mui.Column(c).Fill().Center().Padding(24).Children(func() {
				display.Empty(c, display.EmptyProps{
					Icon:        info.icon(),
					Title:       info.emptyTitle,
					Description: "This screen is still being built. The app shell is ready to host it.",
				})
			})
		})
	})
}
