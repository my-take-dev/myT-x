package devpanel

import (
	"strings"
	"sync"
	"time"
)

const defaultDirCacheTTL = 5 * time.Second

const (
	dirCacheSweepInterval  = 32
	dirCacheSweepThreshold = 512
	dirCacheKeySeparator   = "\x00"
)

type cachedDirEntries struct {
	entries   []FileEntry
	expiresAt time.Time
}

// DirCache stores short-lived directory listings per session/path.
type DirCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	nowFunc func() time.Time
	entries map[string]cachedDirEntries
	sets    uint64
}

func NewDirCache(ttl time.Duration) *DirCache {
	if ttl <= 0 {
		ttl = defaultDirCacheTTL
	}

	return &DirCache{
		ttl:     ttl,
		nowFunc: time.Now,
		entries: make(map[string]cachedDirEntries),
	}
}

func makeDirCacheKey(sessionName, dirPath string) string {
	return sessionName + dirCacheKeySeparator + normalizePanelPath(dirPath)
}

func cloneFileEntries(entries []FileEntry) []FileEntry {
	if len(entries) == 0 {
		return []FileEntry{}
	}

	cloned := make([]FileEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func (c *DirCache) Get(sessionName, dirPath string) ([]FileEntry, bool) {
	key := makeDirCacheKey(sessionName, dirPath)
	now := c.nowFunc()

	c.mu.RLock()
	cached, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}

	if !now.Before(cached.expiresAt) {
		c.mu.Lock()
		current, stillPresent := c.entries[key]
		if stillPresent && !now.Before(current.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	return cloneFileEntries(cached.entries), true
}

func (c *DirCache) Set(sessionName, dirPath string, entries []FileEntry) {
	key := makeDirCacheKey(sessionName, dirPath)
	now := c.nowFunc()

	c.mu.Lock()
	c.entries[key] = cachedDirEntries{
		entries:   cloneFileEntries(entries),
		expiresAt: now.Add(c.ttl),
	}
	c.sets++
	if len(c.entries) > dirCacheSweepThreshold || c.sets%dirCacheSweepInterval == 0 {
		c.evictExpiredLocked(now)
	}
	c.mu.Unlock()
}

func (c *DirCache) Invalidate(sessionName, dirPath string) {
	key := makeDirCacheKey(sessionName, dirPath)
	prefix := key
	if normalizePanelPath(dirPath) != "" {
		prefix += "/"
	}

	c.mu.Lock()
	for currentKey := range c.entries {
		if currentKey == key || strings.HasPrefix(currentKey, prefix) {
			delete(c.entries, currentKey)
		}
	}
	c.mu.Unlock()
}

func (c *DirCache) InvalidateAll(sessionName string) {
	prefix := sessionName + dirCacheKeySeparator

	c.mu.Lock()
	for currentKey := range c.entries {
		if strings.HasPrefix(currentKey, prefix) {
			delete(c.entries, currentKey)
		}
	}
	c.mu.Unlock()
}

func (c *DirCache) evictExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}
