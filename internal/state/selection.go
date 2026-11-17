package state

import "sync"

// Selection is a thread-safe, insertion-ordered set of selected media item
// IDs with an anchor used for shift-range selection in the catalog grid.
type Selection struct {
	mu     sync.RWMutex
	items  map[int64]struct{}
	order  []int64
	anchor int64
}

// NewSelection creates an empty selection.
func NewSelection() Selection {
	return Selection{items: map[int64]struct{}{}}
}

// Clear removes every selection and resets the anchor.
func (sel *Selection) Clear() {
	sel.mu.Lock()
	defer sel.mu.Unlock()
	sel.items = map[int64]struct{}{}
	sel.order = nil
	sel.anchor = 0
}

// Set replaces the selection with ids and sets the anchor to the last id.
func (sel *Selection) Set(ids []int64) {
	sel.mu.Lock()
	defer sel.mu.Unlock()
	sel.items = make(map[int64]struct{}, len(ids))
	sel.order = append([]int64(nil), ids...)
	for _, id := range ids {
		sel.items[id] = struct{}{}
	}
	if n := len(sel.order); n > 0 {
		sel.anchor = sel.order[n-1]
	} else {
		sel.anchor = 0
	}
}

// Toggle adds id when absent, removes it when present, and moves the
// anchor to id. Returns true when id is selected after the toggle.
func (sel *Selection) Toggle(id int64) bool {
	sel.mu.Lock()
	defer sel.mu.Unlock()
	if _, ok := sel.items[id]; ok {
		delete(sel.items, id)
		for i, v := range sel.order {
			if v == id {
				sel.order = append(sel.order[:i], sel.order[i+1:]...)
				break
			}
		}
		sel.anchor = id
		return false
	}
	sel.add(id)
	sel.anchor = id
	return true
}

// Add selects id, preserving existing selections, and moves the anchor.
func (sel *Selection) Add(id int64) {
	sel.mu.Lock()
	defer sel.mu.Unlock()
	sel.add(id)
	sel.anchor = id
}

// AddRange selects every id in the inclusive range low..high and moves the
// anchor to end. Existing selections are preserved.
func (sel *Selection) AddRange(start, end int64) {
	if end < start {
		start, end = end, start
	}
	sel.mu.Lock()
	defer sel.mu.Unlock()
	for id := start; id <= end; id++ {
		sel.add(id)
	}
	sel.anchor = end
}

// Anchor returns the current anchor id for range selection (0 when unset).
func (sel *Selection) Anchor() int64 {
	sel.mu.RLock()
	defer sel.mu.RUnlock()
	return sel.anchor
}

// SetAnchor moves the range-selection anchor without changing membership.
func (sel *Selection) SetAnchor(id int64) {
	sel.mu.Lock()
	defer sel.mu.Unlock()
	sel.anchor = id
}

// Has reports whether id is selected.
func (sel *Selection) Has(id int64) bool {
	sel.mu.RLock()
	defer sel.mu.RUnlock()
	_, ok := sel.items[id]
	return ok
}

// List returns the selected ids in selection order.
func (sel *Selection) List() []int64 {
	sel.mu.RLock()
	defer sel.mu.RUnlock()
	return append([]int64(nil), sel.order...)
}

// Count returns the number of selected ids.
func (sel *Selection) Count() int {
	sel.mu.RLock()
	defer sel.mu.RUnlock()
	return len(sel.order)
}

// RetainAll drops selections not present in ids, keeping order. Used when
// the catalog refreshes and some selected items no longer exist.
func (sel *Selection) RetainAll(ids []int64) {
	keep := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		keep[id] = struct{}{}
	}
	sel.mu.Lock()
	defer sel.mu.Unlock()
	order := sel.order[:0]
	for _, id := range sel.order {
		if _, ok := keep[id]; ok {
			order = append(order, id)
		} else {
			delete(sel.items, id)
		}
	}
	sel.order = order
}

func (sel *Selection) add(id int64) {
	if _, ok := sel.items[id]; !ok {
		sel.items[id] = struct{}{}
		sel.order = append(sel.order, id)
	}
}
