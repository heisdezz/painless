package domain

import "time"

// StageAction defines the physical filesystem operation intended for a media item.
type StageAction string

const (
	StageActionMove   StageAction = "move"
	StageActionDelete StageAction = "delete"
	StageActionIgnore StageAction = "ignore"
)

// StageStatus tracks execution lifecycle of a staged operation.
type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusExecuting StageStatus = "executing"
	StageStatusDone      StageStatus = "done"
	StageStatusFailed    StageStatus = "failed"
)

// StagedOperation records a planned or completed filesystem operation.
type StagedOperation struct {
	ID              string      `json:"id"`
	MediaID         int64       `json:"media_id"`
	FileHash        string      `json:"file_hash"`
	SourcePath      string      `json:"source_path"`
	DestinationPath string      `json:"destination_path"`
	TargetAlbumID   *int64      `json:"target_album_id,omitempty"`
	TargetAlbumName string      `json:"target_album_name,omitempty"`
	Action          StageAction `json:"action"`
	Status          StageStatus `json:"status"`
	ErrorMessage    string      `json:"error_message,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	ExecutedAt      *time.Time  `json:"executed_at,omitempty"`
}

// StagingSummary summarizes the planned batch before disk commit.
type StagingSummary struct {
	TotalOperations int            `json:"total_operations"`
	AlbumMoveCount  map[string]int `json:"album_move_count"`
	UnknownCount    int            `json:"unknown_count"`
	DeleteCount     int            `json:"delete_count"`
	IgnoreCount     int            `json:"ignore_count"`
	TotalBytes      int64          `json:"total_bytes"`
}
