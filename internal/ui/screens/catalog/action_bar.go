package catalog

import (
	"fmt"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

func (s *Screen) actionBar(c *mui.Context) {
	count := s.selection.Len()
	if count == 0 {
		return
	}
	tk := lobetheme.TokensOf(c)
	mui.Row(c).Shrink(0).Gap(8).Padding(10, 14).Radius(tk.BorderRadius).
		Background(tk.BgElevated).AlignItems(mui.Center).Children(func() {
		mui.Text(c, fmt.Sprintf("%d selected", count)).FontWeight(600)
		mui.Box(c).Grow(1)
		if action.Button(c, "Move to Album...", action.ButtonProps{
			Variant: action.Primary, Size: lobetheme.Small,
		}).Clicked() {
			s.actions.AssignToAlbum(s.SelectedIDs())
		}
		if action.Button(c, "Remove from Album", action.ButtonProps{Size: lobetheme.Small}).Clicked() {
			s.actions.RemoveFromAlbum(s.SelectedIDs())
		}
		remove := action.Button(c, "Delete Selected", action.ButtonProps{Size: lobetheme.Small})
		remove.TextColor(tk.Error)
		if remove.Clicked() {
			s.actions.Delete(s.SelectedIDs())
			s.selection.Clear()
		}
		if action.ActionIcon(c, lucide.X(), action.ActionIconProps{
			Size: lobetheme.Small, Title: "Clear selection",
		}).Clicked() {
			s.selection.Clear()
		}
	})
}
