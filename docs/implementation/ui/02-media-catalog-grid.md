# UI Feature 02: Media Catalog Grid, Virtualization & Selection

## 1. Overview
The Media Catalog is the primary browsing interface. It displays thousands of media items in a virtualized, performant grid using MyGo's `ui.GridView`, with multi-selection, search, filtering, and batch operations.

---

## 2. Key Components & Features

1. **Virtualized Grid (`ui.GridView`)**:
   - Thousands of items cannot be rendered as simple DOM/layout elements without massive frame drops.
   - `ui.GridView` calculates visible rows based on window viewport and only builds visible cells.
   - Reusable cell configuration with stable item keys (`Key: func(i int) any { return items[i].ID }`).
2. **Multi-Selection (`ui.GridState.Selection`)**:
   - `Ctrl+Click` (or `Cmd+Click` on macOS): Toggle individual item in/out of selection.
   - `Shift+Click`: Contiguous range selection between last selected item and current item.
   - `Ctrl+A`: Select all displayed items.
   - Selection status reflected visually on the card border and badge checkmark.
3. **Filter Bar & Search**:
   - `ui.TextInput`: Debounced text search matching file path and metadata.
   - Segmented / Toggle Buttons: Filter by type (`All`, `Photos`, `Videos`).
   - Sort dropdown: Date (newest/oldest), Name (A-Z), Size (largest/smallest).
4. **Batch Action Toolbar**:
   - Appears when 1 or more items are selected.
   - Actions:
     - "Move to Album": Opens destination album picker dialog.
     - "Remove from Album": Clears album assignment.
     - "Delete Selected": Confirms removal from drive/library.
     - "Deselect All": Clears selection.

---

## 3. Asynchronous Thumbnail Rendering in Grid

To keep scrolling at 60 FPS:
```go
func (s *CatalogScreen) renderCell(c *ui.Context, item domain.MediaItem) {
    thumbBitmap := s.cache.Get(item.FileHash)
    if thumbBitmap == nil {
        // Thumbnail not yet in RAM; request async background load
        s.cache.RequestLoad(item)
        // Show placeholder while loading
        ui.Placeholder(c).Width(160).Height(120)
        return
    }
    // Draw cached bitmap
    ui.Image(c, thumbBitmap).Width(160).Height(120).Fit(ui.Cover)
}
```

---

## 4. File Structure
- `internal/ui/screens/catalog/catalog_view.go`: Main catalog screen view builder.
- `internal/ui/screens/catalog/filter_bar.go`: Search input, type filter, sort selector.
- `internal/ui/screens/catalog/action_bar.go`: Bottom floating batch actions toolbar.
- `internal/ui/screens/catalog/media_card.go`: Single card cell layout with badge, duration, size.

---

## 5. Acceptance Criteria
- [ ] Smooth scrolling at 60 FPS across 5,000+ cataloged media items.
- [ ] Multi-selection behaves identically to standard OS file managers (Ctrl/Shift/Cmd+A).
- [ ] Batch operations correctly pass IDs to file operations and update the database.
- [ ] Search and filter operations apply with debouncing and instant response.
