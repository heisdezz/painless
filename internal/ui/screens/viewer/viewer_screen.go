package viewer

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	uitheme "painless/internal/ui/theme"
)

// DriveRoot returns the dynamic path to the mounted drive root.
type DriveRoot func() string

// Throttler pauses background work (like thumbnails) during active video playback.
type Throttler interface {
	Pause()
	Resume()
}

// FileOpener allows opening media files for pure Go decoding.
type FileOpener interface {
	ReadFile(relPath string) ([]byte, error)
}

// Source fetches media metadata if not already loaded.
type Source interface {
	GetByID(ctx context.Context, id int64) (*domain.MediaItem, error)
}

// Options configures the media viewer screen.
type Options struct {
	DriveRoot     string
	DriveRootFunc DriveRoot
	Throttler     Throttler
	FileOpener    FileOpener
	Source        Source
	RefreshWindow func()
	OnClose       func()
}

// Screen coordinates high-res photo inspection, video playback, and EXIF drawer.
type Screen struct {
	driveRoot     string
	driveRootFunc DriveRoot
	throttler     Throttler
	fileOpener    FileOpener
	source        Source
	refreshWindow func()
	onClose       func()

	// Navigation & items state
	items        []domain.MediaItem
	currentIndex int
	currentItem  *domain.MediaItem
	activeID     int64
	loading      bool
	loadErr      string

	// Metadata Drawer state
	drawerOpen    bool
	accordionOpen map[string]bool

	// Image state
	currentImageSrc mui.ImageSource
	imageLoading    bool
	imageFailed     bool
	loadedHash      string

	// Video state
	videoPlayer  *mui.VideoPlayer
	videoDecoder *FFmpegDecoder
	videoHash    string

	mu sync.RWMutex
}

// New creates an initialized media viewer screen.
func New(opts Options) *Screen {
	dr := opts.DriveRoot
	if dr == "" && opts.DriveRootFunc != nil {
		dr = opts.DriveRootFunc()
	}
	return &Screen{
		driveRoot:     dr,
		driveRootFunc: opts.DriveRootFunc,
		throttler:     opts.Throttler,
		fileOpener:    opts.FileOpener,
		source:        opts.Source,
		refreshWindow: opts.RefreshWindow,
		onClose:       opts.OnClose,
		accordionOpen: map[string]bool{
			"file":       true,
			"dimensions": true,
			"camera":     true,
			"gps":        true,
		},
	}
}

// Open loads a specific media item by ID using the provided source.
func (s *Screen) Open(id int64) {
	s.mu.Lock()
	s.stopVideoLocked()
	s.activeID = id
	s.currentItem = nil
	s.loading = true
	s.loadErr = ""
	s.loadedHash = ""
	s.currentImageSrc = nil
	s.mu.Unlock()

	s.requestRedraw()

	if s.source == nil {
		s.mu.Lock()
		s.loading = false
		s.loadErr = "No media repository source configured"
		s.mu.Unlock()
		s.requestRedraw()
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		item, err := s.source.GetByID(ctx, id)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.activeID != id {
			return
		}
		s.loading = false
		if err != nil {
			s.loadErr = fmt.Sprintf("Failed to load media #%d: %v", id, err)
		} else {
			s.currentItem = item
		}
		s.requestRedraw()
	}()
}

// OpenItem opens the viewer for a specific media item with navigation context.
func (s *Screen) OpenItem(item domain.MediaItem, allItems []domain.MediaItem) {
	s.mu.Lock()
	s.stopVideoLocked()
	s.items = allItems
	s.currentItem = &item
	s.currentIndex = 0
	for i, it := range allItems {
		if it.ID == item.ID {
			s.currentIndex = i
			break
		}
	}
	s.activeID = item.ID
	s.loadedHash = ""
	s.currentImageSrc = nil
	s.mu.Unlock()

	s.requestRedraw()
}

// CurrentItem returns the currently displayed media item.
func (s *Screen) CurrentItem() *domain.MediaItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentItem
}

// IsOpen reports whether a media item is currently being viewed.
func (s *Screen) IsOpen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentItem != nil
}

// NextItem navigates to the next media item in sequence.
func (s *Screen) NextItem() {
	s.mu.Lock()
	if len(s.items) == 0 {
		s.mu.Unlock()
		return
	}
	s.stopVideoLocked()
	s.currentIndex = (s.currentIndex + 1) % len(s.items)
	s.currentItem = &s.items[s.currentIndex]
	s.activeID = s.currentItem.ID
	s.loadedHash = ""
	s.currentImageSrc = nil
	s.mu.Unlock()

	s.requestRedraw()
}

