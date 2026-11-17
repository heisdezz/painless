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
		}).Width(280)
		action.Segmented(c, &s.filter, filterOptions, action.SegmentedProps{Size: lobetheme.Small})

		mui.Box(c).Grow(1)
		total := 0
		if s.page != nil {
			total = s.page.Total
		}
		display.Tag(c, fmt.Sprintf("%d items", total), display.TagProps{
			Size:  lobetheme.Small,
			Color: display.TagBlue,
		}).TextColor(tk.TextSecondary)

		selected := "date_desc"
		if s.sort >= 0 && s.sort < len(sortOptions) {
			selected = sortOptions[s.sort].Value
		}
		field.Select(c, &selected, field.SelectProps{
			Options:     sortOptions,
			Placeholder: "Sort by",
			Variant:     field.InputFilled,
			Size:        lobetheme.Small,
		})
		for index, option := range sortOptions {
			if option.Value == selected {
				s.sort = index
				break
			}
		}
	})
}
