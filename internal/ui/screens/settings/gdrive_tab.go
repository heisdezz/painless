package settings

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

// syncGdrive marks the credentials dirty whenever the drafts drift from
// the controller's saved values. Drafts are never overwritten here so
// mid-edit text is preserved; New seeds them from the controller.
func (s *Screen) syncGdrive() {
	if s.gdrive == nil {
		return
	}
	s.savedCreds, s.savedFolder = s.gdrive.Credentials()
	s.dirtyCredentials = s.credentialsDraft != s.savedCreds ||
		s.folderDraft != s.savedFolder
}

// gdriveTab renders the service-account credentials, connection test and
// manual backup controls.
func (s *Screen) gdriveTab(c *mui.Context) {
	if s.gdrive == nil {
		emptyController(c, lucide.CloudUpload(), "Google Drive unavailable",
			"No cloud backup controller is wired into this build.")
		return
	}
	tk := lobetheme.TokensOf(c)
	s.syncGdrive()

	mui.Column(c).Fill().Gap(12).Children(func() {
		card(c, "Service account", func() {
			mui.Text(c, "Credentials JSON").FontSize(12).TextColor(tk.TextSecondary)
			field.TextArea(c, &s.credentialsDraft, field.TextAreaProps{
				Placeholder: "Paste the service-account JSON key here",
				MinRows:     5,
				MaxRows:     10,
			})

			mui.Text(c, "Destination folder ID").FontSize(12).TextColor(tk.TextSecondary)
			field.Input(c, &s.folderDraft, field.InputProps{
				Placeholder: "Google Drive folder ID",
			})

			if s.dirtyCredentials {
				mui.Row(c).Shrink(0).Gap(8).AlignItems(mui.Center).Children(func() {
					display.Tag(c, "Unsaved changes", display.TagProps{
						Color: display.TagWarning,
						Size:  lobetheme.Small,
					})
					action.Button(c, "Save credentials", action.ButtonProps{Size: lobetheme.Small}).
						OnClick(func() {
							s.gdrive.SetCredentials(s.credentialsDraft, s.folderDraft)
							s.raise(domain.LogLevelInfo, "Credentials saved",
								"Run a connection test to verify them.")
						})
				})
			}
		})

		card(c, "Connection", func() {
			mui.Row(c).Shrink(0).Gap(8).AlignItems(mui.Center).Children(func() {
				action.Button(c, "Test connection", action.ButtonProps{Size: lobetheme.Small}).
					OnClick(func() { s.runConnectionTest(c) })
				s.connectionStatus(c)
			})

			mui.Row(c).Shrink(0).Gap(8).AlignItems(mui.Center).Children(func() {
				label := "Backup library to Google Drive"
				if s.backupRunning {
					label = "Uploading…"
				}
				action.Button(c, label, action.ButtonProps{Variant: action.Primary, Size: lobetheme.Small}).
					OnClick(func() { s.runCloudBackup(c) })
				if s.backupRunning {
					feedback.Spin(c, feedback.SpinProps{Size: lobetheme.Small}, func() {})
				}
			})
		})
	})
}

// connectionStatus shows the spinner while testing, else the outcome of
// the latest test.
func (s *Screen) connectionStatus(c *mui.Context) {
	if s.testRunning {
		feedback.Spin(c, feedback.SpinProps{Size: lobetheme.Small}, func() {})
		return
	}
	if s.testMessage == "" {
		return
	}
	color := display.TagSuccess
	if s.testFailed {
		color = display.TagError
	}
	display.Tag(c, s.testMessage, display.TagProps{
		Color: color,
		Size:  lobetheme.Small,
	})
}

// runConnectionTest validates the credentials off the UI thread.
func (s *Screen) runConnectionTest(c *mui.Context) {
	if s.testRunning {
		return
	}
	s.testRunning, s.testMessage, s.testFailed = true, "", false
	services := c.Services()
	go func() {
		result := &connectionResult{failed: false, message: "Connection OK"}
		if err := s.gdrive.TestConnection(); err != nil {
			result = &connectionResult{failed: true, message: err.Error()}
		}
		s.mu.Lock()
		s.testResult = result
		s.mu.Unlock()
		services.Invalidate()
	}()
}

// runCloudBackup uploads a fresh snapshot off the UI thread.
func (s *Screen) runCloudBackup(c *mui.Context) {
	if s.backupRunning {
		return
	}
	s.backupRunning = true
	services := c.Services()
	go func() {
		message, err := s.gdrive.BackupNow()
		if err != nil {
			s.raise(domain.LogLevelError, "Google Drive backup failed", err.Error())
		} else {
			s.raise(domain.LogLevelInfo, "Google Drive backup done", message)
		}
		s.mu.Lock()
		s.cloudDone = true
		s.mu.Unlock()
		services.Invalidate()
	}()
}
