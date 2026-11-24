package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"painless/internal/domain"
	"painless/internal/library"
	"painless/internal/logging"
	"painless/internal/server/mobile"
	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/thumbnail"
	"painless/internal/ui/screens/settings"
)

// mobileShutdownGrace bounds one server stop attempt.
const mobileShutdownGrace = 3 * time.Second

// Compile-time proof the controller satisfies the settings screen contract.
var _ settings.MobileSyncController = (*Mobile)(nil)

// Mobile implements settings.MobileSyncController on top of the mobile REST
// server. The controller owns its server, database connection and
// repositories while sync runs; a bus subscription starts and stops it as
// drives attach and detach, so a drive with sync enabled is served the
// moment it is plugged in.
type Mobile struct {
	store *state.Store

	// Toast, when set, surfaces background start/stop failures that no
	// settings tab is open to see.
	Toast func(level domain.LogLevel, title, body string)

	// logs, when set, receives sync lifecycle entries for the logs console.
	logs *logging.Logger

	mu      sync.Mutex
	enabled bool // session-level desired state; not persisted
	cfg     driveSettings
	cfgRoot string // drive root the config was loaded from; "" = none yet
	dbRoot  string // resolved library root the running server was opened at
	srv     *mobile.Server
	db      *storage.DB
	cancel  func() // stops the drive-event pump
}

// NewMobile builds the mobile sync controller and starts watching drive
// changes so an enabled server follows the active drive.
func NewMobile(store *state.Store) *Mobile {
	m := &Mobile{store: store}
	sub := store.Bus.Subscribe(16)
	done := make(chan struct{})
	m.cancel = func() { close(done); sub.Cancel() }
	go m.pump(sub.C, done)
	return m
}

// pump reacts to drive events until Close or the bus tears it down.
func (m *Mobile) pump(events <-chan state.Event, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			switch ev.Kind {
			case state.EventDrivesChanged, state.EventActiveDriveChange, state.EventLibraryLocationChanged:
				m.followDrive()
			}
		}
	}
}

// followDrive reloads per-drive settings when the active drive changed and
// reconciles the running server with the loaded configuration.
func (m *Mobile) followDrive() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	m.reconcileLocked()
}

// refreshLocked reloads the per-drive settings whenever the active drive
// changed since the last call, so public methods never act on a stale
// drive. The enabled flag is session state and survives the reload.
// Callers must hold m.mu.
func (m *Mobile) refreshLocked() {
	root := m.activeRoot()
	if root == m.cfgRoot {
		return
	}
	m.cfg = loadDriveSettings(root)
	m.cfgRoot = root
}

// reconcileLocked aligns the server with the enabled flag, the drive the
// config belongs to, and the resolved library location. Callers must hold
// m.mu.
func (m *Mobile) reconcileLocked() {
	switch {
	case m.srv != nil && (m.cfgRoot == "" || !m.enabled):
		logf(m.logs, "mobile sync", domain.LogLevelInfo, "stopping: %s",
			map[bool]string{true: "active drive detached", false: "sync disabled"}[m.cfgRoot == ""])
		if err := m.stopLocked(); err != nil {
			logf(m.logs, "mobile sync", domain.LogLevelWarn, "stop failed: %v", err)
			if m.Toast != nil {
				m.Toast(domain.LogLevelWarn, "Mobile sync", err.Error())
			}
		}
	case m.srv == nil && m.cfgRoot != "" && m.enabled:
		if err := m.startLocked(); err != nil {
			logf(m.logs, "mobile sync", domain.LogLevelError, "start failed: %v", err)
			if m.Toast != nil {
				m.Toast(domain.LogLevelError, "Mobile sync", err.Error())
			}
		}
	case m.srv != nil && library.ResolveRoot(m.cfgRoot) != m.dbRoot:
		// The library folder for this drive changed; restart so the server
		// serves the catalog at its new location.
		logf(m.logs, "mobile sync", domain.LogLevelInfo, "library folder moved, restarting sync server")
		if err := m.restartLocked(); err != nil {
			logf(m.logs, "mobile sync", domain.LogLevelError, "restart failed: %v", err)
			if m.Toast != nil {
				m.Toast(domain.LogLevelError, "Mobile sync", err.Error())
			}
		}
	}
}

