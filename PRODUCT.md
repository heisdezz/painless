# Product

<!-- impeccable:product-schema 1 -->

## Platform

desktop

(Native Go desktop application — Linux x86_64, Windows x64, macOS arm64/x64. This is a deliberate deviation from impeccable's `web|ios|android|adaptive` enum, confirmed by the user. The UI must feel native-desktop, never like a web page: no boxed cards where chrome doesn't belong, no web-page patterns. UI is built with MyGo (`github.com/egoist/mygo/ui`) and Lobe MyGo GPU components, not HTML/CSS.)

## Users

An individual managing a personal media library (photos and videos) that lives on external/removable drives — USB drives, external HDDs/SSDs, SD cards — plugged into Linux, Windows, or macOS desktops. Single-user, local-first; no accounts or multi-tenancy.

## Product Purpose

Painless is a 100% native Go desktop media organizer for external drives. It scans and indexes media into a portable SQLite library stored on the drive itself (`.media_library.db`), deduplicates via SHA-256, extracts EXIF metadata, generates cached WebP thumbnails (ffmpeg-backed), organizes media into albums, and provides a high-res image viewer and native video player. Success: an indexed, deduplicated, browsable library that travels with the drive and opens instantly on any desktop.

## Positioning

The library database lives on the drive it indexes, so the catalog travels with the media — plug the drive into any machine and the library is already there. Fully native (pure Go, CGo-free, GPU-drawn UI), no web stack, no runtime dependencies beyond ffmpeg.

## Operating Context

- Removable drives are inserted/removed while the app runs; drive detection polls mount changes and the UI must react.
- The portable SQLite DB (`.media_library.db`) and WebP thumbnail cache (`.media_thumbnails/`) live at the drive root alongside the media.
- Video thumbnails/playback shell out to ffmpeg; a CPU throttler (SIGSTOP/SIGCONT, or worker gating on Windows) pauses background encoding during playback.
- A local HTTP streaming server with byte-range support serves playback for the mobile companion; desktop playback opens local files directly, no listener.
- Multi-agent development: `docs/implementation/progress.md` is the coordination source of truth; `commit.md` mandates Conventional Commits and pre-commit checks (`CGO_ENABLED=0 go test ./...`, `go test -race ./internal/storage/... ./internal/state/...`, `go tool mygo vet .`, `gofmt -s -w .`).

## Capabilities and Constraints

- Screens: Dashboard (drive home), Media Catalog (grid), Albums (gallery + CRUD modals), Viewer (image zoom/pan, video player, EXIF rail), Discover (scanner control, live progress, staging review), Settings (backup, mobile pairing QR, logs).
- Scanner: concurrent folder walker, SHA-256 hashing with sparse-hash fast path for large videos, EXIF/duration extraction.
- Staging: atomic file moves, album folder creation, collision handling, cross-device (EXDEV) copy fallback with hash verification.
- Mobile companion: token-authenticated sync/upload REST API; Google Drive cloud backup via service account.
- Technical constraints: pure Go with `CGO_ENABLED=0` (modernc.org/sqlite, no mattn/go-sqlite3); UI components from Lobe MyGo (`pkg/action`, `choice`, `display`, `feedback`, `field`, `layout`, `lucide`, `nav`, `theme`); strict package separation under `internal/` (no monolithic files); tests are string-sensitive on UI text (e.g. "Media Details & EXIF", "n of m" counter format).
- Undecided: none recorded.

## Brand Commitments

- Name: **Painless** (identifier `com.example.painless`).
- Feel: native desktop theater/inspector aesthetic — dark, borderless chrome, precise and restrained; explicitly *not* designed like a website. (User made this binding during the viewer redesign.)

## Evidence on Hand

- `docs/implementation/README.md` — full architecture and module boundaries.
- `docs/implementation/logic/01–08` — per-subsystem implementation specs (storage, drives, staging, scanner, thumbnails, streaming, mobile sync, Google Drive).
- `docs/implementation/ui/01–08` — per-screen UI specs.
- `docs/implementation/progress.md` — task registry and activity log.
- `commit.md` — commit convention, scopes, release automation.
- `.snapshots/` — rendered UI snapshots for visual verification (e.g. `viewer_image.png`, `viewer_video.png`, `viewer_narrow.png`).
- No marketing copy, testimonials, press, or customer evidence exists; future work must not fabricate any.

## Product Principles

1. **The drive is the product boundary.** Everything the app knows about a library lives on that drive; moving or sharing the drive moves the library whole.
2. **Native, never web-flavored.** Desktop conventions, GPU-drawn UI, and platform-appropriate chrome over web-page layouts and patterns.
3. **Safe against data loss.** Atomic moves, hash verification, collision handling, and online backup are non-negotiable behaviors around the user's only copy of their media.
4. **Pure Go, frictionless builds.** CGo-free dependencies and cross-compilable code keep Linux/Windows/macOS binaries trivial to produce.
5. **Responsive to the physical world.** Drive insertion/removal, playback CPU pressure, and background work must visibly and promptly affect the UI.
