package catalog

import (
	"fmt"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

var sortOptions = []field.SelectOption{
	{Value: "date_desc", Label: "Newest first"},
	{Value: "date_asc", Label: "Oldest first"},
	{Value: "name", Label: "Name A-Z"},
	{Value: "size", Label: "File size"},
}

var filterOptions = []string{"All", "Photos", "Videos"}

func (s *Screen) filterBar(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
		field.SearchBar(c, &s.query, field.SearchBarProps{
			Placeholder: "Search media",
			ShortKey:    "mod+f",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		}).Grow(1).Basis(220).MinWidth(140).MaxWidth(360)
		action.Segmented(c, &s.filter, filterOptions, action.SegmentedProps{Size: lobetheme.Small})

		mui.Box(c).Grow(1)
		if s.fetchKey != "" && s.page.Total == 0 {
			display.Tag(c, "Loading…", display.TagProps{
				Size:  lobetheme.Small,
				Color: display.TagBlue,
			}).TextColor(tk.TextSecondary).Shrink(0)
		} else {
			display.Tag(c, fmt.Sprintf("%d items", s.page.Total), display.TagProps{
				Size:  lobetheme.Small,
				Color: display.TagBlue,
			}).TextColor(tk.TextSecondary).Shrink(0)
		}

		selected := "date_desc"
		if s.sort >= 0 && s.sort < len(sortOptions) {
			selected = sortOptions[s.sort].Value
		}
		field.Select(c, &selected, field.SelectProps{
			Options:     sortOptions,
			Placeholder: "Sort by",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		}).Shrink(0)
		for index, option := range sortOptions {
			if option.Value == selected {
				s.sort = index
				break
			}
		}
	})
}
