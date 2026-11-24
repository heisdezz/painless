package domain

import (
	"path/filepath"
	"strings"
	"time"
)

// DriveType categorizes the storage medium.
type DriveType string

const (
	DriveTypeInternal DriveType = "internal"
	DriveTypeExternal DriveType = "external"
	DriveTypeNetwork  DriveType = "network"
	DriveTypeCustom   DriveType = "custom"
)

// DriveStatus indicates whether the volume is ready for I/O operations.
type DriveStatus string

const (
	DriveStatusMounted    DriveStatus = "mounted"
	DriveStatusUnmounted  DriveStatus = "unmounted"
	DriveStatusSyncing    DriveStatus = "syncing"
	DriveStatusUnmounting DriveStatus = "unmounting"
)

// DriveInfo represents volume and filesystem properties of a storage drive.
type DriveInfo struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Type           DriveType   `json:"type"`
	Path           string      `json:"path"`
	FileSystem     string      `json:"file_system"`
	TotalBytes     uint64      `json:"total_bytes"`
	FreeBytes      uint64      `json:"free_bytes"`
	UsedPercentage float64     `json:"used_percentage"`
	Status         DriveStatus `json:"status"`
	HasLibrary     bool        `json:"has_library"`
}

// DriveEventType describes hardware insertion/removal events.
type DriveEventType string

const (
	DriveEventMounted   DriveEventType = "mounted"
	DriveEventUnmounted DriveEventType = "unmounted"
	DriveEventUpdated   DriveEventType = "updated"
)

// DriveEvent represents a notification emitted when drive status changes.
type DriveEvent struct {
	Type      DriveEventType `json:"type"`
	Drive     DriveInfo      `json:"drive"`
	Timestamp time.Time      `json:"timestamp"`
}

var systemBootMountPrefixes = []string{
	"/boot",
	"/efi",
	"/esp",
	"/sys",
	"/proc",
	"/dev",
	"/run/initramfs",
	"/snap",
	"/volumes/efi",
	"/volumes/preboot",
	"/volumes/recovery",
	"/system/volumes/preboot",
	"/system/volumes/recovery",
	"/system/volumes/vm",
	"/system/volumes/update",
	"/system/volumes/iscpreboot",
	"/system/volumes/xarts",
	"/system/volumes/hardware",
}

// IsSystemBootPath reports whether mountPath points to an OS runtime or
// system boot partition (such as /boot, /boot/efi, or /efi) that must not
// be selected as a media library drive.
func IsSystemBootPath(mountPath string) bool {
	if strings.TrimSpace(mountPath) == "" {
		return false
	}
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(mountPath)))
	for _, prefix := range systemBootMountPrefixes {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+"/") {
			return true
		}
	}
	return false
}

// IsSystemBoot reports whether the drive's mount path is a system boot point.
func (d DriveInfo) IsSystemBoot() bool {
	return IsSystemBootPath(d.Path)
}
