package discover

import (
	"os"
	"path/filepath"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/fileops"
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
		fileops.SettingsDir(s.driveRoot),
		fileops.AlbumsDir(s.driveRoot),
		fileops.UnknownDir(s.driveRoot),
	}
	if folder != "" {
		folders = append(folders, folder)
	}

	seenPath := make(map[string]bool, len(folders))
	seenName := make(map[string]bool, len(folders))
	missing := make([]string, 0, len(folders))
	for _, path := range folders {
		if seenPath[path] {
			continue
		}
		seenPath[path] = true
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			continue
		}
		// The configured album folder and the managed albums directory can
		// share a base name; report each one only once.
		name := filepath.Base(path)
		if seenName[name] {
			continue
		}
		seenName[name] = true
		missing = append(missing, name)
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
