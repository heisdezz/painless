# UI Feature 03: Album Gallery, Detail View & Action Modals

## 1. Overview

The Album Management interface organizes media into physical collections on the drive. It utilizes Lobe UI components for modal dialogs (`overlay.Modal`), inputs (`field.Input`, `field.TextArea`), badges (`display.Tag`), empty states (`display.Empty`), and context menus (`nav.ContextMenu`).

---

## 2. Views & Screens

### 1. Album Gallery (`internal/ui/screens/albums/gallery_view.go`)

- Flat borderless filter bar (`galleryFilterBar`) above the grid (no glass blur overlay):
  - `field.SearchBar`: "Search albums..." matching album names and descriptions.
  - `action.Segmented`: `All`, `With media`, `Empty`.
  - Count tag (`display.Tag`): `%d albums` (or `Loading…` during the initial fetch).
  - `field.Select`: Sort order (`Name A-Z`, `Most items`, `Largest size`, `Newest first`).
  - `action.Button` (`Primary`): "New Album".
- Responsive grid of album cards (`layout.Grid`) scrolling cleanly beneath the bar.
- Each album card displays:
  - Album cover preview clipped to the card's rounded frame.
  - Album title and item count tag (`display.Tag`).
  - Description or compact album size / occupancy subtitle.
  - Open button and quiet `action.ActionIcon` triggers (Edit, Merge, Delete).
- Empty states (`display.Empty`): "No albums yet" with a "Create Your First Album" button when no albums exist, or "No matching albums" when active filters match nothing.

### 2. Album Detail View (`internal/ui/screens/albums/detail_view.go`)

- Compact borderless header bar (`Shrink(0)`) with:
  - `action.Button`: "Back to Albums"
  - Album title, item count tag (`display.Tag`), and total size tag (`display.Tag`)
  - Quiet `action.ActionIcon` triggers on the right: Edit (`lucide.Pencil`), Merge (`lucide.GitMerge`), and Delete (`lucide.Trash2`, danger)
  - Optional single-line album description below the header row (omitted when empty), followed by a 1px hairline divider (`BorderSecondary`)
- Filter bar (`detailFilterBar`): `field.SearchBar` ("Search album media"), `action.Segmented` (`All`, `Photos`, `Videos`), and `field.Select` sort order.
- Media grid (`mediagrid.Strip`) displaying members of the selected album with pagination (`nav.Pagination`).

---

## 3. Modal Dialogs (`overlay.Modal`)

All dialogs use `github.com/lobehub/lobe-mygo/pkg/overlay.Modal`, featuring a frosted backdrop, title header, close button, and OK/Cancel actions:

1. **Create Album Modal**:
   - `field.Input`: Album Name (with error validation against illegal filesystem characters `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`).
   - `field.TextArea`: Description.
   - Primary `OK` action: "Create Album".
2. **Edit Album Modal**:
   - `field.Input`: Rename album.
   - Automatically renames physical folder on disk and updates database paths.
3. **Delete Album Modal**:
   - `feedback.Alert`: Danger warning explaining the consequences of deletion.
   - `choice.Radio`:
     - _Soft Delete_: Keep files, move to unassigned `unknown/`.
     - _Permanent Delete_: Physically delete files from the drive.
   - Requires clicking danger `action.Button("Delete Album")`.
4. **Merge Albums Modal**:
   - `field.Select`: Target destination album.
   - Warning alert detailing that all member files will move into the target folder.

---

## 4. Asynchronous Loads, Skeletons & Reveal

The library database lives on the (possibly cold) USB drive, so neither the album summaries query nor the member-page query may run on the UI thread. The album screen mirrors the catalog's treatment (see UI Feature 02, section 4).

1. **Off-thread fetches (`albums.go`)**:
   - `Reload()` and `loadDetailItems()` set a loading flag, capture their repo and the one-slot result channel on the UI thread, and run the query in a worker goroutine; the worker sends an `albumsResult` / keyed `detailResult` and calls `Services.Invalidate()`.
   - `drainResults()` runs at the top of every `View` before anything can start a new fetch, so a worker blocked on the channel slot is always released. Detail results carry an `albumID|page` key: a stale result (album or page changed mid-fetch) is dropped, and a matched result records `membersFor` so `ensureDetailItems` does not refetch in a loop.
   - `albumsLoading` / `detailLoading` are the skeletons' signal; `detailLoading` clears only when the result for the current key drains, so a stale page never drops the skeleton early.
2. **Skeletons (`gallery_view.go`, `detail_view.go`)**:
   - Gallery: while the first summaries fetch is in flight with nothing to show, the grid renders 12 album-card skeletons (rounded cover block with `SkeletonSweep` plus name/meta rows) inside a `layout.Grid` labeled `albums-loading`, before the `No albums yet` empty state.
   - Detail: while the member page is in flight with no items, the view renders 24 `mediacard.SkeletonTile` placeholders in a wrapped row labeled `album-members-loading`, before the `No media in this album` empty state. The header count tag reads `Loading…` until the first page lands; the `%d items` format is unchanged afterwards.
3. **Staggered card reveal (`mediacard.ApplyReveal`)**:
   - When summaries first fill an empty gallery, the screen arms one `mediacard.Reveal` and `albumCard` applies it to each card: two-frame `AnimatePose` enter (`ui.Pose{Y: 8, Fade: 1}` → rest) with per-card slot delays — 18ms per card, 360ms cap, 300ms per-card duration, flat and restrained per `DESIGN.md`.
   - The member strip gets its own reveal through `mediagrid.Item.Reveal` when an album's members first arrive.
   - Reveals arm only on the empty→filled transition: `Reload()` refreshes, member pagination, and post-window cards never re-stagger.

---

## 5. File Structure

- `internal/ui/screens/albums/gallery_view.go`: Album gallery grid view.
- `internal/ui/screens/albums/detail_view.go`: Single album view with media grid.
- `internal/ui/screens/albums/album_card.go`: Album card widget with thumbnail preview.
- `internal/ui/screens/albums/modals/create.go`: `overlay.Modal` for album creation.
- `internal/ui/screens/albums/modals/edit.go`: `overlay.Modal` for album editing.
- `internal/ui/screens/albums/modals/delete.go`: `overlay.Modal` for album deletion.
- `internal/ui/screens/albums/modals/merge.go`: `overlay.Modal` for album merging.

---

## 6. Acceptance Criteria

- [ ] Album gallery renders cleanly with `layout.Grid` and `display.Tag` badges.
- [ ] Modals render with frosted backdrops and dismiss on Escape or Cancel.
- [ ] Creating/renaming an album updates both disk directories and SQLite records.
- [ ] Empty state renders `display.Empty` with functional create button.
