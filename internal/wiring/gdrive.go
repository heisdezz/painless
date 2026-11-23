package wiring

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"painless/internal/domain"
	"painless/internal/gdrive"
	"painless/internal/logging"
	"painless/internal/state"
	"painless/internal/storage"
	"painless/internal/ui/screens/settings"
)

const (
	// gdriveTestTimeout bounds one TestConnection round trip.
	gdriveTestTimeout = 30 * time.Second
	// gdriveBackupTimeout bounds one snapshot-and-upload pass; multi-GB
	// libraries over slow uplinks can legitimately take a while.
	gdriveBackupTimeout = 30 * time.Minute
)

// Compile-time proof the controller satisfies the settings screen contract.
var _ settings.GDriveController = (*GDrive)(nil)

// driveClient is the subset of *gdrive.Client the controller needs; tests
// replace it with a fake through the clientFactory hook.
type driveClient interface {
	TestConnection(ctx context.Context, targetFolderID string) (*gdrive.AccountInfo, error)
	BackupDatabase(ctx context.Context, snapshotter gdrive.DatabaseSnapshotter,
		targetFolderID, tempDir string, onProgress func(written, total int64)) (*gdrive.BackupResult, error)
}

// clientFactory builds a client from service-account credentials JSON.
type clientFactory func(ctx context.Context, credentials []byte) (driveClient, error)

// realClientFactory builds genuine Google Drive API clients.
func realClientFactory(ctx context.Context, credentials []byte) (driveClient, error) {
	return gdrive.NewClient(ctx, credentials)
}

// GDrive implements settings.GDriveController on top of the gdrive client.
// Credentials are per-drive settings; a bus subscription reloads them when
// the active drive changes and drops the cached client.
type GDrive struct {
	store *state.Store

	// newClient is swappable for tests; production uses realClientFactory.
	newClient clientFactory

	// Toast, when set, surfaces background failures.
	Toast func(level domain.LogLevel, title, body string)

	// logs, when set, receives cloud backup entries for the logs console.
	logs *logging.Logger

	mu        sync.Mutex
	cfg       driveSettings
	cfgRoot   string // drive root the config was loaded from; "" = volatile
	client    driveClient
	clientKey string // credentials the cached client was built from
	cancel    func() // stops the drive-event pump
}

// NewGDrive builds the Google Drive controller and starts watching drive
// changes so credentials follow the active drive.
func NewGDrive(store *state.Store) *GDrive {
	g := &GDrive{store: store, newClient: realClientFactory}
	sub := store.Bus.Subscribe(16)
	done := make(chan struct{})
	g.cancel = func() { close(done); sub.Cancel() }
	go g.pump(sub.C, done)
	return g
}

// pump reacts to drive events until Close or the bus tears it down.
func (g *GDrive) pump(events <-chan state.Event, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			switch ev.Kind {
			case state.EventDrivesChanged, state.EventActiveDriveChange:
				g.followDrive()
			}
		}
	}
}

// followDrive reloads per-drive credentials when the active drive changed.
func (g *GDrive) followDrive() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refreshLocked()
}

// refreshLocked reloads the per-drive settings whenever the active drive
// changed since the last call, so public methods never act on a stale
// drive; the cached client is dropped with the old credentials.
// Callers must hold g.mu.
func (g *GDrive) refreshLocked() {
	drive, ok := g.store.State.ActiveDrive()
	root := ""
	if ok {
		root = drive.Path
	}
	if root == g.cfgRoot {
		return
	}
	g.cfg = loadDriveSettings(root)
	g.cfgRoot = root
	g.client, g.clientKey = nil, ""
}

// Credentials returns the saved service-account JSON and folder ID.
func (g *GDrive) Credentials() (string, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refreshLocked()
	return g.cfg.GDriveCredentials, g.cfg.GDriveFolderID
}

// SetCredentials stores edited credentials, persisting them when a drive
// is active, and drops any cached client built from the old credentials.
func (g *GDrive) SetCredentials(json, folderID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refreshLocked()
	g.cfg.GDriveCredentials = strings.TrimSpace(json)
	g.cfg.GDriveFolderID = strings.TrimSpace(folderID)
	if g.cfgRoot != "" {
		if err := g.cfg.save(g.cfgRoot); err != nil && g.Toast != nil {
			g.Toast(domain.LogLevelWarn, "Google Drive", err.Error())
		}
	}
	g.client, g.clientKey = nil, ""
}

