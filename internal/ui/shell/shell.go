// Package shell builds Painless's main window: the navigation rail, the
// active-drive bar, the screen host and the notification stack.
package shell

import (
	mui "github.com/egoist/mygo/ui"

	"painless/internal/state"
	uitheme "painless/internal/ui/theme"
)

// Shell owns the UI-local state that must survive view passes. App data and
// background results live in the state store; only presentation state, such
// as the rail's width, belongs here.
type Shell struct {
	store   *state.Store
	toaster *Toaster

	navWidth    float32
	navExpanded bool
	refresh     func()
}

// New creates a shell around store. A nil store gets a fresh one.
func New(store *state.Store) *Shell {
	if store == nil {
		store = state.NewStore()
	}
	return &Shell{
		store:       store,
		toaster:     newToaster(store),
		navWidth:    220,
		navExpanded: true,
	}
}

// Store returns the shell's state store.
func (s *Shell) Store() *state.Store { return s.store }

// Toaster returns the shell's notification manager.
func (s *Shell) Toaster() *Toaster { return s.toaster }

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
