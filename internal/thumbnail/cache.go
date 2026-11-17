package thumbnail

import (
	"container/list"
	"os"
	"path/filepath"
	"sync"
)

// diskCache stores generated WebP thumbnails on the drive itself, so a
// library keeps its previews when it moves to another computer.
type diskCache struct {
	root string
}

func newDiskCache(driveRoot string) *diskCache {
	return &diskCache{root: filepath.Join(driveRoot, ".media_thumbnails")}
}

// path follows the <hash[:2]>/<hash>.webp layout.
func (c *diskCache) path(hash string) string {
	prefix := hash
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return filepath.Join(c.root, prefix, hash+".webp")
}

func (c *diskCache) get(hash string) ([]byte, bool) {
	data, err := os.ReadFile(c.path(hash))
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *diskCache) put(hash string, data []byte) error {
	path := c.path(hash)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".thumb-*.webp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (c *diskCache) remove(hash string) error {
	err := os.Remove(c.path(hash))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// memoryCache is a bounded LRU of WebP bytes. Get returns a slice owned by
// the cache; callers must treat it as read-only.
type memoryCache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

type cacheEntry struct {
	key  string
	data []byte
}

func newMemoryCache(capacity int) *memoryCache {
	if capacity < 1 {
		capacity = 1
	}
	return &memoryCache{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

func (c *memoryCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(element)
	return element.Value.(*cacheEntry).data, true
}

func (c *memoryCache) put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		element.Value.(*cacheEntry).data = data
		c.order.MoveToFront(element)
		return
	}
	c.items[key] = c.order.PushFront(&cacheEntry{key: key, data: data})
	for c.order.Len() > c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*cacheEntry).key)
	}
}

func (c *memoryCache) remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		c.order.Remove(element)
		delete(c.items, key)
	}
}

func (c *memoryCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
