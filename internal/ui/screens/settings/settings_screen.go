// Package settings renders the Settings screen: database backups, mobile
// companion pairing, Google Drive configuration and the live logs
// console, arranged as Lobe tabs. Controllers carry the real work; the
// screen only collects input and displays results, so every tab is
// testable with fakes.
package settings

import (
	"sync"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/nav"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
	"painless/internal/logging"
)

// BackupsController serves the backup tab. App wiring satisfies it with
// *storage.DB plus a restore helper.
type BackupsController interface {
	// DBPath is the library database location for the health card.
	DBPath() string
	// DBSizeBytes is the current database file size.
	DBSizeBytes() int64
	// CreateBackup snapshots the database into .media_backups.
	CreateBackup() (path string, err error)
	// ListBackups returns existing snapshots, newest first.
	ListBackups() ([]BackupInfo, error)
	// RestoreBackup copies a snapshot over the live database.
	RestoreBackup(path string) error
	// IntegrityCheck runs PRAGMA integrity_check and returns its report.
	IntegrityCheck() (report string, ok bool)
}

// BackupInfo describes one database snapshot row.
type BackupInfo struct {
	Name      string
	Path      string
	SizeBytes int64
	CreatedAt string
}

// MobileSyncController drives the mobile sync server from its tab.
type MobileSyncController interface {
	// Enabled reports whether the HTTP server should run.
	Enabled() bool
	// SetEnabled starts or stops the server.
	SetEnabled(enabled bool)
	// Port is the configured listen port.
	Port() int
	// SetPort changes the port; applies on next start or live restart.
	SetPort(port int)
	// URL is the LAN pairing URL, empty when unknown.
	URL() string
	// Token is the current pairing token.
	Token() string
	// RegenerateToken replaces the token and returns it.
	RegenerateToken() string
}

// GDriveController drives the Google Drive tab.
type GDriveController interface {
	// Credentials returns the saved service-account JSON and folder ID.
	Credentials() (json, folderID string)
	// SetCredentials stores edited credentials.
	SetCredentials(json, folderID string)
	// TestConnection validates credentials and folder access.
	TestConnection() error
	// BackupNow uploads a fresh database snapshot.
	BackupNow() (message string, err error)
}

// Options configures a settings screen. Nil controllers render
// informational empty states instead of broken controls.
type Options struct {
	Backups BackupsController
	Mobile  MobileSyncController
	GDrive  GDriveController
	Logs    *logging.Logger
	// Toast raises a notification through the shell's toaster. Optional.
	Toast func(level domain.LogLevel, title, body string)
}

// Screen owns the settings tab and draft-input state. All fields are
// touched on the UI thread only; controller calls that may block run on
// goroutines and re-enter via services.Invalidate.
type Screen struct {
	backups BackupsController
	mobile  MobileSyncController
	gdrive  GDriveController
	logs    *logging.Logger
	toast   func(domain.LogLevel, string, string)

	tab         int
	integrity   string
	integrityOK bool

	// mobile tab drafts, synced to the controller in syncMobile.
	serverOn   bool
	lastOn     bool
	portValue  float64
	lastPort   int
	tokenDraft string
	lastToken  string
	regenOpen  bool

	// gdrive tab drafts and async test/backup state.
	credentialsDraft string
	folderDraft      string
	savedCreds       string
	savedFolder      string
	dirtyCredentials bool
	testRunning      bool
	testFailed       bool
	testMessage      string
	backupRunning    bool

	// backups tab async/confirm state.
	backupBusy  bool
	restorePath string
	restoreOpen bool

	// logs tab state: seenVersion tracks the rendered entry count so
	// auto-scroll only follows genuinely new entries.
	logSearch   string
	logLevel    int
	autoScroll  bool
	seenVersion int64
	logScroll   mui.ScrollState

	// Async mailbox. Workers publish results from goroutines under mu and
	// View drains them on the UI thread; every other field above is
	// UI-thread-only. The mutex pairs each publish with the Invalidate
	// that schedules the frame that reads it.
	mu         sync.Mutex
	backupDone bool
	testResult *connectionResult
	cloudDone  bool
}

// connectionResult carries the outcome of an async connection test.
type connectionResult struct {
	failed  bool
	message string
}

// New creates the settings screen.
func New(opts Options) *Screen {
	s := &Screen{
		backups: opts.Backups,
		mobile:  opts.Mobile,
		gdrive:  opts.GDrive,
		logs:    opts.Logs,
		toast:   opts.Toast,
	}
	if s.mobile != nil {
		s.serverOn = s.mobile.Enabled()
		s.lastOn = s.serverOn
		s.portValue = float64(s.mobile.Port())
		s.lastPort = s.mobile.Port()
		s.tokenDraft = s.mobile.Token()
		s.lastToken = s.tokenDraft
	}
	if s.gdrive != nil {
		s.credentialsDraft, s.folderDraft = s.gdrive.Credentials()
	}
	return s
}

var tabItems = []nav.TabItem{
	{Label: "Backups", Icon: lucide.DatabaseBackup()},
	{Label: "Mobile sync", Icon: lucide.Smartphone()},
	{Label: "Google Drive", Icon: lucide.CloudUpload()},
	{Label: "Logs", Icon: lucide.ScrollText()},
}

// View builds one frame of the settings screen.
func (s *Screen) View(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	s.drainAsync()
	s.syncMobile()
	mui.Column(c).Fill().Padding(0, 28, 40).Gap(16).Children(func() {
		mui.Box(c).Height(1).FillWidth().Background(tk.Border)
		nav.Tabs(c, &s.tab, nav.TabsProps{
			Items:   tabItems,
			Variant: nav.TabsSquare,
			Size:    lobetheme.Small,
		}, func(i int) {
			switch i {
			case 0:
				s.backupTab(c)
			case 1:
				s.mobileTab(c)
			case 2:
				s.gdriveTab(c)
			case 3:
				s.logsTab(c)
			}
		})
	})
}

// drainAsync applies results published by background workers. Called on
// the UI thread at the top of every frame.
func (s *Screen) drainAsync() {
	s.mu.Lock()
	backupDone, testResult, cloudDone := s.backupDone, s.testResult, s.cloudDone
	s.backupDone, s.testResult, s.cloudDone = false, nil, false
	s.mu.Unlock()

	if backupDone {
		s.backupBusy = false
	}
	if testResult != nil {
		s.testRunning = false
		s.testFailed = testResult.failed
		s.testMessage = testResult.message
	}
	if cloudDone {
		s.backupRunning = false
	}
}

// ShowLogsTab activates the logs tab; the shell routes the standalone
// ScreenLogs navigation here.
func (s *Screen) ShowLogsTab() { s.tab = 3 }

// Logs returns the wired logger, for tests and shell assertions.
func (s *Screen) Logs() *logging.Logger { return s.logs }

// raise shows a toast when a notifier is wired. Safe from any goroutine:
// the notifier must be (store.Notify and the test recorder both are).
func (s *Screen) raise(level domain.LogLevel, title, body string) {
	if s.toast != nil {
		s.toast(level, title, body)
	}
}
