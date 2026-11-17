// Package shell builds Painless's main window: the navigation rail, the
// active-drive bar, the screen host and the notification stack.
package shell

import (
	"context"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/logging"
	"painless/internal/state"
	"painless/internal/ui/screens/albums"
	"painless/internal/ui/screens/catalog"
	"painless/internal/ui/screens/dashboard"
	"painless/internal/ui/screens/discover"
	"painless/internal/ui/screens/settings"
	uitheme "painless/internal/ui/theme"
)

// Shell owns the UI-local state that must survive view passes. App data and
// background results live in the state store; only presentation state, such
// as the rail's width, belongs here.
type Shell struct {
	store     *state.Store
	toaster   *Toaster
	catalog   *catalog.Screen
	albums    *albums.Screen
	dashboard *dashboard.Screen
	discover  *discover.Screen
	settings  *settings.Screen
	drives    *driveController

	logger      *logging.Logger
	navWidth    float32
	navExpanded bool
	refresh     func()
}

// New creates a shell around store. A nil store gets a fresh one.
func New(store *state.Store) *Shell {
	if store == nil {
		store = state.NewStore()
	}
	logger := logging.New(logging.Options{})
	return &Shell{
		store:       store,
		toaster:     newToaster(store),
		catalog:     catalog.New(catalog.Options{}),
		albums:      albums.New(albums.Options{}),
		dashboard:   dashboard.New(dashboard.Options{}),
		discover:    discover.New(discover.Options{Store: store}),
		logger:      logger,
		settings:    settings.New(settings.Options{Logs: logger, Toast: store.Notify}),
		navWidth:    220,
		navExpanded: true,
	}
}

// Store returns the shell's state store.
func (s *Shell) Store() *state.Store { return s.store }

// Toaster returns the shell's notification manager.
func (s *Shell) Toaster() *Toaster { return s.toaster }

// SetCatalog wires a media source, thumbnail renderer and action handlers
// into the library screen. Call before the window is shown.
func (s *Shell) SetCatalog(source catalog.Source, thumbs catalog.Thumbnailer, actions catalog.Actions) {
	s.catalog = catalog.New(catalog.Options{Source: source, Thumbs: thumbs, Actions: actions})
}

// Catalog returns the library screen, for tests and further wiring.
func (s *Shell) Catalog() *catalog.Screen { return s.catalog }

// SetAlbums configures the album gallery screen.
func (s *Shell) SetAlbums(a *albums.Screen) {
	s.albums = a
}

// Albums returns the album gallery screen.
func (s *Shell) Albums() *albums.Screen { return s.albums }

// attachDrives installs the drive controller and its refresh action. The
// controller also seeds the store and watches monitor events when started.
func (s *Shell) attachDrives(controller *driveController) {
	s.drives = controller
	s.refresh = func() { controller.Refresh(context.Background()) }
}

// SetDashboard configures the dashboard screen shown on app launch.
func (s *Shell) SetDashboard(d *dashboard.Screen) {
	s.dashboard = d
}

// Dashboard returns the dashboard screen, for tests and further wiring.
func (s *Shell) Dashboard() *dashboard.Screen { return s.dashboard }

// SetDiscover configures the discover & staging screen.
func (s *Shell) SetDiscover(d *discover.Screen) {
	s.discover = d
}

// Discover returns the discover & staging screen.
func (s *Shell) Discover() *discover.Screen { return s.discover }

// Logger returns the process-wide UI logger, for services and screens to
// write diagnostic entries into the logs console.
func (s *Shell) Logger() *logging.Logger { return s.logger }

// SetSettings configures the settings screen with real controllers.
func (s *Shell) SetSettings(screen *settings.Screen) {
	s.settings = screen
}

// Settings returns the settings screen, for tests and further wiring.
func (s *Shell) Settings() *settings.Screen { return s.settings }

// View builds one frame of the app shell.
func (s *Shell) View(c *mui.Context) {
	uitheme.Apply(c)
	tk := uitheme.Tokens(c)

	page := mui.Column(c).Fill()
	if !c.Vibrancy() {
		page.Background(tk.BgLayout)
	}
	page.Children(func() {
		s.topBar(c, tk)
		mui.Row(c).Grow(1).Basis(0).MinHeight(0).AlignItems(mui.Stretch).Children(func() {
			s.sidebar(c, tk)
			s.content(c, tk)
		})
		s.toaster.Host(c)
	})
}