// startLocked opens the library and begins serving sync from the drive the
// config belongs to. The catalog lives at the drive's resolved library
// location, which may be a folder the user chose in Discover rather than
// the drive root; media paths stay relative to the drive root, so the
// server keeps the drive as its file root.
// Callers must hold m.mu.
func (m *Mobile) startLocked() error {
	root := m.cfgRoot
	if root == "" {
		return errors.New("no active drive")
	}
	if m.srv != nil {
		return nil
	}
	libRoot := library.ResolveRoot(root)
	db, err := storage.Open(libRoot, storage.OpenOptions{})
	if err != nil {
		return fmt.Errorf("open library: %w", err)
	}
	token := m.cfg.MobileToken
	if token == "" {
		token = newToken()
		m.cfg.MobileToken = token
		// Best effort: an unwritable drive still gets a session token.
		_ = m.cfg.save(root)
	}

	thumbs := thumbnail.New(thumbnail.Options{DriveRoot: root, Store: m.store})
	srv := mobile.New(mobile.Options{
		Port:      m.portLocked(),
		Token:     token,
		DriveRoot: root,
		DBFile:    filepath.Join(libRoot, storage.LibraryDBFilename),
		Albums:    storage.NewAlbumRepo(db),
		Media:     storage.NewMediaRepo(db),
		DriveInfo: m.driveInfo,
		// Closures capture this session's db handle; the server shuts down
		// before the handle closes, so they never race a Close.
		LibraryStats:     func() *mobile.LibraryStats { s := libraryStats(db, libRoot); return &s },
		DatabaseSnapshot: dbSnapshotter(db),
		Thumbnails:       thumbs,
	})
	if err := srv.Start(); err != nil {
		_ = db.Close()
		return err
	}
	m.srv, m.db = srv, db
	m.dbRoot = libRoot
	logf(m.logs, "mobile sync", domain.LogLevelInfo,
		"sync server listening on port %d for drive %s", m.portLocked(), root)
	return nil
}

// stopLocked shuts the server down and releases its library connection.
// Callers must hold m.mu.
func (m *Mobile) stopLocked() error {
	if m.srv == nil {
		return nil
	}
	port := m.portLocked()
	srv, db := m.srv, m.db
	m.srv, m.db, m.dbRoot = nil, nil, ""
	ctx, cancel := context.WithTimeout(context.Background(), mobileShutdownGrace)
	defer cancel()
	err := srv.Shutdown(ctx)
	if cerr := db.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		logf(m.logs, "mobile sync", domain.LogLevelWarn,
			"sync server on port %d stopped with error: %v", port, err)
		return err
	}
	logf(m.logs, "mobile sync", domain.LogLevelInfo,
		"sync server on port %d stopped", port)
	return nil
}

// restartLocked applies config changes to a running server. Callers must
// hold m.mu.
func (m *Mobile) restartLocked() error {
	if m.srv == nil {
		return nil
	}
	if err := m.stopLocked(); err != nil {
		return err
	}
	return m.startLocked()
}

// driveInfo reports the active drive snapshot for /api/status.
func (m *Mobile) driveInfo() *domain.DriveInfo {
	if drive, ok := m.store.State.ActiveDrive(); ok {
		return &drive
	}
	return nil
}

// libraryStats gathers the /api/info library summary with single-row
// counts; a missing table (older schema) counts as zero.
func libraryStats(db *storage.DB, root string) mobile.LibraryStats {
	stats := mobile.LibraryStats{}
	if db == nil {
		return stats
	}
	count := func(query string) int64 {
		var n int64
		_ = db.Raw().QueryRow(query).Scan(&n)
		return n
	}
	stats.TotalItems = count("SELECT count(*) FROM media_items")
	stats.Images = count("SELECT count(*) FROM media_items WHERE mime_type LIKE 'image/%'")
	stats.Videos = count("SELECT count(*) FROM media_items WHERE mime_type LIKE 'video/%'")
	stats.Albums = count("SELECT count(*) FROM albums")
	stats.Tags = count("SELECT count(*) FROM tags")
	if info, err := os.Stat(filepath.Join(root, storage.LibraryDBFilename)); err == nil {
		stats.DBSizeBytes = info.Size()
		stats.DBExists = true
	}
	return stats
}

