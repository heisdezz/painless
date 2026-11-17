package discover

import (
	"os"
	"path/filepath"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// scaffoldBanner flags missing library folders so the user can rescan before
// indexing. It stays non-blocking; scanning still works if folders are absent.
func (s *Screen) scaffoldBanner(c *mui.Context, tk *lobetheme.Tokens) {
	if s.driveRoot == "" {
		return
	}
	s.mu.RLock()
	folder := s.albumFolder
	s.mu.RUnlock()
	folders := []string{
		filepath.Join(s.driveRoot, ".painless"),
		folder,
		filepath.Join(s.driveRoot, "unknown"),
	}
	missing := make([]string, 0, len(folders))
	for _, folder := range folders {
		if folder == "" {
			continue
		}
		if info, err := os.Stat(folder); err != nil || !info.IsDir() {
			missing = append(missing, filepath.Base(folder))
		}
	}
	if len(missing) == 0 {
		return
	}

	feedback.Alert(c, "Missing library folders: "+strings.Join(missing, ", "), feedback.AlertProps{
		Type: feedback.Warning, Variant: feedback.Soft,
	})
	mui.Row(c).Gap(8).Children(func() {
		if action.Button(c, "Retry check", action.ButtonProps{Size: lobetheme.Small}).Clicked() {
			s.requestRedraw()
		}
		if action.Button(c, "Rescan folder", action.ButtonProps{Size: lobetheme.Small}).Clicked() {
			s.RescanAlbumFolder()
		}
	})
}
