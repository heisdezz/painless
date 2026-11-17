package discover

import (
	"strconv"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

var ignorePresets = []string{"temp", "cache", "raw", "backups", "archive"}

var protectedIgnoreRules = []string{"albums/", "node_modules/", ".* (hidden)", "lost+found"}

// ignoreList renders the folder/path ignorelist editor.
func (s *Screen) ignoreList(c *mui.Context, tk *lobetheme.Tokens) {
	advancedCard(c, tk, func() {
		mui.Row(c).Gap(10).AlignItems(mui.Center).Children(func() {
			mui.Icon(c, lucide.FolderMinus()).FontSize(18).TextColor(tk.Warning)
			mui.Text(c, "Folder & Path Ignorelist").FontSize(14).FontWeight(600).Grow(1)
			display.Tag(c, customCountLabel(len(s.ignoreRules)), display.TagProps{
				Size: lobetheme.Small, Color: display.TagWarning,
			})
			if action.Button(c, "Reset Defaults", action.ButtonProps{
				Variant: action.Text, Size: lobetheme.Small,
			}).Clicked() {
				s.ignoreRules = nil
			}
		})
		mui.Text(c, "The scanner bypasses matching folder names or path segments during discovery.").
			FontSize(12).TextColor(tk.TextSecondary)

		mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
			field.Input(c, &s.ignoreInput, field.InputProps{
				Placeholder: "Add folder or path to ignore",
				Size:        lobetheme.Small,
			})
			if action.Button(c, "+ Add", action.ButtonProps{Size: lobetheme.Small}).Clicked() {
				s.addIgnoreRule(s.ignoreInput)
			}
		})

		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "QUICK PRESETS").FontSize(11).TextColor(tk.TextTertiary)
			for _, preset := range ignorePresets {
				preset := preset
				if advancedButton(c, "+ "+preset, tk) {
					s.addIgnoreRule(preset)
				}
			}
		})

		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			for _, rule := range protectedIgnoreRules {
				display.Tag(c, rule, display.TagProps{
					Size: lobetheme.Small, Color: display.TagGray, Icon: lucide.Shield(),
				})
			}
			for index, rule := range s.ignoreRules {
				index, rule := index, rule
				display.Tag(c, rule, display.TagProps{
					Size: lobetheme.Small, Color: display.TagBlue, Closable: true,
					OnClose: func() { s.removeIgnoreRule(index) },
				})
			}
		})
	})
}

func customCountLabel(count int) string {
	if count == 1 {
		return "1 custom"
	}
	return itoa(count) + " custom"
}

func itoa(value int) string { return strconv.Itoa(value) }

func trimRule(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	return strings.TrimRight(pattern, `/\\`)
}

func (s *Screen) addIgnoreRule(pattern string) {
	pattern = trimRule(pattern)
	if pattern == "" {
		return
	}
	for _, existing := range s.ignoreRules {
		if existing == pattern {
			return
		}
	}
	s.ignoreRules = append(s.ignoreRules, pattern)
	s.ignoreInput = ""
}

func (s *Screen) removeIgnoreRule(index int) {
	if index < 0 || index >= len(s.ignoreRules) {
		return
	}
	s.ignoreRules = append(s.ignoreRules[:index], s.ignoreRules[index+1:]...)
}

// ignorePatterns returns the custom patterns for the scanner.
func (s *Screen) ignorePatterns() []string {
	return append([]string(nil), s.ignoreRules...)
}
