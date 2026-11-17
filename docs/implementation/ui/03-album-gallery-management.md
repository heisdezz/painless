# UI Feature 03: Album Gallery, Detail View & Action Modals

## 1. Overview

The Album Management interface organizes media into physical collections on the drive. It utilizes Lobe UI components for modal dialogs (`overlay.Modal`), inputs (`field.Input`, `field.TextArea`), badges (`display.Tag`), empty states (`display.Empty`), and context menus (`nav.ContextMenu`).

---

## 2. Views & Screens

### 1. Album Gallery (`internal/ui/screens/albums/gallery_view.go`)

- Responsive grid of album cards (`layout.Grid`).
- Each album card displays:
  - Album cover preview with rounded frame.
  - Album Title & Description.
  - Item count tag: `github.com/lobehub/lobe-mygo/pkg/display.Tag` (e.g. `142 items`).
  - Context menu trigger (`action.ActionIcon` with `lucide.MoreVertical`).
- Empty state: `github.com/lobehub/lobe-mygo/pkg/display.Empty` when no albums exist, prompting user to create one.
- Top bar action button: `action.Button` with `lucide.Plus` ("Create New Album").

### 2. Album Detail View (`internal/ui/screens/albums/detail_view.go`)

- Header banner with album title, editable description (`field.EditableText`), total file size, and item count.
- Action toolbar:
  - `action.Button`: "Add Media"
  - `action.Button`: "Validate Files"
  - `action.Button`: "Merge Into Another Album"
  - `action.Button`: "Delete Album" (Danger variant)
- Media grid displaying members of the selected album.

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

## 4. File Structure

- `internal/ui/screens/albums/gallery_view.go`: Album gallery grid view.
- `internal/ui/screens/albums/detail_view.go`: Single album view with media grid.
- `internal/ui/screens/albums/album_card.go`: Album card widget with thumbnail preview.
- `internal/ui/screens/albums/modals/create.go`: `overlay.Modal` for album creation.
- `internal/ui/screens/albums/modals/edit.go`: `overlay.Modal` for album editing.
- `internal/ui/screens/albums/modals/delete.go`: `overlay.Modal` for album deletion.
- `internal/ui/screens/albums/modals/merge.go`: `overlay.Modal` for album merging.

---

## 5. Acceptance Criteria

- [ ] Album gallery renders cleanly with `layout.Grid` and `display.Tag` badges.
- [ ] Modals render with frosted backdrops and dismiss on Escape or Cancel.
- [ ] Creating/renaming an album updates both disk directories and SQLite records.
- [ ] Empty state renders `display.Empty` with functional create button.
