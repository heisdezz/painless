# UI Feature 06: Settings, Backup Tools & System Logs Viewer

## 1. Overview
The Settings & Tools screen centralizes database backups, Google Drive integration, Mobile Sync server configuration, and live application diagnostic logs.

---

## 2. Sections & Tabs

### 1. Database & Backup Management (`internal/ui/screens/settings/backup_tab.go`)
- **Active Library**: Shows database path, file size, integrity status, and WAL checkpoint status.
- **Actions**:
  - "Create Local Backup": Creates instant online backup in `.media_backups/`.
  - "Verify Database Integrity": Runs `PRAGMA integrity_check`.
- **Backup List Table**:
  - Displays existing backups with timestamps and sizes.
  - "Restore" button with safety confirmation dialog.

### 2. Mobile Companion Sync Setup (`internal/ui/screens/settings/mobile_sync_tab.go`)
- Toggle switch: Enable/Disable Mobile Sync Server.
- Port configuration input (default: `8080`).
- Local LAN connection status:
  - Displays pairing URL (e.g. `http://192.168.1.150:8080`).
  - QR Code rendering for instant mobile device connection.
- Write Token configuration:
  - Masked input field with "Regenerate Token" and "Copy to Clipboard" buttons.

### 3. Google Drive Cloud Backup (`internal/ui/screens/settings/gdrive_tab.go`)
- Service Account Credentials JSON input (or file picker to load `credentials.json`).
- Target Google Drive Folder ID input.
- "Test Connection" button with status indicator (Green/Red badge).
- "Backup Library to Google Drive" action button.

### 4. System Logs Console (`internal/ui/screens/settings/logs_tab.go`)
- Virtualized table of in-memory logs (`ui.Table`).
- Filter logs by level: `All`, `Info`, `Warn`, `Error`.
- Live auto-scroll toggle.
- Search input to filter log messages.
- "Export Logs to File" button.

---

## 3. File Structure
- `internal/ui/screens/settings/settings_screen.go`: Tabbed container for settings.
- `internal/ui/screens/settings/backup_tab.go`: Local database backups view.
- `internal/ui/screens/settings/mobile_sync_tab.go`: Mobile server pairing & token view.
- `internal/ui/screens/settings/gdrive_tab.go`: Google Drive configuration view.
- `internal/ui/screens/settings/logs_tab.go`: Live diagnostic log table.

---

## 4. Acceptance Criteria
- [ ] Database backups can be created and listed with exact timestamps.
- [ ] Mobile sync server can be toggled on/off with live port validation.
- [ ] Google Drive credentials test returns clear diagnostics on success or failure.
- [ ] System logs update in real time with level filtering and search.
