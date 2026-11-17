# UI Feature 06: Settings, Backup Tools & System Logs Viewer

## 1. Overview

The Settings & Tools screen centralizes database backups, Google Drive integration, Mobile Sync server configuration, and live diagnostic logs. It uses Lobe UI tabs (`nav.Tabs`), QR code generator (`display.QRCode`), copy buttons (`action.CopyButton`), password inputs (`field.InputPassword`), switches (`field.Switch`), and empty states (`display.Empty`).

---

## 2. Sections & Tabs (`nav.Tabs`)

Uses `github.com/lobehub/lobe-mygo/pkg/nav.Tabs` with sliding indicator:

1. `TabBackups`: Local SQLite Database Backups
2. `TabMobileSync`: Mobile Companion Pairing & Server Controls
3. `TabGDrive`: Google Drive Cloud Backup
4. `TabLogs`: Live System Logs Console

---

## 3. Tab Implementations

### 1. Database & Backup Management (`internal/ui/screens/settings/backup_tab.go`)

- Database health card (`layout.Block`): Path, file size, integrity status (`display.Tag`).
- Actions:
  - `action.Button`: "Create Local Backup" (triggers instant online backup to `.media_backups/`).
  - `action.Button`: "Verify Database Integrity" (runs `PRAGMA integrity_check`).
- Backups Table:
  - Lists existing backups with timestamps and sizes.
  - `action.Button`: "Restore" (opens safety confirmation `overlay.Modal`).

### 2. Mobile Companion Sync Setup (`internal/ui/screens/settings/mobile_sync_tab.go`)

- Server Toggle: `github.com/lobehub/lobe-mygo/pkg/field.Switch` to enable/disable HTTP server.
- Port Configuration: `field.InputNumber` (default: `8080`).
- LAN Pairing Card:
  - Displays local IP URL (e.g. `http://192.168.1.150:8080`).
  - `action.CopyButton`: One-click copy of URL to clipboard.
  - `github.com/lobehub/lobe-mygo/pkg/display.QRCode`: Visual QR code for instant mobile camera pairing.
- Write Token Authentication:
  - `field.InputPassword`: Displays masked token with toggle to reveal.
  - `action.CopyButton`: Copies write token.
  - `action.Button`: "Regenerate Token" with confirmation prompt.

### 3. Google Drive Cloud Backup (`internal/ui/screens/settings/gdrive_tab.go`)

- Service Account Credentials:
  - `field.TextArea`: Paste service account JSON (or "Browse JSON File..." button).
- Folder ID:
  - `field.Input`: Target Google Drive folder ID.
- Actions:
  - `action.Button`: "Test Connection" (displays `feedback.Spin` while testing, then success/error `display.Tag`).
  - `action.Button`: "Backup Library to Google Drive" (Primary variant).

### 4. System Logs Console (`internal/ui/screens/settings/logs_tab.go`)

- Virtualized log list with color-coded severity:
  - `display.Tag`: Info (blue), Warn (yellow), Error (red).
- Controls:
  - `field.SearchBar`: Live text search filtering log messages.
  - `choice.Segmented`: Filter by level (`All`, `Info`, `Warn`, `Error`).
  - `field.Switch`: "Auto-scroll to latest".
  - `action.Button`: "Export Logs..."

---

## 4. File Structure

- `internal/ui/screens/settings/settings_screen.go`: Tabbed container using `nav.Tabs`.
- `internal/ui/screens/settings/backup_tab.go`: Local database backups view.
- `internal/ui/screens/settings/mobile_sync_tab.go`: `display.QRCode` pairing and token view.
- `internal/ui/screens/settings/gdrive_tab.go`: Google Drive credentials configuration.
- `internal/ui/screens/settings/logs_tab.go`: Searchable diagnostic logs viewer.

---

## 5. Acceptance Criteria

- [ ] `nav.Tabs` switches between settings sections smoothly.
- [ ] `display.QRCode` generates a scannable pairing code for mobile devices.
- [ ] `action.CopyButton` copies server URLs and tokens with checkmark feedback.
- [ ] Google Drive connection test provides immediate status feedback.
- [ ] System logs filter by severity level and keyword with auto-scroll toggle.
