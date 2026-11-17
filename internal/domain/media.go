package domain

import (
	"encoding/json"
	"time"
)

// MediaType represents the high-level category of a media file.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeOther MediaType = "other"
)

// MediaFilter specifies the filter criteria for listing media items.
type MediaFilter string

const (
	MediaFilterAll    MediaFilter = "all"
	MediaFilterImages MediaFilter = "images"
	MediaFilterVideos MediaFilter = "videos"
	MediaFilterAudio  MediaFilter = "audio"
)

// MediaSortBy defines the sorting field for media listings.
type MediaSortBy string

const (
	SortByDate MediaSortBy = "date"
	SortByName MediaSortBy = "name"
	SortBySize MediaSortBy = "size"
)

// SortOrder defines ascending or descending order.
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// MediaMetadata represents structured EXIF, camera, and codec details.
type MediaMetadata struct {
	Width           int        `json:"width,omitempty"`
	Height          int        `json:"height,omitempty"`
	CameraMake      string     `json:"camera_make,omitempty"`
	CameraModel     string     `json:"camera_model,omitempty"`
	LensModel       string     `json:"lens_model,omitempty"`
	FocalLength     float64    `json:"focal_length,omitempty"`
	Aperture        float64    `json:"aperture,omitempty"`
	ExposureTime    string     `json:"exposure_time,omitempty"`
	ISO             int        `json:"iso,omitempty"`
	DateTaken       *time.Time `json:"date_taken,omitempty"`
	GPSLatitude     *float64   `json:"gps_latitude,omitempty"`
	GPSLongitude    *float64   `json:"gps_longitude,omitempty"`
	GPSAltitude     *float64   `json:"gps_altitude,omitempty"`
	DurationSeconds *float64   `json:"duration_seconds,omitempty"`
	VideoCodec      string     `json:"video_codec,omitempty"`
	AudioCodec      string     `json:"audio_codec,omitempty"`
	Bitrate         int64      `json:"bitrate,omitempty"`
}

// MediaItem represents an individual media file cataloged on an external drive.
type MediaItem struct {
	ID                   int64     `json:"id" db:"id"`
	FileHash             string    `json:"file_hash" db:"file_hash"`
	OriginalRelativePath string    `json:"original_relative_path" db:"original_relative_path"`
	CurrentRelativePath  string    `json:"current_relative_path" db:"current_relative_path"`
	FileSize             int64     `json:"file_size" db:"file_size"`
	MimeType             string    `json:"mime_type" db:"mime_type"`
	DurationSeconds      *int      `json:"duration_seconds,omitempty" db:"duration_seconds"`
	MetadataJSON         string    `json:"metadata_json,omitempty" db:"metadata_json"`
	AlbumID              *int64    `json:"album_id,omitempty" db:"album_id"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`

	// ParsedMetadata holds cached unmarshaled MetadataJSON
	ParsedMetadata *MediaMetadata `json:"metadata,omitempty" db:"-"`
}

// ParseMetadata deserializes MetadataJSON into ParsedMetadata.
func (m *MediaItem) ParseMetadata() (*MediaMetadata, error) {
	if m.MetadataJSON == "" {
		return nil, nil
	}
	var meta MediaMetadata
	if err := json.Unmarshal([]byte(m.MetadataJSON), &meta); err != nil {
		return nil, err
	}
	m.ParsedMetadata = &meta
	return &meta, nil
}

// Type returns the high-level category of this media item based on its MIME type.
func (m *MediaItem) Type() MediaType {
	switch {
	case len(m.MimeType) >= 5 && m.MimeType[:5] == "image":
		return MediaTypeImage
	case len(m.MimeType) >= 5 && m.MimeType[:5] == "video":
		return MediaTypeVideo
	case len(m.MimeType) >= 5 && m.MimeType[:5] == "audio":
		return MediaTypeAudio
	default:
		return MediaTypeOther
	}
}

// MediaListParams defines query parameters for fetching media items.
type MediaListParams struct {
	DrivePath string      `json:"drive_path"`
	AlbumID   *int64      `json:"album_id,omitempty"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
	Search    string      `json:"search,omitempty"`
	Filter    MediaFilter `json:"filter,omitempty"`
	SortBy    MediaSortBy `json:"sort_by,omitempty"`
	SortOrder SortOrder   `json:"sort_order,omitempty"`
}

// MediaListPage represents a paginated media query result.
type MediaListPage struct {
	Items      []MediaItem `json:"items"`
	Total      int         `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	NextItemID *int64      `json:"next_item_id,omitempty"`
	PrevItemID *int64      `json:"prev_item_id,omitempty"`
}
