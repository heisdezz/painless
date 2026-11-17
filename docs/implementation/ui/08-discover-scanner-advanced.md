# UI Feature 08: Discover & Scaffolding Advanced Suite

## 1. Overview

The Discover Screen in Painless must faithfully replicate the full functionality and visual structure of the original application (Screenshot 3 & `discover.index.tsx`). It provides:
1. **Active Drive Banner**: Volume name, storage capacity, external/internal tag, and mount point badge.
2. **Album Folder Configuration Card**: Displays the current storage location for albums, thumbnails, and SQLite database. Provides `[Rescan album folder]` (with pause/resume/stop lifecycle and database recovery) and `[Choose folder]` (custom directory picker).
3. **Scan Folder for Media Panel**:
   - Path traversal input with `[Browse]` and primary `[Start Scan]` button.
   - Comprehensive **Folder & Path Ignorelist** engine:
     - Badge showing custom ignore rule count with a `[Reset Defaults]` action.
     - Add ignore pattern input with `[Browse]` folder picker and `[+ Add]` action.
     - `QUICK PRESETS` chips: `+ temp`, `+ cache`, `+ raw`, `+ backups`, `+ archive`.
     - Active ignore rules tags/chips: default protected rules (`albums/`, `node_modules/`, `.* (hidden)`, `lost+found`) plus removable custom rules.
4. **Real-time Scanner Status Card**:
   - Status indicators: `State` (IDLE, SCANNING, PAUSED), `Checked Files`, `Active Ignore Rules` (color-coded tag), and `SQLite Cataloged` count (color-coded tag).
   - Live controls: Pause, Resume, Stop.
5. **Scaffolding Health Check & Repair**:
   - Automated check of directory structure (`.painless/`, `media/albums/`, `unknown/`).
   - Non-blocking banner with retry and rescan recovery options.

---

## 2. Visual Architecture & Wireframe

```
+--------------------------------------------------------------------------+
| Drive Manager / /discover                                                |
|                                                                          |
| [Drive Icon] TOSHIBA MQ01ABD100                 [/run/media/destiny/Ventoy]|
| 932 GB · EXTERNAL                                                        |
+--------------------------------------------------------------------------+
|                                                                          |
| [Folder Icon] Album folder              [Rescan album folder] [Choose ...]
| Albums, thumbnails, and the catalog for this drive are stored here.      |
| /run/media/destiny/Ventoy/media/albums                                   |
| Rescan repairs the catalog in the selected folder.                       |
+--------------------------------------------------------------------------+
|                                      |                                   |
| SCAN FOLDER FOR MEDIA                | SCANNER STATUS                    |
| FOLDER PATH TO TRAVERSE              |                                   |
| [/run/media/destiny/Ventoy  ][Browse]| State:                       IDLE |
| [Start Scan]                         | Checked Files:                  0 |
|                                      | Active Ignore Rules:      5 rules |
| Folder & Path Ignorelist [5 custom]  | SQLite Cataloged:           15278 |
| [Add folder/path       ][Browse][Add]|                                   |
| QUICK PRESETS: [+cache] [+raw] ...   | [Pause / Resume]   [Stop Scan]    |
| [albums/] [node_modules/] [temp]     |                                   |
+--------------------------------------+-----------------------------------+
```

---

## 3. UI Component Breakdown & Lobe Tokens

1. **Drive Banner**:
   - Container: `mui.Row(c).Padding(16, 20).Radius(14).Background(tk.BgContainer).Border(1, tk.BorderSecondary)`.
   - Left: Green drive icon (`lucide.HardDrive()` or `lucide.Usb()`), bold title, size and drive type tags.
   - Right: Mount path tag with `font-mono` styling.

2. **Album Folder Configuration Card**:
   - Container: `mui.Column(c).Padding(20).Radius(14).Background(tk.BgContainer).Border(1, tk.BorderSecondary)`.
   - Header with `lucide.Folder()`, title "Album folder", and action buttons:
     - `Rescan album folder`: `action.Button` with `lucide.RefreshCw()`. When rescan is active, shows progress text (`Checking files: N`, `Added: X`, `Updated: Y`, `Recovered database`) and Pause/Resume/Stop controls.
     - `Choose folder`: `action.Button` with `lucide.FolderOpen()` invoking directory picker.
   - Path text: `mui.Text(c, albumPath).FontMono().FontSize(12).TextColor(tk.TextSecondary)`.
   - Descriptive helper text explaining that files and databases remain intact when switching catalogs.

