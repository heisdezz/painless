# UI Feature 07: Drive Selection & Dashboard Home Screen

## 1. Overview

The Main Entry / Dashboard Home screen is the primary landing view of Painless, matching the source application's root route (`/`). When no drive is selected, it presents a drive selection grid where users can inspect connected storage volumes and select or mount a drive. Once an active drive is selected, it renders a drive overview dashboard with summary stats (total media count, album count, disk storage usage gauge), quick actions (`Scan Volume`, `Refresh`, `Disconnect`), and horizontal preview carousels for recently added media and recent albums.

---

## 2. Visual Architecture & Wireframe

### Case A: No Drive Selected (Drive Picker View)
```
+--------------------------------------------------------------------------+
| SELECT A CONNECTED DRIVE                                  [Refresh Drives]|
|                                                                          |
| +-------------------------+  +-------------------------+                 |
| | [USB] TOSHIBA MQ01...   |  | [Disk] Samsung T7       |                 |
| | 932 GB        [Mounted] |  | 1.8 TB        [Mounted] |                 |
| | ======================= |  | ======================= |                 |
| | 0% used     932 GB total|  | 42% used    1.8 TB total|                 |
| +-------------------------+  +-------------------------+                 |
+--------------------------------------------------------------------------+
```

### Case B: Drive Selected & Mounted (Dashboard Overview)
```
+--------------------------------------------------------------------------+
| [USB Icon] TOSHIBA MQ01ABD100                      [Scan Volume] [R] [Off]|
| /RUN/MEDIA/DESTINY/VENTOY · 932 GB TOTAL                                 |
| Album folder: /run/media/destiny/Ventoy/media/albums                     |
+--------------------------------------------------------------------------+
|                                                                          |
| +------------------+   +------------------+   +------------------------+ |
| | [ImageIcon]      |   | [FolderIcon]     |   | STORAGE USED        0% | |
| | TOTAL MEDIA      |   | TOTAL ALBUMS     |   | ====================== | |
| | 15278            |   | 0                |   | 932 GB total capacity  | |
| +------------------+   +------------------+   +------------------------+ |
|                                                                          |
| RECENTLY ADDED MEDIA                                    View All Media ->|
| +--------+  +--------+  +--------+  +--------+  +--------+  +--------+   |
| | [IMG]  |  | [IMG]  |  | [IMG]  |  | [IMG]  |  | [IMG]  |  | [IMG]  |   |
| | 3.12:0 |  | 3.12:0 |  | 3.12:0 |  | 3.12:0 |  | 3.12:0 |  | 3.12:0 |   |
| +--------+  +--------+  +--------+  +--------+  +--------+  +--------+   |
|                                                                          |
| RECENT ALBUMS                                          View All Albums ->|
| (Horizontal list of album cards or "Loading albums..." spinner)          |
+--------------------------------------------------------------------------+
```

---

## 3. UI Component Breakdown & Lobe Tokens

1. **Drive Selector Cards Grid**:
   - Filter out system root (`/`) drives unless explicitly configured.
   - For each drive:
     - Card container: `mui.Column(c).Radius(16).Background(tk.BgContainer).Border(1, tk.BorderSecondary)`.
     - Drive icon: `lucide.Usb()` for removable/external, `lucide.HardDrive()` for internal, `lucide.Globe()` for network/cloud.
     - Status badge: `display.Tag` (`"Mounted"`, color: `display.TagSuccess`, or `"Unmounted"`, color: `display.TagDefault`).
     - Capacity progress bar: `feedback.Progress(c, percent, feedback.ProgressProps{ ... })` color-coded by thresholds (<50% success, 50-80% primary, >80% error).
     - Click interaction: invokes `store.SetActiveDrive(drive.ID)` and navigates to the dashboard view.

2. **Drive Header Card**:
   - Green squircle container with drive icon (`tk.ColorSuccess`).
   - Drive model name (`TOSHIBA MQ01ABD100`) in font size 18, bold.
   - Subtitle: uppercase path and formatted total bytes.
   - Album folder indicator: path in font mono.
   - Action buttons:
     - `Scan Volume` (`action.Button` with `lucide.Compass()`, variant: `action.Default`), navigates to `ScreenDiscover`.
     - `Refresh` (`action.ActionIcon` with `lucide.RefreshCw()`).
     - `Disconnect` (`action.ActionIcon` with `lucide.Power()`, danger tint), unsets active drive.

3. **Three Stat Cards**:
   - `Total Media Items`: Icon `lucide.Image()`, total media count formatted as large bold text.
   - `Total Albums`: Icon `lucide.Folder()`, total album count.
   - `Storage Used`: Percentage label, usage progress bar, total disk capacity text.

4. **Recently Added Media Carousel**:
   - Top header with `RECENTLY ADDED MEDIA` uppercase tracking and `View All Media ->` action button navigating to `ScreenCatalog`.
   - Grid or horizontal scroll row of 8 items with thumbnails via `Thumbnailer` service.
   - Overlay badge with folder or album key.
   - Click opens media viewer (`viewer.OpenItem`).

5. **Recent Albums Carousel**:
   - Top header with `RECENT ALBUMS` and `View All Albums ->` action button navigating to `ScreenAlbums`.
   - Album cards displaying cover thumbnail, album title, media count tag, and creation date.

---

## 4. State & Service Contract

```go
package dashboard

import (
	"context"

	"painless/internal/domain"
)

// Source feeds media and album records for the dashboard.
type Source interface {
	GetDashboardStats(ctx context.Context) (Stats, error)
	GetRecentMedia(ctx context.Context, limit int) ([]domain.MediaItem, error)
	GetRecentAlbums(ctx context.Context, limit int) ([]domain.AlbumSummary, error)
}

// DriveSource lists connected drives for the drive picker.
type DriveSource interface {
	ListDrives(ctx context.Context) ([]domain.DriveInfo, error)
	SelectDrive(driveID string) error
	RefreshDrives(ctx context.Context) error
}

// Stats holds aggregate numbers for the dashboard cards.
type Stats struct {
	TotalMedia    int64
	TotalAlbums   int64
	StorageUsed   uint64
	StorageTotal  uint64
	UsedPercent   float64
	AlbumLocation string
}
```

---

## 5. File Structure & Responsibilities

- `internal/ui/screens/dashboard/dashboard.go`: Screen controller, view router between Drive Picker and Drive Dashboard.
- `internal/ui/screens/dashboard/drive_picker.go`: Connected drive cards grid with mount badges and storage gauges.
- `internal/ui/screens/dashboard/drive_header.go`: Top drive header card with action buttons (`Scan Volume`, `Refresh`, `Disconnect`).
- `internal/ui/screens/dashboard/stats_panel.go`: 3 stat cards for total media, albums, and storage gauge.
- `internal/ui/screens/dashboard/recent_media.go`: Horizontal carousel / grid of recently indexed media items with thumbnails.
- `internal/ui/screens/dashboard/recent_albums.go`: Carousel of recent album cards with cover thumbnails.
- `internal/ui/screens/dashboard/dashboard_test.go`: Unit tests and mock testing for rendering, state transitions, and actions.
