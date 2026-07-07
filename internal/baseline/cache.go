package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Cache is the optional long-term template memory. It lets the novelty
// detector distinguish "new since yesterday" from "new ever".
type Cache struct {
	Path string
	Seen map[string]CacheEntry `json:"seen"`
}

type CacheEntry struct {
	Masked string    `json:"masked"`
	First  time.Time `json:"first"`
	Last   time.Time `json:"last"`
}

// NewCache returns an empty in-memory cache (Path may be "" for tests).
func NewCache(path string) *Cache {
	return &Cache{Path: path, Seen: map[string]CacheEntry{}}
}

// LoadCache reads the cache file; a missing or corrupt file yields an empty
// cache (the cache is an optimization, never a hard dependency).
func LoadCache(path string) (*Cache, error) {
	c := NewCache(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return c, nil // missing file: start empty
	}
	if err := json.Unmarshal(data, c); err != nil {
		c.Seen = map[string]CacheEntry{} // corrupt: start empty
	}
	c.Path = path
	return c, nil
}

func (c *Cache) Has(id string) bool {
	_, ok := c.Seen[id]
	return ok
}

// Remember records a template sighting.
func (c *Cache) Remember(id, masked string, ts time.Time) {
	e, ok := c.Seen[id]
	if !ok {
		c.Seen[id] = CacheEntry{Masked: masked, First: ts, Last: ts}
		return
	}
	if ts.After(e.Last) {
		e.Last = ts
	}
	c.Seen[id] = e
}

// Save writes atomically (unique tmp + rename). The tmp name must be unique
// per process: overlapping cron invocations sharing one ".tmp" would clobber
// each other's data mid-write.
func (c *Cache) Save() error {
	if c.Path == "" {
		return nil
	}
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".patterns-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), c.Path)
}
