package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMediaItemParsing(t *testing.T) {
	date := time.Date(2026, 7, 15, 14, 30, 0, 0, time.UTC)
	meta := MediaMetadata{
		Width:       3840,
		Height:      2160,
		CameraMake:  "Sony",
		CameraModel: "A7 IV",
		ISO:         400,
		FocalLength: 35.0,
		Aperture:    1.8,
		DateTaken:   &date,
		VideoCodec:  "h264",
	}

	bytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	item := MediaItem{
		ID:                   1,
		FileHash:             "abc123hash",
		OriginalRelativePath: "DCIM/DSC001.JPG",
		CurrentRelativePath:  "albums/Vacation/DSC001.JPG",
		FileSize:             12500000,
		MimeType:             "image/jpeg",
		MetadataJSON:         string(bytes),
		CreatedAt:            time.Now(),
	}

	if item.Type() != MediaTypeImage {
		t.Errorf("expected MediaTypeImage, got %v", item.Type())
	}

	parsed, err := item.ParseMetadata()
	if err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}
	if parsed.CameraModel != "A7 IV" {
		t.Errorf("expected camera model 'A7 IV', got '%s'", parsed.CameraModel)
	}
	if parsed.Width != 3840 || parsed.Height != 2160 {
		t.Errorf("expected dimensions 3840x2160, got %dx%d", parsed.Width, parsed.Height)
	}
}

func TestMediaTypeDetection(t *testing.T) {
	tests := []struct {
		mime string
		want MediaType
	}{
		{"image/jpeg", MediaTypeImage},
		{"image/png", MediaTypeImage},
		{"video/mp4", MediaTypeVideo},
		{"video/quicktime", MediaTypeVideo},
		{"audio/mpeg", MediaTypeAudio},
		{"application/pdf", MediaTypeOther},
		{"", MediaTypeOther},
	}

	for _, tt := range tests {
		item := MediaItem{MimeType: tt.mime}
		if got := item.Type(); got != tt.want {
			t.Errorf("item.Type() for %q = %v, want %v", tt.mime, got, tt.want)
		}
	}
}

func TestValidateAlbumName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"Vacation 2026", true},
		{"Family & Friends", true},
		{"Japan-Trip_Summer", true},
		{"", false},
		{"   ", false},
		{"Vacation/Trip", false},
		{"Drive:D", false},
		{"Question?Mark", false},
		{"Star*Photo", false},
		{"Quote\"Album", false},
		{"Pipe|Name", false},
		{".", false},
		{"..", false},
		{"unknown", false},
		{"UNKNOWN", false},
		{".media_thumbnails", false},
	}

	for _, tt := range tests {
		if got := ValidateAlbumName(tt.name); got != tt.valid {
			t.Errorf("ValidateAlbumName(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

func TestStagingSummarySerialization(t *testing.T) {
	summary := StagingSummary{
		TotalOperations: 10,
		AlbumMoveCount: map[string]int{
			"Vacation": 7,
			"Family":   2,
		},
		UnknownCount: 1,
		TotalBytes:   104857600,
	}

	bytes, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal summary: %v", err)
	}

	var decoded StagingSummary
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal summary: %v", err)
	}

	if decoded.TotalOperations != 10 || decoded.AlbumMoveCount["Vacation"] != 7 {
		t.Errorf("decoded summary mismatch: %+v", decoded)
	}
}
