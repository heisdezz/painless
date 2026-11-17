# Project Progress & Multi-Agent Coordination

Multi-agent coordination source of truth for **Painless** (External Drive Media Organizer ported to native MyGo).

---

## 1. Multi-Agent Coordination Rules (Collision Avoidance)

1. **Claim before work**
   - Pick an unclaimed task whose dependencies are satisfied.
   - Set `Status` to `claimed` or `in_progress`, set `Owner` to your agent/session identifier, set `Updated` (UTC ISO-8601), and append a dated note in the task block and a line to the **Activity Log**.
   - **Never start work on a task owned by another active agent.**

2. **One owner per task**
   - Only the assigned `Owner` may edit that task's fields, status, or notes (except coordinators breaking deadlocks or reclaiming stale tasks).
   - If work spans another task's scope, coordinate via log/handoff rather than modifying their task block.

3. **Strict scope boundaries**
   - Each task defines `Allowed paths/scope`. Modify files **only** within that directory scope.
   - If scope must expand, document it in `Notes` before editing, or preferably split into a separate follow-up task.

4. **Minimize edit conflicts**
   - In `progress.md`, edit **only** your claimed task block and append a new line to **Activity Log**.
   - Do not reformat, reorder, or alter unrelated sections.

5. **Status lifecycle**
   - `todo`: Unclaimed, ready for pickup once dependencies are met.
   - `claimed`: Reserved by an agent; preparation underway.
   - `in_progress`: Actively modifying code in scope.
   - `blocked`: Cannot proceed; note must specify blocker and dependency.
   - `review`: Implementation and tests in scope complete; ready for verification.
   - `done`: Verified per acceptance criteria (`go test`, `go tool mygo vet .`).
   - `cancelled`: Deprecated or superseded with documented reason.

6. **Stale claims & handoffs**
   - A claim without progress for >24h is considered stale and may be reclaimed by logging the previous owner and reason.
   - When handing off, document current status and exact next steps in `Notes`, change status to `todo` or `review`, and record in the Activity Log.

7. **Task IDs**
   - Use stable IDs (`LOGIC-...`, `UI-...`, `SYS-...`). Never reuse an ID.

---

## 2. Implementation Conventions & Coding Standards

### A. Use Established Go Packages (Don't Reinvent the Wheel)
Avoid hand-rolling complex algorithms, codecs, or drivers. Use vetted, standard Go libraries:
- **SQLite Database**: `modernc.org/sqlite` (Pure Go, 100% CGo-free for seamless cross-compilation).
- **Disk & Mount Introspection**: `github.com/shirou/gopsutil/v4/disk` alongside OS-specific `/proc/mounts` and Windows API.
- **MIME Sniffing**: `github.com/h2non/filetype`.
- **EXIF Extraction**: `github.com/rwcarlsen/goexif/exif`.
- **Image Resizing**: `golang.org/x/image/draw` or `github.com/disintegration/imaging`.
- **Streaming & HTTP**: Standard `net/http` (supports Range headers and HTTP 206 Partial Content natively via `http.ServeContent`).
- **Google Drive**: `google.golang.org/api/drive/v3`.
- **In-Memory Cache**: `github.com/hashicorp/golang-lru/v2`.

### B. Separation of Concerns (No Monoliths / "God Files")
- **Modular Directory Layout**: Every package under `internal/` must have a singular, well-defined responsibility.
- **File Size Discipline**: Keep files focused (typically under 200–400 LOC).
- **Prohibited Patterns**:
  - Do NOT cram database queries, business rules, and UI rendering into a single file.
  - Do NOT create global mutable variables across packages; use explicit dependency injection and constructors (`New...`).
  - Separate data entities (`internal/domain`), persistence (`internal/storage`), services (`internal/...`), and UI views (`internal/ui/screens/...`).

