package discover

import (
	"fmt"

	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// driveBanner shows the active volume's identity and mount point.
func (s *Screen) driveBanner(c *mui.Context, tk *lobetheme.Tokens) {
	if s.store == nil {
		return
	}
	drive, ok := s.store.State.ActiveDrive()
	if !ok {
		return
	}
	advancedCard(c, tk, func() {
		mui.Row(c).Gap(14).AlignItems(mui.Center).Children(func() {
			mui.Icon(c, lucide.HardDrive()).FontSize(26).TextColor(tk.Success)
			mui.Column(c).Grow(1).Gap(2).Children(func() {
				mui.Text(c, drive.Name).FontSize(15).FontWeight(600)
				mui.Row(c).Gap(8).Children(func() {
					mui.Text(c, advancedFormatBytes(drive.TotalBytes)).FontSize(12).TextColor(tk.TextSecondary)
					display.Tag(c, string(drive.Type), display.TagProps{Size: lobetheme.Small, Color: display.TagGreen})
				})
			})
			mui.Text(c, drive.Path).FontSize(12).Font("monospace").TextColor(tk.TextTertiary)
		})
	})
}

// albumFolderCard shows where album files live and lets the user rescan the
// folder or choose a different one on the drive.
func (s *Screen) albumFolderCard(c *mui.Context, tk *lobetheme.Tokens) {
	s.mu.RLock()
	folder := s.albumFolder
	busy := s.isScanning || s.isExecuting
	canScan := s.albumScannerFactory != nil
	s.mu.RUnlock()
	advancedCard(c, tk, func() {
		mui.Row(c).Gap(10).AlignItems(mui.Center).Children(func() {
			mui.Icon(c, lucide.Folder()).FontSize(18).TextColor(tk.Primary)
			mui.Column(c).Grow(1).Gap(2).Children(func() {
				mui.Text(c, "Album folder").FontSize(14).FontWeight(600)
				mui.Text(c, "Media in this folder is indexed in the active drive’s Library.").
					FontSize(12).TextColor(tk.TextSecondary)
			})
			action.Button(c, "Rescan album folder", action.ButtonProps{Size: lobetheme.Small}).
				Disabled(busy || !canScan || folder == "").OnClick(s.RescanAlbumFolder)
			action.Button(c, "Choose folder", action.ButtonProps{Size: lobetheme.Small}).
				Disabled(busy || !canScan).OnClick(s.chooseAlbumFolder)
		})
		mui.Text(c, folder).FontSize(12).Font("monospace").TextColor(tk.TextTertiary)
		mui.Text(c, "Choose a folder to load it, or rescan to add new files. Neither action moves files.").
			FontSize(11).TextColor(tk.TextTertiary)
	})
}

// AlbumFolder returns the current source for in-place library indexing.
func (s *Screen) AlbumFolder() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.albumFolder
}

// RescanAlbumFolder indexes media from the configured album folder in place.
func (s *Screen) RescanAlbumFolder() {
	s.mu.Lock()
	if s.albumFolder != "" {
		s.startScanLocked(s.albumFolder, s.albumScannerFactory, true)
	}
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) chooseAlbumFolder() {
	picker := s.directoryPicker
	if picker == nil {
		picker = func(defaultPath string) (string, error) {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Title:       "Choose album folder",
				DefaultPath: defaultPath,
				Directory:   true,
			})
			if err != nil || len(paths) == 0 {
				return "", err
			}
			return paths[0], nil
		}
	}
	current := s.AlbumFolder()
	root := s.driveRoot
	go func() {
		path, err := picker(current)
		if err != nil || path == "" {
			return
		}
		path, err = scanDirectory(root, path)
		s.publish(func() {
			if s.isScanning || s.isExecuting {
				return
			}
			if err != nil {
				s.lastScanErr = err.Error()
				return
			}
			s.albumFolder = path
			s.startScanLocked(path, s.albumScannerFactory, true)
		})
	}()
}

func advancedCard(c *mui.Context, tk *lobetheme.Tokens, body func()) {
	mui.Column(c).Shrink(0).Gap(10).Padding(16, 20).Radius(14).
		Background(tk.BgContainer).Border(1, tk.BorderSecondary).Children(body)
}

func advancedButton(c *mui.Context, label string, tk *lobetheme.Tokens) bool {
	return action.Button(c, label, action.ButtonProps{Size: lobetheme.Small}).
		TextColor(tk.Text).Clicked()
}

func advancedFormatBytes(bytes uint64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	unit := -1
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
