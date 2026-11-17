package shell

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

func (s *Shell) topBar(c *mui.Context, tk *lobetheme.Tokens) {
	mui.Row(c).Shrink(0).Padding(12, 16).Gap(12).AlignItems(mui.Center).Children(func() {
		mui.Icon(c, lucide.HardDrive()).FontSize(20).TextColor(tk.Primary)
		mui.Text(c, "painless").FontSize(18).FontWeight(600)
		mui.Box(c).Grow(1)
		mui.Text(c, "Active drive").FontSize(12).TextColor(tk.TextSecondary)
		s.driveSelect(c, tk)
		refresh := action.ActionIcon(c, lucide.RefreshCw(), action.ActionIconProps{
			Size:  lobetheme.Small,
			Title: "Refresh drives",
		})
		if refresh.Clicked() {
			s.toaster.Info("Scanning for drives", "Looking for removable volumes")
			if s.refresh != nil {
				s.refresh()
			}
		}
	})
}

// driveSelect binds to a local copy of the active drive ID. Lobe's Select
// writes the user's choice back into it during construction, so the change
// can be committed to the store in the same build pass.
func (s *Shell) driveSelect(c *mui.Context, tk *lobetheme.Tokens) {
	drives := s.store.State.Drives()
	if len(drives) == 0 {
		mui.Text(c, "No drive connected").FontSize(13).TextColor(tk.TextPlaceholder)
		return
	}

	options := make([]field.SelectOption, 0, len(drives))
	for _, drive := range drives {
		options = append(options, field.SelectOption{Value: drive.ID, Label: drive.Name})
	}
	selected := s.store.State.ActiveDriveID()
	field.Select(c, &selected, field.SelectProps{
		Options:     options,
		Placeholder: "Choose a drive",
		Size:        lobetheme.Small,
	})
	if selected != s.store.State.ActiveDriveID() {
		s.store.SetActiveDrive(selected)
	}
}

// SetRefreshHandler installs the action run by the refresh button. A drive
// service can use this to rescan; otherwise the button just reports that
// it looked for drives.
func (s *Shell) SetRefreshHandler(fn func()) { s.refresh = fn }
