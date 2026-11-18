# UI Feature 01: Application Shell, Navigation & Notifications

## 1. Overview

The Application Shell is the root container of Painless built on MyGo (`github.com/egoist/mygo/ui`) and Lobe MyGo (`github.com/lobehub/lobe-mygo`). It manages the desktop window, the Lobe design tokens/theme, the persistent collapsible sidebar navigation (`layout.DraggableSideNav`), the active drive switcher, toast notifications (`feedback.Toast`), and modal dialog overlays (`overlay.Modal`).

---

## 2. MyGo & Lobe MyGo Native UI Principles

1. **Window Setup (`mygo.NewWindow`)**:
   - Initial size: 1200 x 800.
   - Minimum size: 800 x 600.
   - Pinned state: `StateKey: "painless_main_window"` (remembers geometry across launches).
   - Custom decoration: the view is the window's decoration. On Linux the window is `Frameless` (a hidden title bar would keep the compositor's own title bar where the compositor decorates windows itself, as KWin does); on Windows and macOS it uses `TitleBarStyle: mygo.TitleBarHidden` with `HideWindowButtons: true`, which keeps the shadow, corners and resizable top edge. Either way the shell's top bar is the title bar — a 52px row marked `DragWindow()` (double click zooms) that ends in the drawn minimize / maximize / close controls (`shell.windowControls`, marked `MaximizeButton()` for Windows 11 snap layouts).
2. **Lobe Theme Tokens**:
   - Use `github.com/lobehub/lobe-mygo/pkg/theme` for surface colors, border strokes, and typography.
   - Consistent dark/light adaptation with GPU-drawn components.
3. **Context Isolation**:
   - Views receive `*ui.Context` during render passes.
   - `*ui.Context` **must not** be stored in app state or background goroutines.
   - Background tasks (scans, drive polling, uploads) communicate results via `window.Update(func())`.

---

## 3. Shell Layout & Structure

```
+--------------------------------------------------------------------------+
| [HardDrive Icon] PAINLESS   Active Drive: [ Samsung T7 (1.8 TB) v ] [Sync] |
+--------------------+-----------------------------------------------------+
| [layout.           |                                                     |
|  DraggableSideNav] |                                                     |
|                    |                                                     |
| [o] Library        |                 MAIN CONTENT AREA                   |
| [ ] Albums         |                (Active Screen View)                 |
| [ ] Discover/Scan  |                                                     |
| [ ] Settings       |                                                     |
| [ ] Logs           |                                                     |
|                    |                                                     |
| ------------------ |                                                     |
| Storage Gauge:     |                                                     |
| [feedback.Progress]|                                                     |
| 620 GB free of 2TB |                                                     |
+--------------------+-----------------------------------------------------+
| [feedback.Toast: Scan completed - 420 media files indexed]               |
+--------------------------------------------------------------------------+
```

---

## 4. Components

1. **Draggable Side Navigation (`internal/ui/shell/sidebar.go`)**:
   - Built on `github.com/lobehub/lobe-mygo/pkg/layout.DraggableSideNav`.
   - Collapses to a slim icon rail (58 DIP) or expands with labels.
   - Navigation targets using `github.com/lobehub/lobe-mygo/pkg/lucide` icons:
     - `ScreenCatalog`: `lucide.FolderHeart` (Media Library)
     - `ScreenAlbums`: `lucide.Images` (Albums Gallery)
     - `ScreenDiscover`: `lucide.ScanSearch` (Drive Scanner & Stager)
     - `ScreenSettings`: `lucide.Settings` (Configuration & Backups)
     - `ScreenLogs`: `lucide.ScrollText` (System Log Console)
2. **Drive Switcher & Storage Gauge (`internal/ui/shell/drive_bar.go`)**:
   - Drive selector powered by `github.com/lobehub/lobe-mygo/pkg/field.Select`.
   - Visual disk usage bar using `github.com/lobehub/lobe-mygo/pkg/feedback.Progress` (with dynamic status colors for >85% and >95% full).
   - "Refresh Drives" action using `github.com/lobehub/lobe-mygo/pkg/action.ActionIcon`.
3. **Toast Notifications (`internal/ui/shell/toast.go`)**:
   - Powered by `github.com/lobehub/lobe-mygo/pkg/feedback.Toast`.
   - Brief notifications that stack in a corner, peek behind the newest, and spread out under the pointer with status tints (`Success`, `Warn`, `Error`).

---

## 5. File Structure

- `internal/ui/shell/window.go`: Window initialization, `WhenReady` callback, view dispatcher.
- `internal/ui/shell/sidebar.go`: `layout.DraggableSideNav` implementation with Lucide icons.
- `internal/ui/shell/drive_bar.go`: `field.Select` drive picker and `feedback.Progress` gauge.
- `internal/ui/shell/toast.go`: Toast manager wrapper around `feedback.Toast`.
- `internal/ui/theme/theme.go`: Lobe design tokens configuration.

---

## 6. Acceptance Criteria

- [ ] Application window launches with Lobe dark styling and remembers size across restarts.
- [ ] `DraggableSideNav` resizes smoothly and collapses to slim rail mode.
- [ ] Drive switcher updates dynamically on USB drive insertion/removal.
- [ ] `feedback.Toast` notifications stack cleanly and dismiss automatically.