// dbSnapshotter returns a /download/db snapshot hook: VACUUM INTO writes
// a consistent copy to a temp file the server deletes after streaming.
func dbSnapshotter(db *storage.DB) func(ctx context.Context) (string, func(), error) {
	return func(ctx context.Context) (string, func(), error) {
		tmp, err := os.CreateTemp("", "painless-library-*.db")
		if err != nil {
			return "", nil, err
		}
		path := tmp.Name()
		if err := tmp.Close(); err != nil {
			_ = os.Remove(path)
			return "", nil, err
		}
		query := fmt.Sprintf("VACUUM INTO '%s';", strings.ReplaceAll(filepath.ToSlash(path), "'", "''"))
		if _, err := db.Raw().ExecContext(ctx, query); err != nil {
			_ = os.Remove(path)
			return "", nil, err
		}
		return path, func() { _ = os.Remove(path) }, nil
	}
}

// activeRoot returns the drive root the config belongs to. Callers must
// hold m.mu.
func (m *Mobile) activeRoot() string {
	drive, ok := m.store.State.ActiveDrive()
	if !ok || drive.Path == "" {
		return ""
	}
	return drive.Path
}

// Enabled reports whether the HTTP server should run.
func (m *Mobile) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	return m.enabled
}

// SetEnabled starts or stops the server and persists the choice.
func (m *Mobile) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	m.enabled = enabled
	m.reconcileLocked()
}

// portLocked is the configured listen port. Callers must hold m.mu.
func (m *Mobile) portLocked() int {
	if m.cfg.MobilePort > 0 && m.cfg.MobilePort <= 65535 {
		return m.cfg.MobilePort
	}
	return mobile.DefaultPort
}

// Port is the configured listen port.
func (m *Mobile) Port() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	return m.portLocked()
}

// SetPort changes the port; applies live when the server is running.
func (m *Mobile) SetPort(port int) {
	if port <= 0 || port > 65535 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	if port == m.cfg.MobilePort {
		return
	}
	m.cfg.MobilePort = port
	if m.cfgRoot != "" {
		_ = m.cfg.save(m.cfgRoot)
	}
	logf(m.logs, "mobile sync", domain.LogLevelInfo, "port changed to %d", port)
	if err := m.restartLocked(); err != nil {
		logf(m.logs, "mobile sync", domain.LogLevelError, "restart failed: %v", err)
		if m.Toast != nil {
			m.Toast(domain.LogLevelError, "Mobile sync", err.Error())
		}
	}
}

// URL is the LAN pairing URL, empty when the server is not running.
func (m *Mobile) URL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	if m.srv == nil {
		return ""
	}
	addr, err := mobile.LANAddress(m.portLocked())
	if err != nil {
		// Fall back to the wildcard listener address so pairing stays
		// possible on hosts without a private IPv4 address.
		return "http://127.0.0.1:" + strconv.Itoa(m.portLocked())
	}
	return "http://" + addr
}

// Token is the current pairing token, generated on first use.
func (m *Mobile) Token() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	if m.cfg.MobileToken != "" {
		return m.cfg.MobileToken
	}
	token := newToken()
	m.cfg.MobileToken = token
	if m.cfgRoot != "" {
		_ = m.cfg.save(m.cfgRoot)
	}
	return token
}

// RegenerateToken replaces the token, restarts a running server so it
// takes effect, and returns the new token.
func (m *Mobile) RegenerateToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	m.cfg.MobileToken = newToken()
	if m.cfgRoot != "" {
		_ = m.cfg.save(m.cfgRoot)
	}
	logf(m.logs, "mobile sync", domain.LogLevelInfo, "pairing token regenerated")
	if err := m.restartLocked(); err != nil {
		logf(m.logs, "mobile sync", domain.LogLevelError, "restart failed: %v", err)
		if m.Toast != nil {
			m.Toast(domain.LogLevelError, "Mobile sync", err.Error())
		}
	}
	return m.cfg.MobileToken
}

// Close stops the server and the drive-event pump. Safe to call twice.
func (m *Mobile) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	_ = m.stopLocked()
}
