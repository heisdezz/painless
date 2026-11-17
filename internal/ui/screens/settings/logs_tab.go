package settings

import (
	"math"
	"os"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/logging"
)

// logLevelOptions backs the severity segmented control; index 0 keeps
// every level.
var logLevelOptions = []string{"All", "Info", "Warn", "Error"}

// logLevelFor maps the segmented index to a logging.Filter level; the
// empty level keeps everything.
func logLevelFor(i int) domain.LogLevel {
	switch i {
	case 1:
		return domain.LogLevelInfo
	case 2:
		return domain.LogLevelWarn
	case 3:
		return domain.LogLevelError
	}
	return ""
}

// logsTab renders the live log console: filter bar and severity-colored
// entries, refreshed every frame from the ring buffer.
func (s *Screen) logsTab(c *mui.Context) {
	if s.logs == nil {
		emptyController(c, lucide.ScrollText(), "Logs unavailable",
			"No logger is wired into this build.")
		return
	}
	tk := lobetheme.TokensOf(c)

	entries := s.logs.Query(logging.Filter{
		Search: s.logSearch,
		Level:  logLevelFor(s.logLevel),
		Limit:  500,
	})
	s.followNewEntries(len(entries))

	mui.Column(c).Fill().Gap(12).Children(func() {
		s.logControls(c)
		s.logList(c, tk, entries)
	})
}

// logControls draws the search, level filter, auto-scroll toggle and
// export button.
func (s *Screen) logControls(c *mui.Context) {
	mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
		mui.Box(c).Width(260).Shrink(0).Children(func() {
			field.SearchBar(c, &s.logSearch, field.SearchBarProps{
				Placeholder: "Filter messages…",
				Size:        lobetheme.Small,
			})
		})
		action.Segmented(c, &s.logLevel, logLevelOptions, action.SegmentedProps{})
		mui.Box(c).Grow(1)

		mui.Row(c).Shrink(0).Gap(6).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "Auto-scroll").FontSize(12)
			choice.Switch(c, &s.autoScroll, choice.SwitchProps{Size: lobetheme.Small})
		})
		action.Button(c, "Export logs…", action.ButtonProps{Size: lobetheme.Small}).OnClick(func() {
			s.exportLogs()
		})
	})
}

// logList draws the filtered entries, newest at the bottom.
func (s *Screen) logList(c *mui.Context, tk *lobetheme.Tokens, entries []domain.LogEntry) {
	if len(entries) == 0 {
		mui.Scroll(c).Grow(1).Basis(0).Children(func() {
			mui.Column(c).Fill().Center().Padding(48).Children(func() {
				display.Empty(c, display.EmptyProps{
					Icon:        lucide.ScrollText(),
					Title:       "No matching entries",
					Description: "Adjust the search or level filter, or wait for new activity.",
				})
			})
		})
		return
	}

	mui.Scroll(c).Grow(1).Basis(0).TrackScroll(&s.logScroll).Children(func() {
		mui.Column(c).Fill().Gap(2).Children(func() {
			for _, entry := range entries {
				e := entry
				mui.Row(c).Shrink(0).Gap(8).Padding(3, 8).Radius(4).
					AlignItems(mui.Start).Children(func() {
					mui.Text(c, e.Timestamp.Local().Format("15:04:05")).
						FontSize(12).TextColor(tk.TextTertiary).Shrink(0)
					levelTag(c, e.Level).Shrink(0)
					if e.Context != "" {
						mui.Text(c, e.Context).FontSize(12).
							TextColor(tk.TextSecondary).Width(120).Shrink(0).SingleLine()
					}
					mui.Text(c, e.Message).FontSize(12).Grow(1)
				})
			}
		})
	})
}

// followNewEntries scrolls to the bottom when fresh entries arrive while
// auto-scroll is on.
func (s *Screen) followNewEntries(count int) {
	if s.autoScroll && int64(count) != s.seenVersion {
		s.logScroll.Y = float32(math.MaxFloat32)
	}
	s.seenVersion = int64(count)
}

// exportLogs writes the currently filtered entries to a timestamped text
// file in the system temp directory.
func (s *Screen) exportLogs() {
	file, err := os.CreateTemp("", "painless-logs-*.txt")
	if err != nil {
		s.raise(domain.LogLevelError, "Export failed", err.Error())
		return
	}
	name := file.Name()
	filter := logging.Filter{Search: s.logSearch, Level: logLevelFor(s.logLevel)}
	if err := s.logs.Export(file, filter); err != nil {
		file.Close()
		s.raise(domain.LogLevelError, "Export failed", err.Error())
		return
	}
	if err := file.Close(); err != nil {
		s.raise(domain.LogLevelError, "Export failed", err.Error())
		return
	}
	s.raise(domain.LogLevelInfo, "Logs exported",
		strings.TrimSpace(name))
}
