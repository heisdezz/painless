package domain

// Tag represents a descriptive label for categorization.
type Tag struct {
	ID       int64  `json:"id" db:"id"`
	Name     string `json:"name" db:"name"`
	ColorHex string `json:"color_hex" db:"color_hex"`
	Category string `json:"category" db:"category"`
}

// MediaTag represents a many-to-many relationship between a MediaItem and a Tag.
type MediaTag struct {
	MediaID int64 `json:"media_id" db:"media_id"`
	TagID   int64 `json:"tag_id" db:"tag_id"`
}
