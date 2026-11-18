package settings

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

// backupTab renders the database health card and the snapshot list.
func (s *Screen) backupTab(c *mui.Context) {
	if s.backups == nil {
		emptyController(c, lucide.DatabaseBackup(), "Backups unavailable",
			"No database controller is wired into this build.")
		return
	}
	tk := lobetheme.TokensOf(c)

	mui.Column(c).Fill().Gap(12).Children(func() {
		s.healthCard(c, tk)
		s.snapshotsCard(c, tk)

		if s.restorePath != "" {
			path := s.restorePath
			if feedback.Modal(c, &s.restoreOpen, feedback.ModalProps{
				Title:      "Restore this backup?",
				OkText:     "Restore backup",
				CancelText: "Cancel",
			}, func() {
				mui.Text(c, "The live library database is overwritten with the "+
					"selected snapshot. Media files on the drive are not touched.").
					FontSize(13).TextColor(tk.TextSecondary)
				mui.Text(c, path).FontSize(12).TextColor(tk.TextTertiary)
			}) {
				s.restoreOpen = false
				s.restorePath = ""
				s.startRestore(path, c)
			}
		}
	})
}

// healthCard shows database location, size and integrity with actions.
func (s *Screen) healthCard(c *mui.Context, tk *lobetheme.Tokens) {
	card(c, "Database health", func() {
		fieldRow(c, "Location", s.backups.DBPath())
		fieldRow(c, "Size", formatBytes(s.backups.DBSizeBytes()))

		mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "Integrity").FontSize(12).TextColor(tk.TextSecondary).Width(110).Shrink(0)
			switch {
			case s.integrity == "":
				display.Tag(c, "Not verified", display.TagProps{Color: display.TagGray, Size: lobetheme.Small})
			case s.integrityOK:
				display.Tag(c, "OK", display.TagProps{Color: display.TagSuccess, Size: lobetheme.Small})
			default:
				display.Tag(c, "Issues found", display.TagProps{Color: display.TagError, Size: lobetheme.Small})
			}
			if s.integrity != "" && !s.integrityOK {
				mui.Text(c, s.integrity).FontSize(12).TextColor(tk.TextTertiary).Grow(1)
			}
		})

		mui.Row(c).Shrink(0).Gap(8).Children(func() {
			action.Button(c, "Verify integrity", action.ButtonProps{Size: lobetheme.Small}).OnClick(func() {
				report, ok := s.backups.IntegrityCheck()
				s.integrity, s.integrityOK = report, ok
				if ok {
					s.raise(domain.LogLevelInfo, "Integrity check passed", report)
				} else {
					s.raise(domain.LogLevelError, "Integrity check failed", report)
				}
			})
			label := "Create backup"
			if s.backupBusy {
				label = "Backing up…"
			}
			action.Button(c, label, action.ButtonProps{Variant: action.Primary, Size: lobetheme.Small}).
				OnClick(func() { s.startBackup(c) })
		})
	})
}

// startBackup snapshots the database off the UI thread.
func (s *Screen) startBackup(c *mui.Context) {
	if s.backupBusy {
		return
	}
	s.backupBusy = true
	services := c.Services()
	go func() {
		path, err := s.backups.CreateBackup()
		if err != nil {
			s.raise(domain.LogLevelError, "Backup failed", err.Error())
		} else {
			s.raise(domain.LogLevelInfo, "Backup created", path)
		}
		s.mu.Lock()
		s.backupDone = true
		s.mu.Unlock()
		services.Invalidate()
	}()
}

// startRestore copies a snapshot over the live database off the UI
// thread.
func (s *Screen) startRestore(path string, c *mui.Context) {
	services := c.Services()
	go func() {
		if err := s.backups.RestoreBackup(path); err != nil {
			s.raise(domain.LogLevelError, "Restore failed", err.Error())
			return
		}
		s.raise(domain.LogLevelInfo, "Backup restored",
			"The library has been reopened with the restored backup.")
		services.Invalidate()
	}()
}

// snapshotsCard lists the snapshots in .media_backups, newest first.
func (s *Screen) snapshotsCard(c *mui.Context, tk *lobetheme.Tokens) {
	card(c, "Snapshots", func() {
		backups, err := s.backups.ListBackups()
		if err != nil {
			mui.Text(c, "Could not list backups: "+err.Error()).
				FontSize(12).TextColor(tk.TextTertiary)
			return
		}
		if len(backups) == 0 {
			display.Empty(c, display.EmptyProps{
				Icon:        lucide.DatabaseBackup(),
				Title:       "No backups yet",
				Description: "Create a snapshot above; it lands in .media_backups next to the library.",
			})
			return
		}
		mui.Column(c).Shrink(0).Gap(2).Children(func() {
			for _, b := range backups {
				backup := b
				mui.Row(c).Shrink(0).Gap(12).Padding(6, 8).Radius(6).
					AlignItems(mui.Center).Children(func() {
					mui.Column(c).Width(280).Shrink(0).Gap(1).Children(func() {
						mui.Text(c, backup.Name).FontSize(13).SingleLine()
						mui.Text(c, backup.CreatedAt).FontSize(11).TextColor(tk.TextTertiary).SingleLine()
					})
					mui.Text(c, formatBytes(backup.SizeBytes)).
						FontSize(12).TextColor(tk.TextSecondary).Width(80).Shrink(0)
					mui.Box(c).Grow(1)
					action.Button(c, "Restore", action.ButtonProps{Size: lobetheme.Small}).OnClick(func() {
						s.restorePath, s.restoreOpen = backup.Path, true
					})
				})
			}
		})
	})
}
