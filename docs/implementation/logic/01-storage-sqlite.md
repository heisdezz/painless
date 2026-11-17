# Logic Feature 01: SQLite Database & Storage Engine

## 1. Overview
The storage engine manages the portable SQLite database (`.media_library.db`) residing directly on external drives. This architecture guarantees that media tags, metadata, album classifications, and indexing travel with the physical drive.

---

## 2. Dependencies & Package Selection
- **Recommended Driver**: `modernc.org/sqlite`
  - *Rationale*: Pure Go implementation with zero CGo dependency (`CGO_ENABLED=0`). Enables effortless cross-compilation across Linux (x86_64, arm64) and Windows without toolchain friction.
  - *Alternative*: `github.com/mattn/go-sqlite3` is prohibited unless strict CGo-native extensions are explicitly mandated.
- **Migration & Querying**: Standard `database/sql` combined with embedded SQL migrations via `embed.FS`.

---

## 3. Database Schema & Migration Spec

Database path: `<driveRoot>/.media_library.db`

### Schema Definition
```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;

-- Albums
CREATE TABLE IF NOT EXISTS albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    relative_path TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Media items
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

-- Tags
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color_hex TEXT DEFAULT '#64748b',
    category TEXT DEFAULT 'general'
);

-- Media Tags junction
CREATE TABLE IF NOT EXISTS media_tags (
    media_id INTEGER NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (media_id, tag_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_media_file_hash ON media_items(file_hash);
CREATE INDEX IF NOT EXISTS idx_media_album_id ON media_items(album_id);
CREATE INDEX IF NOT EXISTS idx_media_current_path ON media_items(current_relative_path);
CREATE INDEX IF NOT EXISTS idx_media_mime ON media_items(mime_type);
```

---

## 4. Architectural Requirements & Concurrency

1. **Connection Pooling**:
   - SQLite supports single-writer, multiple-readers in WAL mode.
   - Max open connections: `db.SetMaxOpenConns(max(4, runtime.NumCPU()))`.
   - Max idle connections: `db.SetMaxIdleConns(2)`.
   - Busy timeout set to 5000ms to prevent lock contention errors during concurrent scanning and UI reads.
2. **Read-Only Fallback**:
   - If an external drive is mounted read-only (e.g. NTFS mount options or write lock switch), gracefully open in read-only mode (`?mode=ro`) and notify the UI layer without crashing.
3. **Database Backup**:
   - Provide an online backup method using SQLite backup API (`VACUUM INTO ?` or step-by-step page copy) to save backups under `<driveRoot>/.media_backups/backup_<timestamp>.db`.
4. **File Structure under `internal/storage/`**:
   - `database.go`: Connection manager, pragmas, ping, close.
   - `migrations.go`: Embedded schema execution and version tracker.
   - `media_repo.go`: `GetMediaItems`, `GetMediaItem`, `InsertMedia`, `UpdateMediaAlbum`, `DeleteMediaItems`.
   - `album_repo.go`: `ListAlbums`, `GetAlbum`, `CreateAlbum`, `UpdateAlbum`, `DeleteAlbum`, `MergeAlbums`.
   - `backup.go`: `CreateBackup`, `ListBackups`, `RestoreBackup`.

---

## 5. Acceptance Criteria
- [ ] Database successfully initializes on a target directory with WAL enabled.
- [ ] Queries for paginated media items (10,000+ rows) return in under 15ms.
- [ ] Transactions correctly rollback on file relocation failures.
- [ ] All unit tests pass with `CGO_ENABLED=0 go test ./internal/storage/...`.