### C. MyGo Native UI Conventions
Follow the pinned rules in `mygo-maintenance`:
- **Main Thread Context**: `*ui.Context` is valid only on the main UI thread during build passes. Keep `*ui.Context` out of background goroutines and application state.
- **Async Updates**: Background workers must publish UI updates via `window.Update(func())`.
- **Element Lifetime**: `ui.Element` values expire at the end of each frame. Never store an element or its subparts. Store only data models and `ui.Handle`.
- **Stable Identity**: Bind persistent state using `.Bind(&handle)`. Always provide stable keys for list and grid items with `c.Key(id)`.
- **Drawing Callbacks**: Limit drawing code to painting; keep expensive disk/network I/O off the UI thread.

### D. Testing & Quality Verification
- Pure Go unit tests: `CGO_ENABLED=0 go test ./...`.
- MyGo static analysis: `go tool mygo vet .`.
- Data race checking: `go test -race ./internal/storage/... ./internal/state/...`.

---

## 3. Task Registry

### Phase 1: Storage & Core Domain

#### LOGIC-DOMAIN-MODELS: Core Domain Models
- Status: todo
- Owner: 
- Updated: 
- Depends on: none
- Allowed paths/scope: `internal/domain/**`
- Acceptance: MediaItem, Album, Tag, DriveInfo, LogEntry, and StagedOperation structs defined with JSON/SQL tags.

#### LOGIC-DB-SQLITE: SQLite Database Engine & Migrations
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DOMAIN-MODELS
- Allowed paths/scope: `internal/storage/**`
- Acceptance: Connection pool initialized with WAL mode, busy timeout 5000ms, schema migrations v1, read-only fallback, and online backup support (`modernc.org/sqlite`).

#### LOGIC-MEDIA-REPO: Media & Album Repositories
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DB-SQLITE
- Allowed paths/scope: `internal/storage/media_repo.go`, `internal/storage/album_repo.go`
- Acceptance: CRUD for albums and paginated/filtered queries for media items with sub-15ms performance on 10k rows.

---

### Phase 2: Drive Detection & Filesystem Operations

#### LOGIC-DRIVE-DETECT: Removable Drive Detection & Monitoring
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DOMAIN-MODELS
- Allowed paths/scope: `internal/drive/**`
- Acceptance: Detects USB drives on Linux & Windows, polls mount changes every 3s, supports custom folder mounts, and emits drive events.

#### LOGIC-FILE-STAGING: Staging Engine & Physical File Ops
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DB-SQLITE
- Allowed paths/scope: `internal/fileops/**`
- Acceptance: Virtual staging model, atomic cross-device file moves, name collision resolution (`_1.jpg`), album directory scaffolding, and rollback safety.

---

### Phase 3: Scanner & Thumbnail Pipeline

#### LOGIC-SCANNER: Concurrent Media Scanner & Duplicate Hashing
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DB-SQLITE, LOGIC-FILE-STAGING
- Allowed paths/scope: `internal/scanner/**`
- Acceptance: Multi-core directory walker, SHA-256 duplicate hashing, MIME detection (`filetype`), EXIF extraction (`goexif`), and batched SQLite inserts.

#### LOGIC-THUMBNAIL: Thumbnail Pipeline & CPU Throttler
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DOMAIN-MODELS
- Allowed paths/scope: `internal/thumbnail/**`
- Acceptance: Ffmpeg/imaging thumbnail generation, on-disk WebP caching (`.media_thumbnails/`), memory LRU, and SIGSTOP/SIGCONT CPU pause gate.

---

### Phase 4: Network Services & Cloud Backup

#### LOGIC-MEDIA-SERVER: Local Media Streaming HTTP Server
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DB-SQLITE
- Allowed paths/scope: `internal/server/stream/**`
- Acceptance: HTTP 206 Range requests via `http.ServeContent`, path traversal defense, and thumbnail endpoint on port 51789.

