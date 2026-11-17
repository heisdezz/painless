package viewer

import (
	"os"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) renderImageViewer(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	c.Key(item.FileHash)

	s.ensureImageLoaded(item)

	s.mu.RLock()
	src := s.currentImageSrc
	loading := s.imageLoading
	failed := s.imageFailed
	s.mu.RUnlock()

	fileName := filepath.Base(item.CurrentRelativePath)

	mui.Column(c).Grow(1).Basis(0).Center().Children(func() {
		display.Image(c, src, display.ImageProps{
			Contain: true,
			Loading: loading,
			Failed:  failed,
			Alt:     fileName,
			Variant: display.ImageFilled,
		})
	})
}

func (s *Screen) ensureImageLoaded(item *domain.MediaItem) {
	s.mu.Lock()
	if s.loadedHash == item.FileHash {
		s.mu.Unlock()
		return
	}
	s.loadedHash = item.FileHash
	s.currentImageSrc = nil
	s.imageLoading = true
	s.imageFailed = false
	s.mu.Unlock()

	go func(hash, relPath string) {
		var data []byte
		var err error

		if s.fileOpener != nil {
			data, err = s.fileOpener.ReadFile(relPath)
		} else if s.driveRoot != "" {
			fullPath := filepath.Join(s.driveRoot, relPath)
			data, err = os.ReadFile(fullPath)
		}

		if err != nil || len(data) == 0 {
			s.mu.Lock()
			if s.loadedHash == hash {
				s.imageLoading = false
				s.imageFailed = true
			}
			s.mu.Unlock()
			s.requestRedraw()
			return
		}

		imgSrc, decErr := mui.DecodeImage(data)
		s.mu.Lock()
		if s.loadedHash == hash {
			s.imageLoading = false
			if decErr != nil {
				s.imageFailed = true
			} else {
				s.currentImageSrc = imgSrc
			}
		}
		s.mu.Unlock()
		s.requestRedraw()
	}(item.FileHash, item.CurrentRelativePath)
}