// clientFor builds or returns the cached client for the current
// credentials. Callers must hold g.mu; the returned client is immutable
// and safe to use after the lock is released.
func (g *GDrive) clientFor() (driveClient, string, error) {
	creds := g.cfg.GDriveCredentials
	if creds == "" {
		return nil, "", errors.New("service account credentials not configured")
	}
	if g.client != nil && g.clientKey == creds {
		return g.client, g.cfg.GDriveFolderID, nil
	}
	client, err := g.newClient(context.Background(), []byte(creds))
	if err != nil {
		return nil, "", err
	}
	logf(g.logs, "gdrive", domain.LogLevelInfo, "client created for service account")
	g.client, g.clientKey = client, creds
	return client, g.cfg.GDriveFolderID, nil
}

// TestConnection validates credentials and folder access.
func (g *GDrive) TestConnection() error {
	g.mu.Lock()
	g.refreshLocked()
	client, folderID, err := g.clientFor()
	g.mu.Unlock()
	if err != nil {
		logf(g.logs, "gdrive", domain.LogLevelError, "connection test failed: %v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gdriveTestTimeout)
	defer cancel()
	if _, err = client.TestConnection(ctx, folderID); err != nil {
		logf(g.logs, "gdrive", domain.LogLevelError, "connection test failed: %v", err)
		return err
	}
	logf(g.logs, "gdrive", domain.LogLevelInfo, "connection test succeeded (folder %s)", folderLabel(folderID))
	return nil
}

// BackupNow uploads a fresh database snapshot and returns a human-readable
// result message for the settings tab.
func (g *GDrive) BackupNow() (string, error) {
	drive, ok := g.store.State.ActiveDrive()
	if !ok || drive.Path == "" {
		err := errors.New("no active drive")
		logf(g.logs, "gdrive", domain.LogLevelWarn, "backup failed: %v", err)
		return "", err
	}
	g.mu.Lock()
	g.refreshLocked()
	client, folderID, err := g.clientFor()
	g.mu.Unlock()
	if err != nil {
		logf(g.logs, "gdrive", domain.LogLevelError, "backup failed: %v", err)
		return "", err
	}
	var res *gdrive.BackupResult
	err = withLibrary(drive.Path, func(db *storage.DB) error {
		ctx, cancel := context.WithTimeout(context.Background(), gdriveBackupTimeout)
		defer cancel()
		var err error
		res, err = client.BackupDatabase(ctx, sqlSnapshotter{db: db.Raw()}, folderID, "", nil)
		return err
	})
	if err != nil {
		logf(g.logs, "gdrive", domain.LogLevelError, "backup failed: %v", err)
		return "", err
	}
	message := fmt.Sprintf("Uploaded %s (%s)", res.FileName, formatBytes(res.Size))
	logf(g.logs, "gdrive", domain.LogLevelInfo, "backup complete: %s", message)
	return message, nil
}

// folderLabel masks an empty folder ID in log output; the shared-folder
// case is the common one and reads better than an empty pair of quotes.
func folderLabel(folderID string) string {
	if folderID == "" {
		return "(shared root)"
	}
	return folderID
}

// sqlSnapshotter adapts a raw sql.DB to gdrive.DatabaseSnapshotter with a
// path-targeted VACUUM INTO, mirroring the storage package's own backup.
type sqlSnapshotter struct {
	db *sql.DB
}

// CreateBackup writes a consistent snapshot of the library to backupPath.
func (s sqlSnapshotter) CreateBackup(backupPath string) error {
	query := fmt.Sprintf("VACUUM INTO '%s';", strings.ReplaceAll(filepath.ToSlash(backupPath), "'", "''"))
	_, err := s.db.Exec(query)
	return err
}

// Close stops the drive-event pump. Safe to call twice.
func (g *GDrive) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
}

// formatBytes renders a byte count for status messages.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
