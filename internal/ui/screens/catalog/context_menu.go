package catalog

import (
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/overlay"

	"painless/internal/domain"
)

func contextItems() []overlay.MenuItem {
	return []overlay.MenuItem{
		{Key: "open", Label: "Open in Viewer", Icon: lucide.Image()},
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
