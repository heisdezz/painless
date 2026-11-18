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

## 4. Asynchronous Page Loading, Skeleton Grid & Reveal

The library database lives on the (possibly cold) USB drive, so the page query itself must never run on the UI thread.

1. **Off-thread page fetch (`catalog.go`)**:
   - `load()` runs at the top of every `View`. It first drains finished fetches from a one-slot `results` channel, then starts a worker goroutine for the current query key (`committedQuery|filter|sort`). Drain-before-start guarantees a worker blocked on the channel slot is always released, so at most two fetches are ever in flight and no send sticks.
   - Results carry their key; a stale result (query changed mid-fetch) is dropped, and a requery keeps the stale page visible (stale-while-revalidate) instead of flashing the skeleton again.
   - `s.fetchKey != ""` is the loading signal for the rest of the screen.
2. **Skeleton grid (`catalog_view.go`)**:
   - While the first page is pending (`no items && fetching`), the grid renders 30 skeleton tiles through `skeletonGridView`: same 168x188 chrome as the real card (10px padding, 8px gap, hairline border, radius) filled with `display.Skeleton` blocks — a 148x110 preview with `SkeletonSweep`, plus name and meta rows.
   - It carries its own `GridState` and the element label `catalog-loading` (tests), so it never borrows the real grid's scroll or selection state.
   - The filter bar's count tag reads `Loading…` until the first page lands; the `%d items` format is unchanged afterwards.
3. **Staggered card reveal (`mediacard`)**:
   - When the first page arrives on an empty grid, the screen creates one `mediacard.Reveal` (18ms per card, 360ms cap on slot delays, 300ms per-card duration) and passes it through `mediagrid.Item` into `mediacard.Props`.
   - `mediacard.Render` plays each card's entrance with the two-frame `AnimatePose` pattern: first build shows the card at `ui.Pose{Y: 8, Fade: 1}` and requests another frame; the next build animates it to rest after its slot delay. Flat and restrained per `DESIGN.md` — a subtle rise and fade, no bounce, no shadows.
   - Each card's slot delay is frozen in `ui.Local` state on its keyed node, so thumbnail-driven rebuilds never restart or re-delay a mid-flight entrance. After `Reveal.Active()` expires, cards render plainly and cards scrolled into the virtualized grid never re-animate.
   - The reveal fires only on the empty→items transition: requeries, `Reload()` after library mutations, and the dashboard's `mediagrid.Strip` (nil Reveal) are never re-staggered.

---

## 5. File Structure

- `internal/ui/screens/catalog/catalog.go`: Screen state, async page fetch (`load`/`drainResults`), query-key cache, thumbnail worker pool.
- `internal/ui/screens/catalog/catalog_view.go`: Main catalog screen view builder and the first-load skeleton grid.
- `internal/ui/screens/catalog/filter_bar.go`: `field.SearchBar`, `action.Segmented`, `field.Select`, item count / loading tag.
- `internal/ui/screens/catalog/action_bar.go`: Floating batch action toolbar with Lobe buttons.
- `internal/ui/screens/catalog/media_card.go`: Card data assembly (per-item `mediagrid.Item`, click semantics, selection helpers).
- `internal/ui/screens/catalog/context_menu.go`: Context-menu entries and command dispatch.
- `internal/ui/components/mediacard/card.go`: Shared media tile (fixed 168x188 metrics, skeleton while its bitmap loads, entrance reveal).
- `internal/ui/components/mediagrid/grid.go`: Virtualized grid (`Grid`) and wrapped strip (`Strip`) layouts over the shared card.

---

## 6. Acceptance Criteria

- [ ] Smooth 60 FPS scrolling with virtualized `ui.GridView`.
- [ ] The SQLite page query runs off-thread: the first visit renders the skeleton grid instead of blocking the UI.
- [ ] Cards stagger in (rise + fade, 18ms apart) when the first page arrives; requeries never re-run the stagger.
- [ ] `field.SearchBar` filters items with debounced text matching.
- [ ] `choice.Segmented` filters Photos vs Videos instantly with sliding animation.
- [ ] `display.Empty` displays when no items match search criteria.
- [ ] Right-click triggers `nav.ContextMenu` with functional actions.