3. **Scan Folder for Media (Left Panel)**:
   - Target path input: `field.Input` with `lucide.Folder()` prefix.
   - Native directory picker trigger: `action.Button` ("Browse", `lucide.FolderOpen()`).
   - Primary `[Start Scan]` button (`action.Primary`, green surface, `lucide.Play()`).
   - **Ignorelist Section**:
     - Header: `lucide.FolderMinus()`, "Folder & Path Ignorelist", custom count badge (`display.Tag` with warning tint), and `action.Button` ("Reset Defaults", variant: `action.Text`, icon: `lucide.RotateCcw()`).
     - Subtitle helper: "The scanner will automatically bypass matching folder names or path segments during discovery."
     - Add rule input row: `field.Input` + `Browse` button + `[+ Add]` button (`action.Button`).
     - Preset suggestion row: clickable preset tags (`+ temp`, `+ cache`, `+ raw`, `+ backups`, `+ archive`).
     - Active rules tag cloud:
       - Protected system rules: `albums/`, `node_modules/`, `.* (hidden)`, `lost+found` with `lucide.Shield()` icons.
       - Custom rules: `display.Tag` with `Closable: true` or inline close action to remove from active ignore list.

4. **Scanner Status Panel (Right Panel)**:
   - Container: `mui.Column(c).Padding(20).Radius(14).Background(tk.BgContainer).Border(1, tk.BorderSecondary)`.
   - Key-value rows:
     - `State`: `IDLE` (gray tag), `SCANNING` (blue pulsing tag), or `PAUSED` (warning tag).
     - `Checked Files`: integer counter formatted with commas.
     - `Active Ignore Rules`: formatted in yellow warning text.
     - `SQLite Cataloged`: total indexed items count formatted in green success text.
   - Control buttons:
     - Pause/Resume: `action.Button` (`lucide.Pause()` / `lucide.Play()`).
     - Stop: `action.Button` (`lucide.Square()`, danger tint).

5. **Scaffolding Health Check**:
   - Automatically verifies `.painless/painless.db` and folder integrity upon drive mount.
   - Displays error banner with retry options if filesystem permissions or database locks fail.

---

## 4. State & Service Contract

```go
package discover

import (
	"context"

	"painless/internal/domain"
	"painless/internal/scanner"
)

// AlbumLocationManager manages the configurable album folder on an external drive.
type AlbumLocationManager interface {
	GetAlbumLocation(driveRoot string) (string, error)
	SetAlbumLocation(driveRoot string, newFolder string) error
	StartAlbumRescan(ctx context.Context, driveRoot string) error
	PauseAlbumRescan(driveRoot string, paused bool) error
	StopAlbumRescan(driveRoot string) error
	GetAlbumRescanStatus(driveRoot string) (*AlbumRescanStatus, error)
}

// AlbumRescanStatus reports repair and re-indexing progress.
type AlbumRescanStatus struct {
	Scanning          bool
	Paused            bool
	Stopped           bool
	Checked           int
	Added             int
	Updated           int
	Missing           int
	Albums            int
	RecoveredDatabase bool
	Error             string
}

// IgnoreListManager manages scanner ignore patterns.
type IgnoreListManager interface {
	GetIgnoreList() []string
	AddIgnore(pattern string)
	RemoveIgnore(pattern string)
	ResetDefaults()
}
```

---

## 5. File Structure & Responsibilities

- `internal/ui/screens/discover/discover.go`: Screen coordinator, layout assembly for Album Folder, Scanner, and Ignorelist panels.
- `internal/ui/screens/discover/album_folder_card.go`: Album folder path display, rescan repair controls, and folder selector.
- `internal/ui/screens/discover/scan_controls.go`: Path input, browse dialog, and start/pause/stop scanner controls.
- `internal/ui/screens/discover/ignore_list.go`: Ignore rule input, native browse picker, quick preset chips, and tag cloud.
- `internal/ui/screens/discover/scanner_status.go`: Live telemetry card (State, Checked Files, Active Rules, Cataloged count).
- `internal/ui/screens/discover/scaffold_banner.go`: Scaffolding verification banner and retry options.
- `internal/ui/screens/discover/discover_test.go`: Unit tests for album folder configuration, ignore list management, presets, and live scanner status updates.
