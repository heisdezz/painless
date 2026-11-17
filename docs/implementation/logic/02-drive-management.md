# Logic Feature 02: Drive Detection & Removable Storage Management

## 1. Overview
The drive detection service detects, monitors, and catalogues removable drives (USB flash drives, external HDDs/SSDs, SD cards) and user-specified mount folders across Linux, macOS, and Windows.

---

## 2. Dependencies & Package Selection
- **Recommended Library**: `github.com/shirou/gopsutil/v4/disk`
  - *Rationale*: Mature, widely tested cross-platform disk partition and usage information.
- **OS-specific fallback / enhancements**:
  - **Linux**: Read `/proc/mounts`, parse `lsblk --json` or query `sysfs`/`udev` to distinguish external USB blocks (`ID_BUS=usb`) from internal NVMe/SATA partitions.
  - **Windows**: Use standard `golang.org/x/sys/windows` (`GetLogicalDrives`, `GetDriveTypeW`) to classify `DRIVE_REMOVABLE` vs `DRIVE_FIXED`.
  - **Custom Folders**: In-memory user-mounted drives (for development or local testing without physical USB hardware).

---

## 3. Data Model

```go
package domain

type DriveType string

const (
    DriveTypeInternal DriveType = "internal"
    DriveTypeExternal DriveType = "external"
    DriveTypeNetwork  DriveType = "network"
    DriveTypeCustom   DriveType = "custom"
)

type DriveStatus string

const (
    DriveStatusMounted   DriveStatus = "mounted"
    DriveStatusUnmounted DriveStatus = "unmounted"
    DriveStatusSyncing   DriveStatus = "syncing"
)

type DriveInfo struct {
    ID             string      `json:"id"`             // Unique identifier (UUID or mount path)
    Name           string      `json:"name"`           // User-friendly label (e.g. "Samsung T7", "Drive D:")
    Type           DriveType   `json:"type"`           // internal, external, custom
    Path           string      `json:"path"`           // Root mount path (e.g. /media/user/Drive, D:\)
    FileSystem     string      `json:"file_system"`    // ext4, ntfs, exfat, vfat
    TotalBytes     uint64      `json:"total_bytes"`    // Total capacity in bytes
    FreeBytes      uint64      `json:"free_bytes"`     // Available space
    UsedPercentage float64     `json:"used_percentage"`// 0.0 - 100.0%
    Status         DriveStatus `json:"status"`         // mounted, unmounted
    HasLibrary     bool        `json:"has_library"`    // True if .media_library.db exists
}
```

---

## 4. Implementation Details

1. **Detection Pipeline (`internal/drive/detector.go`)**:
   - `ListDrives(ctx context.Context) ([]domain.DriveInfo, error)`
   - Filter out pseudo-filesystems on Linux: `tmpfs`, `devtmpfs`, `squashfs`, `overlay`, `sysfs`, `proc`.
   - Filter filesystem types for user storage: `ext4`, `btrfs`, `exfat`, `vfat`, `ntfs`, `apfs`, `hfs+`.
2. **Drive Monitor & Polling (`internal/drive/monitor.go`)**:
   - Background ticker (default: 3 seconds) checking mount table changes.
   - On change, diff against previous drive map and emit domain events: `DriveMountedEvent` / `DriveUnmountedEvent`.
   - If an active library drive is abruptly unplugged, signal safety abort to ongoing scans and active write transactions.
3. **Custom Folder Mounting (`internal/drive/custom.go`)**:
   - Allow users to mount arbitrary local directories as virtual drives for development and non-USB usage.
   - Validate directory existence and read/write permissions.

---

## 5. File Structure
- `internal/drive/drive.go`: Service interface, registry of user custom mounts.
- `internal/drive/detect_linux.go`: Linux `/proc/mounts` and block device introspection.
- `internal/drive/detect_windows.go`: Windows API `GetLogicalDrives` and volume flags.
- `internal/drive/detect_other.go`: Fallback using `gopsutil`.
- `internal/drive/monitor.go`: Ticker-based change detector.

---

## 6. Acceptance Criteria
- [ ] Correctly enumerates external USB flash drives on both Linux and Windows.
- [ ] Emits events when a drive is inserted or removed.
- [ ] Flags `HasLibrary: true` if `.media_library.db` exists at drive root.
- [ ] User custom folders are correctly merged and validated.
