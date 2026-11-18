package settings

import (
	"strconv"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

// card draws a titled section shared by every settings tab: a quiet panel
// step with a 1px hairline and a small uppercase-style section header.
func card(c *mui.Context, title string, body func()) {
	tk := lobetheme.TokensOf(c)
	mui.Column(c).Shrink(0).MaxWidthPercent(100).Padding(16, 20).Gap(12).
		Background(tk.BgContainer).Border(1, tk.BorderSecondary).
		Radius(tk.BorderRadius).Children(func() {
		if title != "" {
			mui.Text(c, title).FontSize(tk.FontSizeSM - 2).FontWeight(800).
				LetterSpacing(1).TextColor(tk.TextTertiary)
		}
		body()
	})
}

// fieldRow is one label/value line inside a card.
func fieldRow(c *mui.Context, label, value string) {
	tk := lobetheme.TokensOf(c)
	mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
		mui.Text(c, label).FontSize(tk.FontSizeSM - 1).TextColor(tk.TextTertiary).
			Width(120).Shrink(0)
		mui.Text(c, value).FontSize(tk.FontSizeSM).Grow(1)
	})
}

// emptyController fills a tab whose backing service is not wired.
func emptyController(c *mui.Context, icon *mui.SVG, title, description string) {
	mui.Column(c).Fill().Center().Padding(48).Children(func() {
		display.Empty(c, display.EmptyProps{
			Icon:        icon,
			Title:       title,
			Description: description,
		})
	})
}

// levelTag maps a log level to its severity-colored tag.
func levelTag(c *mui.Context, level domain.LogLevel) mui.Element {
	color := display.TagInfo
	switch level {
	case domain.LogLevelWarn:
		color = display.TagWarning
	case domain.LogLevelError:
		color = display.TagError
	}
	return display.Tag(c, strings.ToUpper(string(level)), display.TagProps{
		Color: color,
		Size:  lobetheme.Small,
	})
}

// formatBytes renders a byte count in human-readable binary units.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	value := float64(bytes)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		value /= unit
		if value < unit {
			return strconv.FormatFloat(value, 'f', 1, 64) + " " + suffix
		}
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " EiB"
}
