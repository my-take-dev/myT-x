package panestate

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestManagerFeedSnapshot(t *testing.T) {
	manager := NewManager(1024)
	manager.EnsurePane("%0", 40, 8)
	manager.Feed("%0", []byte("hello world"))

	snapshot := manager.Snapshot("%0")
	if !strings.Contains(snapshot, "hello world") {
		t.Fatalf("snapshot does not contain terminal output: %q", snapshot)
	}
}

func TestManagerLazyEmulationForInactivePanes(t *testing.T) {
	manager := NewManager(1024)
	manager.EnsurePane("%0", 40, 8)
	manager.EnsurePane("%1", 40, 8)
	manager.SetActivePanes(map[string]struct{}{
		"%0": {},
	})

	manager.Feed("%0", []byte("active"))
	manager.Feed("%1", []byte("inactive"))

	inactiveSnapshot := manager.Snapshot("%1")
	if !strings.Contains(inactiveSnapshot, "inactive") {
		t.Fatalf("inactive pane snapshot should come from replay, got %q", inactiveSnapshot)
	}

	manager.SetActivePanes(map[string]struct{}{
		"%1": {},
	})
	activeSnapshot := manager.Snapshot("%1")
	if !strings.Contains(activeSnapshot, "inactive") {
		t.Fatalf("activated pane snapshot should keep buffered output, got %q", activeSnapshot)
	}
}

func TestManagerResizeAndRetain(t *testing.T) {
	manager := NewManager(1024)
	manager.EnsurePane("%0", 40, 8)
	manager.EnsurePane("%1", 40, 8)

	manager.ResizePane("%0", 80, 24)
	manager.Feed("%0", []byte("pane0"))
	manager.Feed("%1", []byte("pane1"))

	manager.RetainPanes(map[string]struct{}{
		"%0": {},
	})

	if manager.Snapshot("%1") != "" {
		t.Fatalf("pane %%1 should have been removed")
	}
	if !strings.Contains(manager.Snapshot("%0"), "pane0") {
		t.Fatalf("pane %%0 snapshot should remain available")
	}
}

func TestManagerReplayFallbackAndCap(t *testing.T) {
	manager := NewManager(3)
	manager.Feed("%0", []byte("    "))

	snapshot := manager.Snapshot("%0")
	if snapshot != "   " {
		t.Fatalf("snapshot = %q, want three spaces", snapshot)
	}
}

func TestManagerReplayRingKeepsTailOrder(t *testing.T) {
	ring := newReplayRing(5)
	ring.write([]byte("abc"))
	ring.write([]byte("def"))

	got := string(ring.snapshot())
	if got != "bcdef" {
		t.Fatalf("snapshot = %q, want %q", got, "bcdef")
	}
}

func TestManagerReplayRingHandlesLargeChunk(t *testing.T) {
	ring := newReplayRing(5)
	ring.write([]byte("0123456789"))

	got := string(ring.snapshot())
	if got != "56789" {
		t.Fatalf("snapshot = %q, want %q", got, "56789")
	}
}

func TestManagerConcurrentFeedSnapshot(t *testing.T) {
	manager := NewManager(4096)
	paneIDs := []string{"%0", "%1", "%2", "%3"}
	for _, id := range paneIDs {
		manager.EnsurePane(id, 80, 24)
	}
	manager.SetActivePanes(map[string]struct{}{
		"%0": {}, "%1": {}, "%2": {}, "%3": {},
	})

	var wg sync.WaitGroup

	// Concurrent feeds to different panes.
	for _, id := range paneIDs {
		wg.Go(func() {
			for i := range 1000 {
				manager.Feed(id, fmt.Appendf(nil, "data-%d\n", i))
			}
		})
	}

	// Concurrent snapshots from different panes.
	for _, id := range paneIDs {
		wg.Go(func() {
			for range 100 {
				_ = manager.Snapshot(id)
			}
		})
	}

	wg.Wait()

	// All panes should have data.
	for _, id := range paneIDs {
		snap := manager.Snapshot(id)
		if snap == "" {
			t.Errorf("pane %s snapshot is empty after concurrent feeds", id)
		}
	}
}

func TestManagerConcurrentFeedAndRemove(t *testing.T) {
	manager := NewManager(4096)
	manager.EnsurePane("%0", 80, 24)
	manager.SetActivePanes(map[string]struct{}{"%0": {}})

	var wg sync.WaitGroup

	// Feed in a loop.
	wg.Go(func() {
		for range 500 {
			manager.Feed("%0", []byte("data\n"))
		}
	})

	// Remove while feeding (should not panic).
	wg.Go(func() {
		for range 50 {
			manager.RemovePane("%0")
			manager.EnsurePane("%0", 80, 24)
			manager.SetActivePanes(map[string]struct{}{"%0": {}})
		}
	})

	wg.Wait()

	// Verify manager is in a consistent state after concurrent operations.
	// The pane should exist and be usable.
	manager.Feed("%0", []byte("final\n"))
	snap := manager.Snapshot("%0")
	if !strings.Contains(snap, "final") {
		t.Errorf("pane should be usable after concurrent feed/remove, snapshot = %q", snap)
	}
}

