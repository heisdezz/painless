# UI Feature 03: Album Gallery, Detail View & Action Modals

## 1. Overview
The Album Management interface allows users to organize media into physical collections on the drive. It consists of the Album Gallery, the Album Detail view, and native modal dialogs for CRUD operations.

---

## 2. Views & Screens

### 1. Album Gallery (`internal/ui/screens/albums/gallery_view.go`)
- Responsive grid of album cards.
- Each album card displays:
  - Cover preview thumbnail (first item in album or customized cover).
  - Album Name & Item Count badge (e.g. "Summer Vacation · 142 items").
  - Context menu / quick actions: Edit, Merge, Delete, Validate.
- Top bar action: **"+ Create New Album"**.

### 2. Album Detail View (`internal/ui/screens/albums/detail_view.go`)
- Header banner with album title, editable description, total file size, and item count.
- Action toolbar: "Add Media", "Validate Files", "Merge Into Another Album", "Delete Album".
- Media grid containing only items assigned to this album.

---

## 3. Modal Dialogs (`internal/ui/screens/albums/modals/`)

All modals are rendered using MyGo dialog primitives (`ui.Modal` / `ui.Dialog`):

1. **Create Album Modal**:
   - `ui.TextInput` for Name (validates against filesystem invalid characters: `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`).
   - `ui.TextArea` for Description.
   - Primary "Create" button & "Cancel" button.
2. **Edit Album Modal**:
   - Rename existing album (updates both DB and folder on disk).
3. **Delete Album Modal**:
   - Options:
     - *Soft Delete*: Keep files, move back to unassigned (`unknown/`).
     - *Permanent Delete*: Physically delete files from drive (requires typing "DELETE" or explicit confirmation checkbox).
4. **Merge Albums Modal**:
   - Source album and target destination picker.
   - Warning dialog detailing that all files will be consolidated into the target album directory.

---

## 4. File Structure
- `internal/ui/screens/albums/gallery_view.go`: Album gallery grid view.
- `internal/ui/screens/albums/detail_view.go`: Single album view with media grid.
- `internal/ui/screens/albums/album_card.go`: Album card widget with thumbnail stack.
- `internal/ui/screens/albums/modals/create.go`: Create album dialog.
- `internal/ui/screens/albums/modals/edit.go`: Edit album dialog.
- `internal/ui/screens/albums/modals/delete.go`: Delete album dialog.
- `internal/ui/screens/albums/modals/merge.go`: Merge albums dialog.

---

## 5. Acceptance Criteria
- [ ] Creating an album creates both the DB entry and the physical directory `albums/<name>`.
- [ ] Renaming an album safely renames the directory without breaking item paths.
- [ ] Merging albums moves all member files into target directory and cleans up the source folder.
- [ ] Modals capture focus properly and dismiss on Escape or Cancel.
