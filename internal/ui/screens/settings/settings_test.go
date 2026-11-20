package settings

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"painless/internal/domain"
	"painless/internal/logging"
	"painless/internal/state"
)

type fakeBackups struct {
	mu         sync.Mutex
	path       string
	size       int64
	created    int
	backups    []BackupInfo
	restoreTo  string
	restoreErr error
}

func (f *fakeBackups) DBPath() string     { return f.path }
func (f *fakeBackups) DBSizeBytes() int64 { return f.size }

func (f *fakeBackups) ListBackups() ([]BackupInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]BackupInfo(nil), f.backups...), nil
}

func (f *fakeBackups) IntegrityCheck() (string, bool) { return "ok", true }

func (f *fakeBackups) CreateBackup() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created++
	name := "backup_20260101_000000.db"
	f.backups = append(f.backups, BackupInfo{Name: name, Path: "/drive/" + name,
		SizeBytes: 1024, CreatedAt: "2026-01-01 00:00"})
	return "/drive/" + name, nil
}

func (f *fakeBackups) RestoreBackup(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restoreTo = path
	return f.restoreErr
}

// waitCreated polls until the async backup landed or the deadline passes.
func (f *fakeBackups) waitCreated(want int) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		n := f.created
		f.mu.Unlock()
		if n >= want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func (f *fakeBackups) waitRestored() string {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		to := f.restoreTo
		f.mu.Unlock()
		if to != "" {
			return to
		}
		time.Sleep(time.Millisecond)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restoreTo
}

type fakeMobile struct {
	port   int
	token  string
	regens int
	calls  []string
}

func (f *fakeMobile) Enabled() bool      { return len(f.calls) > 0 && f.calls[len(f.calls)-1] == "enabled" }
func (f *fakeMobile) SetEnabled(on bool) { f.calls = append(f.calls, "enabled") }
func (f *fakeMobile) Port() int          { return f.port }
func (f *fakeMobile) SetPort(port int)   { f.port = port; f.calls = append(f.calls, "port") }
func (f *fakeMobile) URL() string        { return "http://192.168.1.10:51790" }
func (f *fakeMobile) Token() string      { return f.token }
func (f *fakeMobile) RegenerateToken() string {
	f.regens++
	f.token = "tok-regenerated"
	return f.token
}

type fakeGDrive struct {
	mu        sync.Mutex
	creds     string
	folder    string
	saved     [2]string
	tests     int
	testErr   error
	backups   int
	backupMsg string
	backupErr error
}

func (f *fakeGDrive) Credentials() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.creds, f.folder
}

func (f *fakeGDrive) SetCredentials(json, folderID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = [2]string{json, folderID}
	f.creds, f.folder = json, folderID
}

func (f *fakeGDrive) TestConnection() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tests++
	return f.testErr
}

func (f *fakeGDrive) BackupNow() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.backups++
	return f.backupMsg, f.backupErr
}

func (f *fakeGDrive) waitTests(want int) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		n := f.tests
		f.mu.Unlock()
		if n >= want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func (f *fakeGDrive) waitBackups(want int) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		n := f.backups
		f.mu.Unlock()
		if n >= want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

type toastLog struct {
	level domain.LogLevel
	title string
	body  string
}

type toastRecorder struct {
	mu      sync.Mutex
	entries []toastLog
}

func (r *toastRecorder) toast(level domain.LogLevel, title, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, toastLog{level, title, body})
}

func (r *toastRecorder) last() toastLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) == 0 {
		return toastLog{}
	}
	return r.entries[len(r.entries)-1]
}

func newFullScreen() (*Screen, *fakeBackups, *fakeMobile, *fakeGDrive, *toastRecorder) {
	backups := &fakeBackups{path: "/drive/.media_library.db", size: 4096}
	mobile := &fakeMobile{port: 51790, token: "tok-1"}
	gdrive := &fakeGDrive{creds: "{}", folder: "folder-1", backupMsg: "uploaded"}
	toasts := &toastRecorder{}
	screen := New(Options{
		Backups: backups,
		Mobile:  mobile,
		GDrive:  gdrive,
		Logs:    logging.New(logging.Options{Capacity: 16}),
		Toast:   toasts.toast,
	})
	return screen, backups, mobile, gdrive, toasts
}

func TestSettingsTabSwitching(t *testing.T) {
	screen, _, _, _, _ := newFullScreen()
	tt := mui.NewTester(screen.View, 1200, 800)

	if !tt.HasText("Database health") {
		t.Fatalf("backups tab not shown initially: %q", tt.Texts())
	}
	for _, tab := range []string{"Mobile sync", "Google Drive", "Logs"} {
		if err := tt.Click(tab); err != nil {
			t.Fatalf("click %q: %v", tab, err)
		}
		tt.Frame()
	}
	if screen.tab != 3 {
		t.Fatalf("tab = %d, want 3", screen.tab)
	}
	if !tt.HasText("Auto-scroll") {
		t.Fatalf("logs tab not shown: %q", tt.Texts())
	}
}

