package state

// Screen identifies a primary view in the application shell navigation.
type Screen string

const (
	ScreenDashboard Screen = "dashboard"
	ScreenCatalog   Screen = "catalog"
	ScreenAlbums    Screen = "albums"
	ScreenDiscover  Screen = "discover"
	ScreenSettings  Screen = "settings"
	ScreenLogs      Screen = "logs"
	// ScreenViewer is a transient full-screen view reached from the
	// catalog or dashboard; it has no sidebar entry.
	ScreenViewer Screen = "viewer"
)

// AllScreens lists the navigation targets in sidebar order.
var AllScreens = []Screen{ScreenDashboard, ScreenCatalog, ScreenAlbums, ScreenDiscover, ScreenSettings, ScreenLogs}

// Screens returns a copy of all navigation targets in sidebar order.
func Screens() []Screen {
	out := make([]Screen, len(AllScreens))
	copy(out, AllScreens)
	return out
}

// Valid reports whether s names a known screen.
func (s Screen) Valid() bool {
	switch s {
	case ScreenDashboard, ScreenCatalog, ScreenAlbums, ScreenDiscover, ScreenSettings, ScreenLogs, ScreenViewer:
		return true
	default:
		return false
	}
}
