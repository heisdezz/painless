package domain

import "time"

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