func TestSettingsMobileSyncsToggleAndPort(t *testing.T) {
	screen, _, mobile, _, toasts := newFullScreen()
	tt := mui.NewTester(screen.View, 1200, 800)

	screen.serverOn = true
	tt.Frame()
	if len(mobile.calls) == 0 || mobile.calls[0] != "enabled" {
		t.Fatalf("SetEnabled(true) not applied: calls = %v", mobile.calls)
	}
	if got := toasts.last(); got.title != "Mobile sync started" {
		t.Fatalf("last toast = %+v", got)
	}

	screen.portValue = 1234
	tt.Frame()
	if mobile.port != 1234 {
		t.Fatalf("port = %d, want 1234", mobile.port)
	}

	// Token changes on the controller flow back into the masked field.
	mobile.token = "tok-2"
	tt.Frame()
	if screen.tokenDraft != "tok-2" {
		t.Fatalf("tokenDraft = %q, want tok-2", screen.tokenDraft)
	}
}

func TestSettingsRegenerateToken(t *testing.T) {
	screen, _, mobile, _, _ := newFullScreen()
	tt := mui.NewTester(screen.View, 1200, 800)

	if err := tt.Click("Mobile sync"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Regenerate"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !screen.regenOpen {
		t.Fatal("confirm modal did not open")
	}
	if !tt.HasText("Regenerate write token?") {
		t.Fatalf("modal title missing: %q", tt.Texts())
	}
	if err := tt.Click("Regenerate token"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if mobile.regens != 1 {
		t.Fatalf("regens = %d, want 1", mobile.regens)
	}
	if screen.tokenDraft != "tok-regenerated" {
		t.Fatalf("tokenDraft = %q", screen.tokenDraft)
	}
	if screen.regenOpen {
		t.Fatal("modal left open")
	}
}

func TestSettingsCreateBackup(t *testing.T) {
	screen, backups, _, _, toasts := newFullScreen()
	tt := mui.NewTester(screen.View, 1200, 800)

	if err := tt.Click("Create backup"); err != nil {
		t.Fatal(err)
	}
	if !backups.waitCreated(1) {
		t.Fatal("CreateBackup not called")
	}
	for screen.backupBusy {
		time.Sleep(time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("backup_20260101_000000.db") {
		t.Fatalf("snapshot row missing: %q", tt.Texts())
	}
	if got := toasts.last(); got.title != "Backup created" {
		t.Fatalf("last toast = %+v", got)
	}
}

func TestSettingsRestoreBackup(t *testing.T) {
	screen, backups, _, _, _ := newFullScreen()
	backups.mu.Lock()
	backups.backups = []BackupInfo{{
		Name: "old.db", Path: "/drive/old.db", SizeBytes: 512, CreatedAt: "2025-12-31 23:59",
	}}
	backups.mu.Unlock()
	tt := mui.NewTester(screen.View, 1200, 800)

	if err := tt.Click("Restore"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !screen.restoreOpen || screen.restorePath != "/drive/old.db" {
		t.Fatalf("restore state = open %v path %q", screen.restoreOpen, screen.restorePath)
	}
	if err := tt.Click("Restore backup"); err != nil {
		t.Fatal(err)
	}
	if got := backups.waitRestored(); got != "/drive/old.db" {
		t.Fatalf("restoreTo = %q", got)
	}
}

func TestSettingsIntegrityTag(t *testing.T) {
	screen, _, _, _, _ := newFullScreen()
	tt := mui.NewTester(screen.View, 1200, 800)

	if !tt.HasText("Not verified") {
		t.Fatalf("integrity placeholder missing: %q", tt.Texts())
	}
	if err := tt.Click("Verify integrity"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !screen.integrityOK || !tt.HasText("OK") {
		t.Fatalf("integrity = %q ok=%v", screen.integrity, screen.integrityOK)
	}
}

func TestSettingsGDriveFlow(t *testing.T) {
	screen, _, _, gdrive, toasts := newFullScreen()
	tt := mui.NewTester(screen.View, 1200, 800)

	if err := tt.Click("Google Drive"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Destination folder ID") {
		t.Fatalf("gdrive tab missing: %q", tt.Texts())
	}

	// Dirty drafts show the save action.
	screen.credentialsDraft = `{"client_email":"x"}`
	tt.Frame()
	if !screen.dirtyCredentials {
		t.Fatal("edits not detected as dirty")
	}
	if err := tt.Click("Save credentials"); err != nil {
		t.Fatal(err)
	}
	gdrive.mu.Lock()
	savedJSON, savedFolder := gdrive.saved[0], gdrive.saved[1]
	gdrive.mu.Unlock()
	if savedJSON != `{"client_email":"x"}` || savedFolder != "folder-1" {
		t.Fatalf("saved = %q / %q", savedJSON, savedFolder)
	}

	// A failing test surfaces its error as a red status.
	gdrive.mu.Lock()
	gdrive.testErr = errors.New("bad credentials")
	gdrive.mu.Unlock()
	if err := tt.Click("Test connection"); err != nil {
		t.Fatal(err)
	}
	if !gdrive.waitTests(1) {
		t.Fatal("TestConnection not called")
	}
	for screen.testRunning {
		time.Sleep(time.Millisecond)
		tt.Frame()
	}
	if !screen.testFailed || !tt.HasText("bad credentials") {
		t.Fatalf("testFailed = %v, texts = %q", screen.testFailed, tt.Texts())
	}

	if err := tt.Click("Backup library to Google Drive"); err != nil {
		t.Fatal(err)
	}
	if !gdrive.waitBackups(1) {
		t.Fatal("BackupNow not called")
	}
	for screen.backupRunning {
		time.Sleep(time.Millisecond)
		tt.Frame()
	}
	if got := toasts.last(); got.title != "Google Drive backup done" {
		t.Fatalf("last toast = %+v", got)
	}
}

func TestSettingsLogsFilterAndExport(t *testing.T) {
	screen, _, _, _, toasts := newFullScreen()
	log := screen.logs
	log.Info("scanner", "walked %d files", 12)
	log.Warn("scanner", "hash mismatch on cat.jpg")
	log.Error("thumbnail", "ffmpeg exited 1")
	log.Info("drive", "drive mounted")

	tt := mui.NewTester(screen.View, 1200, 800)
	if err := tt.Click("Logs"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("walked 12 files") || !tt.HasText("ffmpeg exited 1") {
		t.Fatalf("log rows missing: %q", tt.Texts())
	}

	// Level filter keeps only warns.
	screen.logLevel = 2
	tt.Frame()
	if tt.HasText("walked 12 files") || !tt.HasText("hash mismatch on cat.jpg") {
		t.Fatalf("warn filter texts = %q", tt.Texts())
	}

	// Search filters across message and context.
	screen.logLevel = 0
	screen.logSearch = "thumbnail"
	tt.Frame()
	if !tt.HasText("ffmpeg exited 1") || tt.HasText("drive mounted") {
		t.Fatalf("search filter texts = %q", tt.Texts())
	}

	if err := tt.Click("Export logs…"); err != nil {
		t.Fatal(err)
	}
	got := toasts.last()
	if got.title != "Logs exported" || !strings.Contains(got.body, "painless-logs") {
		t.Fatalf("last toast = %+v", got)
	}
}

func TestSettingsEmptyControllers(t *testing.T) {
	screen := New(Options{})
	tt := mui.NewTester(screen.View, 1200, 800)
	for _, want := range []string{"Backups unavailable", "Mobile sync unavailable",
		"Google Drive unavailable", "Logs unavailable"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %q", want, tt.Texts())
		}
		if err := tt.Click(tabAfter(want)); err != nil {
			t.Fatalf("click: %v", err)
		}
		tt.Frame()
	}
}

// tabAfter returns the tab label that switches to the screen showing the
// given empty-state title.
func tabAfter(emptyTitle string) string {
	switch emptyTitle {
	case "Backups unavailable":
		return "Mobile sync"
	case "Mobile sync unavailable":
		return "Google Drive"
	case "Google Drive unavailable":
		return "Logs"
	}
	return "Backups"
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{5 * 1024 * 1024, "5.0 MiB"},
	}
	for _, tc := range cases {
		if got := formatBytes(tc.in); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGeneralTabTogglesAutoplay(t *testing.T) {
	store := state.NewStore()
	tt := mui.NewTester(New(Options{Store: store}).View, 1000, 800)
	if err := tt.Click("General"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if !tt.HasText("Autoplay videos") {
		t.Fatalf("missing autoplay switch label; texts = %q", tt.Texts())
	}
	var switches []mui.AccessInfo
	for _, info := range tt.AccessAll() {
		if info.Role == mui.RoleSwitch {
			switches = append(switches, info)
		}
	}
	if len(switches) < 2 {
		t.Fatalf("need two General switches, got %d", len(switches))
	}
	autoplay := switches[1]
	tt.ClickAt(autoplay.Bounds.X+autoplay.Bounds.W/2, autoplay.Bounds.Y+autoplay.Bounds.H/2)
	tt.Frame()

	if store.AutoplayEnabled() {
		t.Fatal("AutoplayEnabled = true after clicking the switch, want false")
	}
}

func TestGeneralTabTogglesThumbnails(t *testing.T) {
	store := state.NewStore()
	tt := mui.NewTester(New(Options{Store: store}).View, 1000, 800)
	if !tt.HasText("General") {
		t.Fatalf("general tab texts = %q", tt.Texts())
	}

	if err := tt.Click("General"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if !tt.HasText("Display thumbnails") {
		t.Fatalf("missing switch label; texts = %q", tt.Texts())
	}

	info, ok := tt.AccessByRole(mui.RoleSwitch, "")
	if !ok {
		t.Fatalf("no switch rendered; texts = %q", tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Frame()

	if store.ThumbnailsEnabled() {
		t.Fatal("ThumbnailsEnabled = true after clicking the switch, want false")
	}
}
