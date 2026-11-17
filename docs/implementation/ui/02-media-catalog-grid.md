# UI Feature 02: Media Catalog Grid, Virtualization & Selection

## 1. Overview

The Media Catalog is the primary media browsing view. It displays thousands of media items using MyGo's virtualized `ui.GridView` alongside Lobe UI components for search (`field.SearchBar`), filtering (`choice.Segmented`), sorting (`field.Select`), and empty states (`display.Empty`).

---

## 2. Key Components & Lobe Integration

1. **Search & Filter Bar (`internal/ui/screens/catalog/filter_bar.go`)**:
   - `github.com/lobehub/lobe-mygo/pkg/field.SearchBar`: Debounced search with search icon, clear button, and window-wide shortcut key tag (`Cmd/Ctrl+F`).
   - `github.com/lobehub/lobe-mygo/pkg/choice.Segmented`: Sliding pill segment indicator choosing between `All`, `Photos`, and `Videos`.
   - `github.com/lobehub/lobe-mygo/pkg/field.Select`: Sort dropdown (Newest First, Oldest First, Name A-Z, File Size).
   - Item count badge: `github.com/lobehub/lobe-mygo/pkg/display.Tag` or `feedback.Badge`.
2. **Virtualized Media Grid (`ui.GridView`)**:
   - High-throughput virtualized rendering for 10,000+ items.
   - Reusable cell configuration with stable keys (`Key: func(i int) any { return items[i].ID }`).
   - Cards display video duration overlay badge (`display.Tag`) and file size.
3. **Empty State (`display.Empty`)**:
   - When no media matches search filters or the library is empty, render `github.com/lobehub/lobe-mygo/pkg/display.Empty` with `lucide.FolderSearch`, descriptive text, and a "Start Scan" action button (`action.Button`).
4. **Multi-Selection & Batch Action Bar**:
   - Multi-selection powered by `ui.GridState.Selection` (Ctrl+Click, Shift+Click, Ctrl+A).
   - Floating batch action toolbar (`internal/ui/screens/catalog/action_bar.go`):
     - `action.Button`: "Move to Album..." (Primary)
     - `action.Button`: "Remove from Album"
     - `action.Button`: "Delete Selected" (Danger variant)
     - `action.ActionIcon`: `lucide.X` to clear selection.
5. **Context Menu (`nav.ContextMenu`)**:
   - Right-click on any media card opens a Lobe `nav.ContextMenu` with options:
     - Open in Viewer
     - Assign to Album...
     - Reveal in File Manager
     - Inspect EXIF Metadata
     - Delete

---

## 3. Asynchronous Thumbnail Rendering in Grid

To maintain 60 FPS scrolling:

```go
func (s *CatalogScreen) renderCell(c *ui.Context, item domain.MediaItem) {
    thumbBitmap := s.cache.Get(item.FileHash)
    if thumbBitmap == nil {
        s.cache.RequestLoad(item)
        // Lobe skeleton block during load
        display.Skeleton(c).Width(160).Height(120).Radius(8)
        return
    }
    ui.Image(c, thumbBitmap).Width(160).Height(120).Fit(ui.Cover)
}
```

---

## 4. File Structure

- `internal/ui/screens/catalog/catalog_view.go`: Main catalog screen view builder.
- `internal/ui/screens/catalog/filter_bar.go`: `field.SearchBar`, `choice.Segmented`, `field.Select`.
- `internal/ui/screens/catalog/action_bar.go`: Floating batch action toolbar with Lobe buttons.
- `internal/ui/screens/catalog/media_card.go`: Single card layout with hover highlights and badges.

---

## 5. Acceptance Criteria

- [ ] Smooth 60 FPS scrolling with virtualized `ui.GridView`.
- [ ] `field.SearchBar` filters items with debounced text matching.
- [ ] `choice.Segmented` filters Photos vs Videos instantly with sliding animation.
- [ ] `display.Empty` displays when no items match search criteria.
- [ ] Right-click triggers `nav.ContextMenu` with functional actions.
