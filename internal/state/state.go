package state

import (
	"sync"

	"painless/internal/domain"
)

// AppState is the central, thread-safe application state shared by UI
// screens and background workers. UI views read it during build passes on
// the main thread; background workers mutate it only from callbacks that
// the bridge runs through mygo Window.Update, or via the event bus for
// notifications.
//
// AppState never stores ui.Context or ui.Element values.
type AppState struct {
	mu sync.RWMutex

	screen     Screen
	prev       Screen // screen active before the current one, used by BackScreen
	drives     []domain.DriveInfo
	activeID   string // domain.DriveInfo.ID of the active drive, "" if none
	selection  Selection
	jobs       map[string]*Job
	jobOrder   []string
	thumbnails bool
	autoplay   bool
}

// New creates an initialized AppState with the catalog screen active.
func New() *AppState {
	return &AppState{
		screen:     ScreenDashboard,
		selection:  NewSelection(),
		jobs:       map[string]*Job{},
		thumbnails: true,
		autoplay:   true,
	}
}

// ThumbnailsEnabled reports whether media thumbnails are displayed.
func (s *AppState) ThumbnailsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.thumbnails
}

// SetThumbnailsEnabled toggles the display of media thumbnails.
func (s *AppState) SetThumbnailsEnabled(enabled bool) {
	s.mu.Lock()
	s.thumbnails = enabled
	s.mu.Unlock()
}

// AutoplayEnabled reports whether videos start playing when opened.
func (s *AppState) AutoplayEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.autoplay
}

// SetAutoplayEnabled toggles autoplay for videos opened in the viewer.
func (s *AppState) SetAutoplayEnabled(enabled bool) {
	s.mu.Lock()
	s.autoplay = enabled
	s.mu.Unlock()
}

// Screen returns the currently displayed navigation screen.
func (s *AppState) Screen() Screen {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.screen
}

// SetScreen switches the active navigation screen. Invalid screens are
// ignored. Returns true when the screen changed.
func (s *AppState) SetScreen(scr Screen) bool {
	if !scr.Valid() {
		return false
	}
	s.mu.Lock()
	changed := s.screen != scr
	if changed {
		s.prev = s.screen
		s.screen = scr
	}
	s.mu.Unlock()
	return changed
}

// BackScreen returns the route to return to when leaving the current screen:
// the screen active before the current one, or the catalog when there is no
// recorded route or it is the current screen.
func (s *AppState) BackScreen() Screen {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.prev.Valid() && s.prev != s.screen {
		return s.prev
	}
	return ScreenCatalog
}

// Drives returns a copy of the currently detected drives.
func (s *AppState) Drives() []domain.DriveInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.DriveInfo(nil), s.drives...)
}

// SetDrives replaces the detected drive list and returns the previous
// active drive ID and whether the active drive is still present.
func (s *AppState) SetDrives(drives []domain.DriveInfo) (prevActiveID string, activePresent bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prevActiveID = s.activeID
	s.drives = append([]domain.DriveInfo(nil), drives...)
	for _, d := range s.drives {
		if d.ID == s.activeID {
			return prevActiveID, true
		}
	}
	// Active drive vanished: fall back to the first drive with a library,
	// else the first drive, else none.
	s.activeID = ""
	if len(s.drives) > 0 {
		for _, d := range s.drives {
			if d.HasLibrary {
				s.activeID = d.ID
				break
			}
		}
		if s.activeID == "" {
			s.activeID = s.drives[0].ID
		}
	}
	return prevActiveID, false
}

// ActiveDrive returns a copy of the active drive, or false when no drive
// is active.
func (s *AppState) ActiveDrive() (domain.DriveInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.drives {
		if d.ID == s.activeID {
			return d, true
		}
	}
	return domain.DriveInfo{}, false
}

// ActiveDriveID returns the active drive ID, empty when none.
func (s *AppState) ActiveDriveID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeID
}

// ClearActiveDrive detaches the active drive. Returns true when a drive
// was active and the selection changed.
func (s *AppState) ClearActiveDrive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.activeID != ""
	s.activeID = ""
	return changed
}

// SetActiveDrive selects the active drive by ID. Returns true when the
// drive exists and the selection changed.
func (s *AppState) SetActiveDrive(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.drives {
		if d.ID == id {
			changed := s.activeID != id
			s.activeID = id
			return changed
		}
	}
	return false
}
