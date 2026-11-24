package drive

import (
	"context"
	"path/filepath"
	"strings"

	"painless/internal/domain"

	"github.com/shirou/gopsutil/v4/disk"
)

// Ignored filesystem types that do not represent physical user storage volumes.
var ignoredFSTypes = map[string]struct{}{
	"tmpfs":       {},
	"devtmpfs":    {},
	"squashfs":    {},
	"overlay":     {},
	"sysfs":       {},
	"proc":        {},
	"cgroup":      {},
	"cgroup2":     {},
	"rpc_pipefs":  {},
	"securityfs":  {},
	"debugfs":     {},
	"tracefs":     {},
	"pstore":      {},
	"autofs":      {},
	"mqueue":      {},
	"hugetlbfs":   {},
	"fusectl":     {},
	"configfs":    {},
	"binfmt_misc": {},
	"efivarfs":    {},
	"devpts":      {},
}

// detectSystemDrives enumerates disk partitions and returns categorized DriveInfo.
func detectSystemDrives(ctx context.Context) ([]domain.DriveInfo, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	var drives []domain.DriveInfo
	seen := make(map[string]struct{})

	for _, part := range partitions {
		fstype := strings.ToLower(part.Fstype)
		if _, ignored := ignoredFSTypes[fstype]; ignored {
			continue
		}

		cleanMount := filepath.Clean(part.Mountpoint)
		if cleanMount == "" || domain.IsSystemBootPath(cleanMount) {
			continue
		}
		if _, already := seen[cleanMount]; already {
			continue
		}
		seen[cleanMount] = struct{}{}

		// Determine if drive is external/removable
		driveType := classifyDriveType(part.Device, cleanMount, fstype)

		// Fetch storage capacity
		var total, free uint64
		var usedPct float64
		usage, err := disk.UsageWithContext(ctx, cleanMount)
		if err == nil && usage != nil {
			total = usage.Total
			free = usage.Free
			usedPct = usage.UsedPercent
		}

		name := formatDriveName(part.Device, cleanMount, driveType)
		hasLib := checkHasLibrary(cleanMount)

		drives = append(drives, domain.DriveInfo{
			ID:             cleanMount,
			Name:           name,
			Type:           driveType,
			Path:           cleanMount,
			FileSystem:     part.Fstype,
			TotalBytes:     total,
			FreeBytes:      free,
			UsedPercentage: usedPct,
			Status:         domain.DriveStatusMounted,
			HasLibrary:     hasLib,
		})
	}

	return drives, nil
}

// classifyDriveType determines whether a mount point is internal or external/removable.
func classifyDriveType(device, mountPoint, fstype string) domain.DriveType {
	lowerMount := strings.ToLower(mountPoint)

	// Typical external mount locations on Unix/Linux/macOS
	if strings.HasPrefix(lowerMount, "/run/media/") ||
		strings.HasPrefix(lowerMount, "/media/") ||
		strings.HasPrefix(lowerMount, "/volumes/") {
		return domain.DriveTypeExternal
	}

	// Network filesystem types
	if fstype == "nfs" || fstype == "cifs" || fstype == "smb" || fstype == "sshfs" {
		return domain.DriveTypeNetwork
	}

	return domain.DriveTypeInternal
}

func formatDriveName(device, mountPoint string, dType domain.DriveType) string {
	base := filepath.Base(mountPoint)
	if base == "/" || base == "." || base == "" {
		return "System Drive (" + mountPoint + ")"
	}
	if dType == domain.DriveTypeExternal {
		return "Removable: " + base
	}
	return base
}
