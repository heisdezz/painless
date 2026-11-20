package settings

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// generalTab shows system-wide display toggles.
func (s *Screen) generalTab(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	mui.Column(c).Fill().Gap(16).Padding(12).Children(func() {
		card(c, "General", func() {
			mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
				mui.Column(c).Grow(1).Gap(2).Children(func() {
					mui.Text(c, "Display thumbnails").FontSize(13)
					mui.Text(c, "Catalog, dashboard and album cards show preview images.").
						FontSize(12).TextColor(tk.TextSecondary)
				})
				choice.Switch(c, &s.thumbsOn, choice.SwitchProps{Size: lobetheme.Small})
			})
		})
	})
}

// syncThumbnails pushes the thumbnails toggle into the store when the switch
// moved. Lobe inputs mutate their bound fields in place, so side effects run
// by comparing against the last applied value — the same pattern syncMobile
// uses.
func (s *Screen) syncThumbnails() {
	if s.store == nil {
		return
	}
	if s.thumbsOn != s.thumbsLastOn {
		s.thumbsLastOn = s.thumbsOn
		s.store.SetThumbnailsEnabled(s.thumbsOn)
	}
}
