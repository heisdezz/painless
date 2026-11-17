# UI Feature 01: Application Shell, Navigation & Notifications

## 1. Overview
The Application Shell is the root container of Painless built on MyGo (`github.com/egoist/mygo` and `github.com/egoist/mygo/ui`). It manages the desktop window, the dark theme styling, the persistent navigation sidebar, the active drive switcher, toast notifications, and modal dialog overlays.

---

## 2. MyGo Native UI Principles

1. **Window Setup (`mygo.NewWindow`)**:
   - Initial size: 1200 x 800.
   - Minimum size: 800 x 600.
   - Pinned state: `StateKey: "painless_main_window"` (remembers geometry across launches).
2. **Context Isolation**:
   - Views receive `*ui.Context` during render passes.
   - `*ui.Context` **must not** be stored in app state or goroutines.
   - Persistent handles (`ui.Handle`) and stable keys (`c.Key(...)`) must be used for all stateful widgets.
3. **Async UI Updates**:
   - Background tasks (scans, drive polling, uploads) communicate results via `window.Update(func())`.

---

## 3. Shell Layout & Structure

```
+--------------------+-----------------------------------------------------+
| [Icon] PAINLESS    | Active Drive: [ Samsung T7 (1.8 TB) v ] [Refresh]   |
+--------------------+-----------------------------------------------------+
| [o] Library        |                                                     |
| [ ] Albums         |                                                     |
| [ ] Discover/Scan  |                 MAIN CONTENT AREA                   |
| [ ] Settings       |                (Active Screen View)                 |
| [ ] Logs           |                                                     |
|                    |                                                     |
| ------------------ |                                                     |
| Drive Storage:     |                                                     |
| [==== 68% ====   ] |                                                     |
| 620 GB free of 2TB |                                                     |
+--------------------+-----------------------------------------------------+
| [Toast / Status Bar: Scan completed - 420 media files indexed]           |
+--------------------------------------------------------------------------+
```

---

## 4. Components

1. **Sidebar Navigation (`internal/ui/shell/sidebar.go`)**:
   - Uses `ui.Sidebar` or a structured `ui.Column` with active route indicator.
   - Navigation targets:
     - `ScreenCatalog` (Media Library)
     - `ScreenAlbums` (Albums Gallery)
     - `ScreenDiscover` (Drive Scanner & Stager)
     - `ScreenSettings` (Configuration & Backups)
     - `ScreenLogs` (System Log Viewer)
2. **Drive Switcher & Storage Meter (`internal/ui/shell/drive_bar.go`)**:
   - Dropdown or flyout menu listing all mounted removable drives.
   - Visual bar displaying used vs free space percentage (`t.Success`, `t.Warning`, `t.Danger` color transitions).
   - "Unmount / Eject" button with confirmation.
3. **Toast Notifications (`internal/ui/shell/toast.go`)**:
   - Uses `ui.Toast` for floating, transient feedback (e.g. "5 files moved to Vacation", "Backup completed", "Drive unmounted").

---

## 5. File Structure
- `internal/ui/shell/window.go`: Window setup, `WhenReady` callback, view switch dispatcher.
- `internal/ui/shell/sidebar.go`: Navigation bar and route buttons.
- `internal/ui/shell/drive_bar.go`: Active drive selector and disk usage gauge.
- `internal/ui/shell/toast.go`: Toast manager and renderer.
- `internal/ui/theme/theme.go`: Dark theme tokens, accent colors, spacing metrics.

---

## 6. Acceptance Criteria
- [ ] Application window launches with dark theme styling and remembers size across restarts.
- [ ] Sidebar switches views instantly without flickers or layout jumps.
- [ ] Drive switcher updates available drives when USB drives are inserted/removed.
- [ ] Toast notifications appear and auto-dismiss after 3 seconds.