// PrevItem navigates to the previous media item in sequence.
func (s *Screen) PrevItem() {
	s.mu.Lock()
	if len(s.items) == 0 {
		s.mu.Unlock()
		return
	}
	s.stopVideoLocked()
	s.currentIndex = (s.currentIndex - 1 + len(s.items)) % len(s.items)
	s.currentItem = &s.items[s.currentIndex]
	s.activeID = s.currentItem.ID
	s.loadedHash = ""
	s.currentImageSrc = nil
	s.mu.Unlock()

	s.requestRedraw()
}

// ToggleDrawer opens or closes the EXIF metadata drawer.
func (s *Screen) ToggleDrawer() {
	s.mu.Lock()
	s.drawerOpen = !s.drawerOpen
	s.mu.Unlock()
	s.requestRedraw()
}

// Close closes the viewer and stops active playback.
func (s *Screen) Close() {
	s.mu.Lock()
	s.stopVideoLocked()
	s.currentItem = nil
	s.items = nil
	s.drawerOpen = false
	s.mu.Unlock()

	if s.onClose != nil {
		s.onClose()
	}
	s.requestRedraw()
}

// View builds one frame of the media viewer screen.
func (s *Screen) View(c *mui.Context) {
	uitheme.Apply(c)
	tk := lobetheme.TokensOf(c)

	s.mu.RLock()
	itemPtr := s.currentItem
	s.mu.RUnlock()

	if itemPtr == nil {
		mui.Column(c).Fill().Center().Children(func() {
			display.Empty(c, display.EmptyProps{
				Icon:        lucide.Image(),
				Title:       "No media selected",
				Description: "Select an image or video from the Library to inspect.",
			})
		})
		return
	}

	item := *itemPtr

	// Global key handlers for gallery navigation
	c.OnWindowShortcut(0, mui.KeyLeft, func() {
		s.PrevItem()
	})
	c.OnWindowShortcut(0, mui.KeyRight, func() {
		s.NextItem()
	})
	c.OnWindowShortcut(0, mui.KeyEscape, func() {
		s.Close()
	})

	mui.Column(c).Fill().Background(tk.BgLayout).Children(func() {
		// Top toolbar
		s.renderTopBar(c, tk, item)

		// Main preview workspace
		mui.Box(c).Grow(1).Children(func() {
			if item.Type() == domain.MediaTypeVideo {
				s.renderVideoPlayer(c, tk, &item)
			} else {
				s.renderImageViewer(c, tk, &item)
			}
		})
	})

	// EXIF Drawer
	s.metadataDrawer(c, tk)
}

func (s *Screen) renderTopBar(c *mui.Context, tk *lobetheme.Tokens, item domain.MediaItem) {
	mui.Row(c).Height(48).Padding(0, 16).AlignItems(mui.Center).
		Background(tk.BgContainer).Border(1, tk.BorderSecondary).Children(func() {
		// Close / Back button
		action.Button(c, "Close", action.ButtonProps{
			Variant: action.Text,
			Size:    lobetheme.Small,
		}).OnClick(func() {
			s.Close()
		})

		// Title & Sequence info
		mui.Row(c).Padding(0, 12).AlignItems(mui.Center).Gap(8).Grow(1).Children(func() {
			fileName := filepath.Base(item.CurrentRelativePath)
			mui.Text(c, fileName).FontSize(14).FontWeight(600).TextColor(tk.Text).SingleLine()

			s.mu.RLock()
			total := len(s.items)
			currIdx := s.currentIndex + 1
			s.mu.RUnlock()

			if total > 0 {
				display.Tag(c, fmt.Sprintf("%d of %d", currIdx, total), display.TagProps{
					Size:    lobetheme.Small,
					Variant: display.TagFilled,
				})
			}
		})

		// Navigation & Drawer actions
		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			action.ActionIcon(c, lucide.ChevronLeft(), action.ActionIconProps{
				Size:  lobetheme.Middle,
				Title: "Previous Item",
			}).OnClick(func() {
				s.PrevItem()
			})

			action.ActionIcon(c, lucide.ChevronRight(), action.ActionIconProps{
				Size:  lobetheme.Middle,
				Title: "Next Item",
			}).OnClick(func() {
				s.NextItem()
			})

			action.ActionIcon(c, lucide.Info(), action.ActionIconProps{
				Size:  lobetheme.Middle,
				Title: "Toggle EXIF & Metadata",
			}).OnClick(func() {
				s.ToggleDrawer()
			})

			action.ActionIcon(c, lucide.X(), action.ActionIconProps{
				Size:  lobetheme.Middle,
				Title: "Close Viewer",
			}).OnClick(func() {
				s.Close()
			})
		})
	})
}

func (s *Screen) requestRedraw() {
	if s.refreshWindow != nil {
		s.refreshWindow()
	}
}

func (s *Screen) getFullPath(relPath string) string {
	s.mu.RLock()
	dr := s.driveRoot
	if dr == "" && s.driveRootFunc != nil {
		dr = s.driveRootFunc()
	}
	s.mu.RUnlock()

	if dr == "" {
		return relPath
	}
	return filepath.Join(dr, relPath)
}