func TestManagerConcurrentFeedSnapshotRemove(t *testing.T) {
	manager := NewManager(4096)
	manager.EnsurePane("%0", 80, 24)
	manager.SetActivePanes(map[string]struct{}{"%0": {}})

	var wg sync.WaitGroup

	// goroutine 1: Feed loop.
	wg.Go(func() {
		for i := range 500 {
			manager.Feed("%0", fmt.Appendf(nil, "feed-%d\n", i))
		}
	})

	// goroutine 2: Snapshot loop.
	wg.Go(func() {
		for range 200 {
			_ = manager.Snapshot("%0")
		}
	})

	// goroutine 3: RemovePane + EnsurePane cycle.
	wg.Go(func() {
		for range 50 {
			manager.RemovePane("%0")
			manager.EnsurePane("%0", 80, 24)
			manager.SetActivePanes(map[string]struct{}{"%0": {}})
		}
	})

	wg.Wait()

	// After all goroutines finish, pane should be usable.
	manager.Feed("%0", []byte("final\n"))
	snap := manager.Snapshot("%0")
	if !strings.Contains(snap, "final") {
		t.Errorf("pane should be usable after 3-way concurrent access, snapshot = %q", snap)
	}
}

func TestReplayRingSnapshotInto(t *testing.T) {
	tests := []struct {
		name   string
		cap    int
		writes []string
		bufCap int
		want   string
	}{
		{
			name:   "nil buffer allocates internally",
			cap:    10,
			writes: []string{"hello"},
			bufCap: 0,
			want:   "hello",
		},
		{
			name:   "sufficient buffer reused",
			cap:    10,
			writes: []string{"abc"},
			bufCap: 16,
			want:   "abc",
		},
		{
			name:   "undersized buffer grows",
			cap:    10,
			writes: []string{"abcdefgh"},
			bufCap: 2,
			want:   "abcdefgh",
		},
		{
			name:   "wrapped ring copies correctly",
			cap:    5,
			writes: []string{"abc", "def"},
			bufCap: 10,
			want:   "bcdef",
		},
		{
			name:   "empty ring returns zero-length",
			cap:    10,
			writes: nil,
			bufCap: 8,
			want:   "",
		},
		{
			name:   "large chunk overwrites ring",
			cap:    5,
			writes: []string{"0123456789"},
			bufCap: 8,
			want:   "56789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ring := newReplayRing(tt.cap)
			for _, w := range tt.writes {
				ring.write([]byte(w))
			}

			var buf []byte
			if tt.bufCap > 0 {
				buf = make([]byte, 0, tt.bufCap)
			}

			got := ring.snapshotInto(buf)
			if string(got) != tt.want {
				t.Errorf("snapshotInto = %q, want %q", string(got), tt.want)
			}

			// Cross-check with snapshot() for non-empty.
			if tt.want != "" {
				ref := string(ring.snapshot())
				if string(got) != ref {
					t.Errorf("snapshotInto = %q, differs from snapshot() = %q", string(got), ref)
				}
			}
		})
	}
}

func BenchmarkReplayRingSnapshot(b *testing.B) {
	ring := newReplayRing(512 * 1024)
	data := make([]byte, 256*1024)
	for i := range data {
		data[i] = byte('A' + i%26)
	}
	ring.write(data)

	b.Run("snapshot_alloc", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = ring.snapshot()
		}
	})

	b.Run("snapshotInto_reuse", func(b *testing.B) {
		buf := make([]byte, 0, 512*1024)
		b.ReportAllocs()
		for b.Loop() {
			buf = ring.snapshotInto(buf)
		}
	})
}

func BenchmarkManagerConcurrentFeed(b *testing.B) {
	paneIDs := []string{"%0", "%1", "%2", "%3"}
	chunk := []byte("benchmark data line\n")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		manager := NewManager(4096)
		for _, id := range paneIDs {
			manager.EnsurePane(id, 80, 24)
		}
		manager.SetActivePanes(map[string]struct{}{
			"%0": {}, "%1": {}, "%2": {}, "%3": {},
		})

		var wg sync.WaitGroup
		for _, id := range paneIDs {
			wg.Go(func() {
				for range 100 {
					manager.Feed(id, chunk)
				}
			})
		}
		wg.Wait()
	}
}
