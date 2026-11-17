-- Migration V1: Baseline Schema for Media Library

CREATE TABLE IF NOT EXISTS albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    relative_path TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS media_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_hash TEXT NOT NULL,                   -- SHA-256
    original_relative_path TEXT NOT NULL,      -- Relative to library root
    current_relative_path TEXT NOT NULL,       -- Current relative path on drive
    file_size INTEGER NOT NULL,                -- Bytes
    mime_type TEXT NOT NULL,                   -- e.g., image/jpeg, video/mp4
    duration_seconds INTEGER DEFAULT NULL,     -- Audio/video duration
    metadata_json TEXT,                        -- EXIF, camera, dimensions, GPS
    album_id INTEGER REFERENCES albums(id) ON DELETE SET NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color_hex TEXT DEFAULT '#64748b',
    category TEXT DEFAULT 'general'
);

CREATE TABLE IF NOT EXISTS media_tags (
    media_id INTEGER NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (media_id, tag_id)
);

-- Performance Indexes
CREATE INDEX IF NOT EXISTS idx_media_file_hash ON media_items(file_hash);
CREATE INDEX IF NOT EXISTS idx_media_album_id ON media_items(album_id);
CREATE INDEX IF NOT EXISTS idx_media_current_path ON media_items(current_relative_path);
CREATE INDEX IF NOT EXISTS idx_media_mime ON media_items(mime_type);
CREATE INDEX IF NOT EXISTS idx_media_created ON media_items(created_at);
