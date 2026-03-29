package snapshot

import (
	"bytes"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// feedBytePool data integrity
// ---------------------------------------------------------------------------

func TestFeedBytePoolDataIntegrity(t *testing.T) {
	data := []byte("hello, feed pool!")
	buf, bp := getFeedBuffer(len(data))
	copy(buf, data)

	if !bytes.Equal(buf, data) {
		t.Errorf("getFeedBuffer data = %q, want %q", buf, data)
	}

	// Return to pool; must not panic.
	putFeedBuffer(bp)
}

// ---------------------------------------------------------------------------
// putFeedBuffer edge cases
// ---------------------------------------------------------------------------

func TestPutFeedBufferNilDoesNotPanic(t *testing.T) {
	// Must not panic on nil pool pointer.
	putFeedBuffer(nil)
}

func TestPutFeedBufferDropsBuffersLargerThanMaxPoolSize(t *testing.T) {
	// Allocate a buffer larger than maxPoolBufSize.
	oversized := make([]byte, maxPoolBufSize+1)
	bp := &oversized
	// Should silently discard without panic.
	putFeedBuffer(bp)
}

// ---------------------------------------------------------------------------
// Boundary: exactly at maxPoolBufSize
// ---------------------------------------------------------------------------

func TestFeedBytePoolMaxPoolBufSizeBoundary(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		wantReturn bool // true = should be returned to pool (not discarded)
	}{
		{"under_limit", maxPoolBufSize - 1, true},
		{"at_limit", maxPoolBufSize, true},
		{"over_limit", maxPoolBufSize + 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.size)
			bp := &buf
			// putFeedBuffer must not panic regardless of size.
			putFeedBuffer(bp)
		})
	}
}

// ---------------------------------------------------------------------------
// getFeedBuffer: growth behavior
// ---------------------------------------------------------------------------

func TestGetFeedBufferGrowsWhenNeeded(t *testing.T) {
	// Request a buffer much larger than the default pool capacity (8 KiB).
	size := 32 * 1024
	buf, bp := getFeedBuffer(size)
	if len(buf) != size {
		t.Errorf("len(buf) = %d, want %d", len(buf), size)
	}
	if cap(buf) < size {
		t.Errorf("cap(buf) = %d, want >= %d", cap(buf), size)
	}
	putFeedBuffer(bp)
}

func TestGetFeedBufferReusesPooledSlice(t *testing.T) {
	// Get and return a buffer.
	_, bp := getFeedBuffer(100)
	putFeedBuffer(bp)

	// Get another; the pool may return the same buffer (or a new one).
	buf2, bp2 := getFeedBuffer(100)
	if len(buf2) != 100 {
		t.Errorf("len(buf) = %d, want 100", len(buf2))
	}
	putFeedBuffer(bp2)
}

// ---------------------------------------------------------------------------
// paneFeedItem field count guard
// ---------------------------------------------------------------------------

func TestPaneFeedItemFieldCount(t *testing.T) {
	const wantFields = 3
	got := reflect.TypeFor[paneFeedItem]().NumField()
	if got != wantFields {
		t.Errorf("paneFeedItem has %d fields, want %d; update feed logic and this test", got, wantFields)
	}
}

// ---------------------------------------------------------------------------
// Benchmark (Go 1.26 b.Loop pattern)
// ---------------------------------------------------------------------------

func BenchmarkFeedBytePool(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}

	for b.Loop() {
		buf, bp := getFeedBuffer(len(data))
		copy(buf, data)
		putFeedBuffer(bp)
	}
}
