package shell

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

func (s *Shell) topBar(c *mui.Context, tk *lobetheme.Tokens) {
	// The window hides its title bar and its window buttons, so this row is
	// the title bar: it drags the window, a double click on it zooms, and
	// the controls sit at its right end. c.TitleBar() is zero then; its
	// insets are kept so a platform that still reports a system band
	// leaves the view clear of it.
	bar := c.TitleBar()
	mui.Row(c).Height(max(titleBarHeight, bar.Height)).Shrink(0).
		Padding(0, bar.Right+16, 0, bar.Left+16).Gap(12).AlignItems(mui.Center).
		DragWindow().Children(func() {
		mui.Icon(c, lucide.HardDrive()).FontSize(20).TextColor(tk.Primary)
		mui.Text(c, "painless").FontSize(18).FontWeight(600)
		mui.Box(c).Grow(1)
		mui.Text(c, "Active drive").FontSize(12).TextColor(tk.TextSecondary)
		s.driveSelect(c, tk)
		refresh := action.ActionIcon(c, lucide.RefreshCw(), action.ActionIconProps{
			Size:  lobetheme.Small,
			Title: "Refresh drives",
		})
		if refresh.Clicked() && s.refresh != nil {
			s.refresh()
		}
		s.windowControls(c)
	})
}

// windowControls draws the minimize, maximize and close buttons the view
// takes over from the system, as bare quiet icons: instrument-panel
// restraint, no boxed chrome. The maximize button is marked so Windows 11
// shows its snap layouts over it as it would over its own.
func (s *Shell) windowControls(c *mui.Context) {
	if s.win == nil {
		return
	}
	mui.Row(c).Gap(4).AlignItems(mui.Center).Children(func() {
		if action.ActionIcon(c, lucide.Minus(), action.ActionIconProps{
			Size:  lobetheme.Small,
			Title: "Minimize",
		}).Clicked() {
			s.win.Minimize()
		}

		icon, title := lucide.Square(), "Maximize"
		if s.maximized {
			icon, title = lucide.Copy(), "Restore"
		}
		maximize := action.ActionIcon(c, icon, action.ActionIconProps{
			Size:  lobetheme.Small,
			Title: title,
		}).MaximizeButton()
		if maximize.Clicked() {
			s.win.ToggleMaximize()
		}

		if action.ActionIcon(c, lucide.X(), action.ActionIconProps{
			Size:  lobetheme.Small,
			Title: "Close",
		}).Clicked() {
			s.win.Close()
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
		options = append(options, field.SelectOption{
			Value:    drive.ID,
			Label:    drive.Name,
			Disabled: drive.IsSystemBoot(),
		})
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
