package devpanel

import (
	"fmt"
	"testing"
	"time"
)

func TestDirCacheClonesEntries(t *testing.T) {
	cache := NewDirCache(time.Minute)
	entries := []FileEntry{{Name: "src", Path: "src", IsDir: true, HasChildren: true}}

	cache.Set("session", "", entries)
	entries[0].Name = "mutated"

	cachedEntries, ok := cache.Get("session", "")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if cachedEntries[0].Name != "src" {
		t.Fatalf("cached entry mutated: got %q, want %q", cachedEntries[0].Name, "src")
	}

	cachedEntries[0].Name = "changed-again"
	cachedEntriesSecondRead, ok := cache.Get("session", "")
	if !ok {
		t.Fatal("expected cache hit on second read")
	}
	if cachedEntriesSecondRead[0].Name != "src" {
		t.Fatalf("cache get should return clones: got %q, want %q", cachedEntriesSecondRead[0].Name, "src")
	}
	if !cachedEntriesSecondRead[0].HasChildren {
		t.Fatal("HasChildren should be preserved in cloned entries")
	}
}

func TestDirCacheExpiresEntries(t *testing.T) {
	cache := NewDirCache(time.Second)
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	cache.Set("session", "", []FileEntry{{Name: "src", Path: "src", IsDir: true}})

	now = now.Add(1500 * time.Millisecond)
	if _, ok := cache.Get("session", ""); ok {
		t.Fatal("expected cache entry to expire")
	}
}

func TestDirCacheHasChildrenUsesFreshEntries(t *testing.T) {
	cache := NewDirCache(time.Second)
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	cache.Set("session", "empty", []FileEntry{})
	cache.Set("session", "non-empty", []FileEntry{{Name: "child.txt", Path: "non-empty/child.txt"}})

	hasChildren, ok := cache.HasChildren("session", "empty")
	if !ok {
		t.Fatal("expected fresh cache hit for empty directory")
	}
	if hasChildren {
		t.Fatal("empty directory cache should report no children")
	}

	hasChildren, ok = cache.HasChildren("session", "non-empty")
	if !ok {
		t.Fatal("expected fresh cache hit for non-empty directory")
	}
	if !hasChildren {
		t.Fatal("non-empty directory cache should report children")
	}

	now = now.Add(2 * time.Second)
	if _, ok := cache.HasChildren("session", "non-empty"); ok {
		t.Fatal("expired cache should not report HasChildren")
	}
}

func TestDirCacheInvalidatesSubtrees(t *testing.T) {
	cache := NewDirCache(time.Minute)
	cache.Set("session", "", []FileEntry{{Name: "src", Path: "src", IsDir: true}})
	cache.Set("session", "src", []FileEntry{{Name: "nested", Path: "src/nested", IsDir: true}})
	cache.Set("session", "src/nested", []FileEntry{{Name: "deep.txt", Path: "src/nested/deep.txt"}})

	cache.Invalidate("session", "src")

	if _, ok := cache.Get("session", "src"); ok {
		t.Fatal("expected direct key to be invalidated")
	}
	if _, ok := cache.Get("session", "src/nested"); ok {
		t.Fatal("expected descendant key to be invalidated")
	}
	if _, ok := cache.Get("session", ""); !ok {
		t.Fatal("expected sibling cache entry to remain")
	}
}

func TestDirCacheSetSweepsExpiredEntries(t *testing.T) {
	cache := NewDirCache(time.Second)
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	cache.Set("session", "stale", []FileEntry{{Name: "stale.txt", Path: "stale/stale.txt"}})

	now = now.Add(2 * time.Second)
	for i := range dirCacheSweepInterval - 1 {
		cache.Set("session", fmt.Sprintf("warmup/%d", i), []FileEntry{{Name: "warmup.txt", Path: fmt.Sprintf("warmup/%d/file.txt", i)}})
	}
	cache.Set("session", "fresh", []FileEntry{{Name: "fresh.txt", Path: "fresh/fresh.txt"}})

	if _, ok := cache.Get("session", "stale"); ok {
		t.Fatal("expected expired entry to be swept on Set")
	}
	if _, ok := cache.Get("session", "fresh"); !ok {
		t.Fatal("expected fresh entry to remain in cache")
	}
}

func TestDirCacheKeysDoNotCollideAcrossSessionAndPathBoundaries(t *testing.T) {
	cache := NewDirCache(time.Minute)

	cache.Set("a", "b/c", []FileEntry{{Name: "first", Path: "b/c/first.txt"}})
	cache.Set("a/b", "c", []FileEntry{{Name: "second", Path: "c/second.txt"}})

	firstEntries, ok := cache.Get("a", "b/c")
	if !ok || len(firstEntries) != 1 || firstEntries[0].Name != "first" {
		t.Fatalf("unexpected first cache entry: ok=%v entries=%v", ok, firstEntries)
	}
	secondEntries, ok := cache.Get("a/b", "c")
	if !ok || len(secondEntries) != 1 || secondEntries[0].Name != "second" {
		t.Fatalf("unexpected second cache entry: ok=%v entries=%v", ok, secondEntries)
	}
}