#### LOGIC-MOBILE-SYNC: Mobile Companion REST & Upload API
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DB-SQLITE, LOGIC-THUMBNAIL
- Allowed paths/scope: `internal/server/mobile/**`
- Acceptance: Authenticated REST endpoints (constant-time token check), direct-to-disk streaming multipart upload, and LAN IP pairing.

#### LOGIC-GDRIVE-BACKUP: Google Drive Backup Integration
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DB-SQLITE
- Allowed paths/scope: `internal/gdrive/**`
- Acceptance: Service account JSON auth, connection validation, and online SQLite snapshot upload using `google.golang.org/api/drive/v3`.

---

### Phase 5: App State & Native UI Shell

#### UI-STATE-BRIDGE: Central State & Window.Update Bridge
- Status: todo
- Owner: 
- Updated: 
- Depends on: LOGIC-DOMAIN-MODELS
- Allowed paths/scope: `internal/state/**`, `internal/ui/bridge.go`
- Acceptance: Thread-safe central app state, active drive tracking, selection store, and background worker event publication.

#### UI-SHELL-NAV: App Shell, Theme & Sidebar Navigation
- Status: todo
- Owner: 
- Updated: 
- Depends on: UI-STATE-BRIDGE
- Allowed paths/scope: `internal/ui/shell/**`, `internal/ui/theme/**`
- Acceptance: `mygo.NewWindow` setup, dark theme styling, persistent `ui.Sidebar`, drive switcher with capacity bar, and toast notification host.

---

### Phase 6: Native UI Screens

#### UI-CATALOG-GRID: Media Catalog GridView & Selection
- Status: todo
- Owner: 
- Updated: 
- Depends on: UI-SHELL-NAV, LOGIC-THUMBNAIL, LOGIC-MEDIA-REPO
- Allowed paths/scope: `internal/ui/screens/catalog/**`
- Acceptance: Virtualized `ui.GridView`, multi-selection (Ctrl/Shift/Cmd+A), debounced search, MIME filter bar, and floating batch actions toolbar.

#### UI-ALBUM-GALLERY: Album Gallery & Action Modals
- Status: todo
- Owner: 
- Updated: 
- Depends on: UI-SHELL-NAV, LOGIC-MEDIA-REPO, LOGIC-FILE-STAGING
- Allowed paths/scope: `internal/ui/screens/albums/**`
- Acceptance: Album card grid with cover previews, album detail view, and modals (Create, Edit, Delete, Merge).

#### UI-MEDIA-VIEWER: Photo Zoom Viewer & Native Video Player
- Status: todo
- Owner: 
- Updated: 
- Depends on: UI-SHELL-NAV, LOGIC-THUMBNAIL
- Allowed paths/scope: `internal/ui/screens/viewer/**`
- Acceptance: Full-res photo viewer with pan/zoom, EXIF metadata side drawer, and native video player using ffmpeg frame pipeline with `ui.VideoPlayer`.

#### UI-DISCOVER-STAGER: Scanner View & Staging Consolidator
- Status: todo
- Owner: 
- Updated: 
- Depends on: UI-SHELL-NAV, LOGIC-SCANNER, LOGIC-FILE-STAGING
- Allowed paths/scope: `internal/ui/screens/discover/**`
- Acceptance: Live scan progress bar, staging review table, album destination assignment, and "Consolidate & Execute" confirmation modal.

#### UI-SETTINGS-LOGS: Settings, Backup Tools & Diagnostic Logs
- Status: todo
- Owner: 
- Updated: 
- Depends on: UI-SHELL-NAV, LOGIC-MOBILE-SYNC, LOGIC-GDRIVE-BACKUP
- Allowed paths/scope: `internal/ui/screens/settings/**`, `internal/logging/**`
- Acceptance: Database backup/restore list, mobile sync toggle & token display, Google Drive credentials configuration, and searchable system logs table.

---

## 4. Activity Log

- 2026-11-17T18:00:00Z [system] Progress tracker established. Multi-agent collision avoidance rules, implementation standards, and task registry initialized.
