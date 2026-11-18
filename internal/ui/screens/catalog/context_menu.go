package catalog

import (
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/overlay"

	"painless/internal/domain"
)

func contextItems(selected bool) []overlay.MenuItem {
	selectItem := overlay.MenuItem{Key: "toggle", Label: "Select", Icon: lucide.CheckSquare()}
	if selected {
		selectItem = overlay.MenuItem{Key: "toggle", Label: "Deselect", Icon: lucide.Square()}
	}
	return []overlay.MenuItem{
		{Key: "open", Label: "Open in Viewer", Icon: lucide.Image()},
		selectItem,
		{Divider: true},
		{Key: "assign", Label: "Assign to Album...", Icon: lucide.FolderPlus()},
		{Key: "remove", Label: "Remove from Album", Icon: lucide.FolderMinus()},
		{Divider: true},
		{Key: "reveal", Label: "Reveal in File Manager", Icon: lucide.FolderOpen()},
		{Key: "inspect", Label: "Inspect EXIF Metadata", Icon: lucide.Info()},
		{Key: "delete", Label: "Delete", Icon: lucide.Trash2(), Danger: true},
	}
}

func (s *Screen) handleContext(key string, item domain.MediaItem) {
	ids := s.SelectedIDs()
	if !s.selection.Has(item.ID) {
		ids = []int64{item.ID}
	}
	switch key {
	case "open":
		s.actions.OpenInViewer(item)
	case "toggle":
		s.toggleSelectionByID(item.ID)
	case "assign":
		s.actions.AssignToAlbum(ids)
	case "remove":
		s.actions.RemoveFromAlbum(ids)
	case "reveal":
		s.actions.Reveal(item)
	case "inspect":
		s.actions.Inspect(item)
	case "delete":
		s.actions.Delete(ids)
		s.selection.Clear()
	}
}

// toggleSelectionByID adds or removes an item by id and keeps the keyboard
// focus in step when the item is present on the current page.
func (s *Screen) toggleSelectionByID(id int64) {
	if s.selection.Has(id) {
		s.selection.Remove(id)
	} else {
		s.selection.Add(id)
	}
	for i := range s.page.Items {
		if s.page.Items[i].ID == id {
			s.selected, s.pivot = i, i
			return
		}
	}
}
