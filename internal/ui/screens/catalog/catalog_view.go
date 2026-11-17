package catalog

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

func (s *Screen) gridView(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	if s.loadErr != nil {
		mui.Column(c).Grow(1).Center().Children(func() {
			mui.Text(c, "Could not load media: "+s.loadErr.Error()).TextColor(tk.Error)
		})
		return
	}
	items := s.page.Items
	if len(items) == 0 {
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mui.Column(c).Fill().Center().Padding(24).Children(func() {
				display.Empty(c, display.EmptyProps{
					Icon:        lucide.FolderSearch(),
					Title:       "No media found",
					Description: "Try a different search or scan the drive to index its files.",
					Action: func() {
						if action.Button(c, "Start Scan", action.ButtonProps{Variant: action.Primary}).Clicked() {
							s.actions.StartScan()
						}
					},
				})
			})
		})
		return
	}
	mui.GridView(c, &s.grid, len(items), 160, 160, func(i int) {
		s.mediaCard(c, items[i])
	}).Grow(1)
}
