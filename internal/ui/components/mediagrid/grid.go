// Package mediagrid renders media tiles in the two layouts the app uses: a
// virtualized library grid and the dashboard's wrapped recent-media strip.
// Both use the same fixed-size shared card so previews stay consistent.
package mediagrid

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/overlay"

	"painless/internal/ui/components/mediacard"
)

// Item is one tile's data and callbacks.
type Item struct {
	Key           any
	Label         string
	Name          string
	SizeLabel     string
	DurationLabel string
	FormatLabel   string
	Bitmap        *mui.Bitmap
	Selected      bool
	Menu          []overlay.MenuItem
	OnClick       func(mui.Modifiers)
	OnContext     func(string)
	// Reveal plays a staggered entrance; RevealIndex is this tile's slot.
	Reveal      *mediacard.Reveal
	RevealIndex int
}

func (item Item) render(c *mui.Context) {
	if item.Key != nil {
		c.Key(item.Key)
	}
	mediacard.Render(c, mediacard.Props{
		Width:         mediacard.CardWidth,
		Label:         item.Label,
		Name:          item.Name,
		SizeLabel:     item.SizeLabel,
		DurationLabel: item.DurationLabel,
		FormatLabel:   item.FormatLabel,
		Bitmap:        item.Bitmap,
		Selected:      item.Selected,
		Items:         item.Menu,
		OnClick:       item.OnClick,
		OnContext:     item.OnContext,
		Reveal:        item.Reveal,
		RevealIndex:   item.RevealIndex,
	})
}

// Grid builds the virtualized library grid. state carries scroll position and
// multi-selection; the cell size matches the shared card.
func Grid(c *mui.Context, state *mui.GridState, items []Item) mui.Element {
	return mui.GridView(c, state, len(items), mediacard.CardWidth, mediacard.CardHeight, func(i int) {
		items[i].render(c)
	})
}

// Strip builds the wrapped, fixed-size row used by the dashboard.
func Strip(c *mui.Context, items []Item) {
	mui.Row(c).Wrap().Gap(8).Children(func() {
		for _, item := range items {
			item.render(c)
		}
	})
}
