package wiring

import (
	"painless/internal/domain"
	"painless/internal/logging"
	"painless/internal/state"
	"painless/internal/ui/screens/settings"
)

// Controllers bundles the real settings screen controllers backed by the
// service layer. Build once with New at app startup; Close on shutdown.
type Controllers struct {
	store *state.Store
	logs  *logging.Logger

	Backups *Backups
	Mobile  *Mobile
	GDrive  *GDrive
}

// New builds the controllers for a store. The controllers resolve the
// active drive themselves, so they keep working across drive switches
// without re-wiring. A nil logger leaves the producers silent; tests
// construct controllers directly without one.
func New(store *state.Store, logs *logging.Logger) *Controllers {
	c := &Controllers{
		store:   store,
		logs:    logs,
		Backups: NewBackups(store),
		Mobile:  NewMobile(store),
		GDrive:  NewGDrive(store),
	}
	c.Backups.logs = logs
	c.Mobile.logs = logs
	c.GDrive.logs = logs
	// Surface background start/stop failures that no settings tab is open
	// to see; without these the reconciler fails silently.
	c.Mobile.Toast = store.Notify
	c.GDrive.Toast = store.Notify
	return c
}

// Close releases background resources: the mobile sync server and the
// drive-event pumps. Safe to call more than once.
func (c *Controllers) Close() {
	c.Mobile.Close()
	c.GDrive.Close()
}

// SettingsScreen builds the settings screen wired to these controllers,
// ready to hand to Shell.SetSettings.
func (c *Controllers) SettingsScreen(logs *logging.Logger, toast func(level domain.LogLevel, title, body string)) *settings.Screen {
	return settings.New(settings.Options{
		Backups: c.Backups,
		Mobile:  c.Mobile,
		GDrive:  c.GDrive,
		Logs:    logs,
		Toast:   toast,
		Store:   c.store,
	})
}
