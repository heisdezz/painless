# Commit Convention

Painless uses [Conventional Commits](https://www.conventionalcommits.org/).
CI and release automation read these messages to compute the next semantic version automatically, so every commit that lands on `main` must strictly adhere to the specification below.

---

## 1. Format

```
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

- **type** — required, one of the standardized types in the table below.
- **scope** — optional noun specifying the subsystem touched (e.g. `storage`, `drive`, `staging`, `scanner`, `thumbnail`, `stream`, `mobile`, `gdrive`, `ui`, `catalog`, `albums`, `viewer`, `settings`, `logs`, `ci`).
- **subject** — imperative present tense ("add", not "added" or "adds"), lower case, no trailing period, at most 72 characters.
- **body** — optional; explain *why* the change was made, technical trade-offs, and context (not simply a diff re-hash).
- **footer** — optional; issue references (`Refs: #12`) or `BREAKING CHANGE:`.

---

## 2. Types and Version Impact

| Type | Description | SemVer Release Impact |
| :--- | :--- | :--- |
| `feat` | A new user-facing capability or service feature | **minor** (`0.1.0 → 0.2.0`) |
| `fix` | A bug fix or error recovery correction | **patch** (`0.1.0 → 0.1.1`) |
| `perf` | A performance improvement (e.g. faster indexing, caching) | **patch** |
| `revert` | Reverting a previous commit | **patch** |
| `docs` | Documentation changes only | none |
| `chore` | Maintenance, dependencies, toolchain, housekeeping | none |
| `style` | Formatting, whitespace, linting with no logic change | none |
| `refactor` | Code restructuring without fixing a bug or adding a feature | none |
| `test` | Adding or correcting unit, integration, or UI tests | none |
| `build` | Build system, packaging, or Go module changes | none |
| `ci` | CI workflow configuration changes | none |

> [!IMPORTANT]
> Any commit marked as breaking triggers a **major** bump (`0.1.0 → 1.0.0`), regardless of the commit type.

---

## 3. Breaking Changes

Mark breaking changes using an exclamation mark `!` before the colon:

```
feat(storage)!: migrate schema to support multi-drive catalog joins
```

or with an explicit `BREAKING CHANGE:` footer:

```
refactor(drive): restructure DriveInfo mount metadata

BREAKING CHANGE: changes the exported signature of ListDrives and removes legacy drive ID format.
```

---

## 4. How Versioning Is Computed

1. The baseline is the highest existing `vX.Y.Z` git tag. If no tags exist, the version specified in `mygo.json` is used as the baseline.
2. The release workflow evaluates every commit on `main` since the baseline tag and applies the **highest bump** encountered (`major > minor > patch`).
3. If all commits since the last tag are non-versioned types (`docs`, `chore`, `style`, `refactor`, `test`), no release is triggered.
4. When a release is triggered, `mygo.json` is stamped, and a GitHub release with native cross-platform binaries (Linux x86_64, Windows x64, macOS arm64/x64) is created.

---

## 5. Standard Scopes for Painless

To keep commit logs organized and filterable, use one of these standard scopes:

| Scope | Area Covered |
| :--- | :--- |
| `storage` | SQLite database, schema migrations, WAL pragmas, query repos |
| `drive` | Removable drive detection, volume monitoring, mount points |
| `staging` | Staging engine, atomic file moves, album folder creation, collision handling |
| `scanner` | Directory walker, SHA-256 duplicate hashing, EXIF metadata parser |
| `thumbnail` | Ffmpeg subprocess, WebP caching, SIGSTOP/SIGCONT CPU throttler |
| `stream` | Local HTTP media streaming server, Range 206 partial content |
| `mobile` | Mobile companion sync server, token authentication, multipart uploads |
| `gdrive` | Google Drive cloud backup, service account integration |
| `ui` | General native MyGo UI elements, theme tokens, layout primitives |
| `shell` | Window setup, sidebar navigation, drive selector, toast notifications |
| `catalog` | Media library grid view, virtualized scrolling, multi-selection |
| `albums` | Album cards, album detail view, CRUD modals (create/edit/delete/merge) |
| `viewer` | High-res image zoom/pan, native video player, EXIF metadata drawer |
| `discover` | Scanner control UI, live progress indicators, staging review table |
| `settings` | Settings tabs, backup management, pairing QR code, diagnostic logs |
| `state` | Central application state, selection store, background event bridge |
| `deps` | Go module updates (`go.mod`, `go.sum`) |
| `ci` | GitHub Actions and release automation |

---

## 6. Pre-Commit Quality Checks

Before committing changes, ensure your working tree passes the following checks:

```bash
# 1. Verify compilation and tests with pure Go (no CGo required)
CGO_ENABLED=0 go test ./...

# 2. Check for race conditions in shared state / background workers
go test -race ./internal/storage/... ./internal/state/...

# 3. Run MyGo static verification for UI context and element lifetime
go tool mygo vet .

# 4. Ensure Go formatting
gofmt -s -w .
```

---

## 7. Examples

### Good Feature Commit
```
feat(thumbnail): throttle ffmpeg workers on video playback

Software video decoding in ffmpeg pins CPU cores, leading to audio
dropouts and frame stutter when a video is being played in the UI.
This hooks thumbnail worker processes to the video player's play/pause
signals, sending SIGSTOP to pause background generation immediately and
SIGCONT once playback stops.

Refs: #42
```

### Good Fix Commit
```
fix(staging): handle cross-device move fallback without data loss

os.Rename fails with EXDEV when moving files across different volume
mount points. Added an automatic fallback to buffered copy with SHA-256
verification before unlinking the source file.
```

### Good Performance Commit
```
perf(scanner): implement fast sparse hash for large video duplicates

Full SHA-256 computation on 4GB video files saturated disk I/O.
Implemented a two-tier hashing heuristic: a sparse 192KB header/middle/tail
hash screens candidates first, reserving full SHA-256 calculation only
for verified candidates.
```

---

## 8. Commit Message Template

```
<type>(<scope>): <subject>

<why this change is needed and context on technical decisions>

<optional issue references or BREAKING CHANGE: ...>
```
