package discover

import (
	"strconv"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// scannerStatus shows live scan telemetry and the pause/stop controls.
func (s *Screen) scannerStatus(c *mui.Context, tk *lobetheme.Tokens) {
	advancedCard(c, tk, func() {
		s.mu.RLock()
		paused, scanning := s.isPaused, s.isScanning
		progress := s.scanProg
		rules := len(protectedIgnoreRules) + len(s.ignoreRules)
		s.mu.RUnlock()

		mui.Text(c, "Scanner Status").FontSize(14).FontWeight(600)

		state, color := "IDLE", display.TagGray
		switch {
		case paused:
			state, color = "PAUSED", display.TagWarning
		case scanning:
			state, color = "SCANNING", display.TagBlue
		}
		statusRow(c, tk, "State", func() {
			display.Tag(c, state, display.TagProps{Size: lobetheme.Small, Color: color})
		})

		checked := 0
		if progress.IsRunning {
			checked = progress.FilesScanned
		}
		statusRow(c, tk, "Checked Files", func() {
			mui.Text(c, comma(checked)).FontSize(13).Font("monospace")
		})

		statusRow(c, tk, "Active Ignore Rules", func() {
			mui.Text(c, comma(rules)+" rules").FontSize(13).TextColor(tk.Warning)
		})

		cataloged := 0
		if s.catalogCount != nil {
			cataloged = s.catalogCount()
		}
		statusRow(c, tk, "SQLite Cataloged", func() {
			mui.Text(c, comma(cataloged)).FontSize(13).TextColor(tk.Success)
		})

		mui.Row(c).Gap(8).Children(func() {
			label := "Pause"
			if paused {
				label = "Resume"
			}
			action.Button(c, label, action.ButtonProps{Size: lobetheme.Small}).
				Disabled(!scanning).OnClick(func() {
				if paused {
					s.resumeScan()
				} else {
					s.pauseScan()
				}
			})
			action.Button(c, "Stop Scan", action.ButtonProps{Size: lobetheme.Small}).
				Disabled(!scanning).OnClick(func() { s.stopScan() })
		})
	})
}

func statusRow(c *mui.Context, tk *lobetheme.Tokens, label string, value func()) {
	mui.Row(c).Gap(12).AlignItems(mui.Center).Children(func() {
		mui.Text(c, label).FontSize(12).TextColor(tk.TextSecondary).Grow(1)
		value()
	})
}

func comma(value int) string {
	text := strconv.Itoa(value)
	if len(text) <= 3 {
		return text
	}
	out := make([]byte, 0, len(text)+len(text)/3)
	for index, digit := range text {
		if index > 0 && (len(text)-index)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(digit))
	}
	return string(out)
}
