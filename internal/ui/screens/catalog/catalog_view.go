package catalog

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/ui/components/mediacard"
	"painless/internal/ui/components/mediagrid"
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
		if s.fetchKey != "" {
			s.skeletonGridView(c)
			return
		}
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
	cards := make([]mediagrid.Item, 0, len(items))
	for index, item := range items {
		card := s.mediaCard(index, item)
		card.Reveal, card.RevealIndex = s.reveal, index
		cards = append(cards, card)
	}
	grid := mediagrid.Grid(c, &s.grid, cards).Grow(1)
	// Double-click or Enter on a focused cell also opens the viewer.
	if grid.Submitted() && s.selected >= 0 && s.selected < len(items) {
		s.actions.OpenInViewer(items[s.selected], items)
	}
}

// skeletonGridView holds the grid's place while the first page loads: tiles
// the size and chrome of the real card, filled with sweeping skeleton blocks.
// It keeps its own scroll state so it never borrows the real grid's.
func (s *Screen) skeletonGridView(c *mui.Context) {
	mui.GridView(c, &s.skeletonGrid, 30, mediacard.CardWidth, mediacard.CardHeight, func(int) {
		mediacard.SkeletonTile(c)
	}).Grow(1).Label("catalog-loading")
}
