package main

import "time"

const (
	// inputHistoryDir is the subdirectory under the config directory where
	// JSONL history files are stored.
	inputHistoryDir = "input-history"

	// inputHistoryMaxFiles caps the number of retained history files.
	// 50 files at ~10K entries each covers weeks of typical usage.
	inputHistoryMaxFiles = 50

	// inputHistoryMaxEntries is the in-memory ring buffer capacity.
	// 10000 entries provides a long scrollback while bounding memory to ~4 MB.
	inputHistoryMaxEntries = 10000

	// inputHistoryEmitMinInterval throttles "app:input-history-updated" ping
	// events to prevent Wails IPC saturation under rapid input.
	inputHistoryEmitMinInterval = 50 * time.Millisecond

	// inputLineFlushTimeout is the inactivity timeout for flushing an incomplete
	// line buffer. If no Enter is received within this duration, the partial
	// buffer is written as-is (e.g. for password prompts or interactive mode).
	inputLineFlushTimeout = 5 * time.Second

	// inputHistoryMaxInputLen caps the rune count of a single history entry.
	// 4000 runes accommodates long paste operations while preventing unbounded growth.
	inputHistoryMaxInputLen = 4000

	// shutdownFlushSentinel is the generation value used for forced shutdown
	// flushes. Using ^uint64(0) (math.MaxUint64) avoids collision with normal
	// timer generations, which increment from 0 upward and cannot realistically
	// reach this value within any process lifetime.
	shutdownFlushSentinel = ^uint64(0)
)

// InputHistoryEntry represents a single input history record.
type InputHistoryEntry struct {
	Seq       uint64 `json:"seq"`     // cumulative auto-incrementing counter (assigned by writeInputHistoryEntry)
	Timestamp string `json:"ts"`      // "20060102150405" format
	PaneID    string `json:"pane_id"` // target pane identifier
	Input     string `json:"input"`   // user input text (may contain control characters)
	Source    string `json:"source"`  // "keyboard", "sync-input"
	Session   string `json:"session"` // session name (may be empty)
}

// inputLineBuffer accumulates keystrokes for a single pane until Enter (\r)
// is received, producing one history entry per logical command line.
//
// Not safe for concurrent use; callers must hold inputLineBufMu.
type inputLineBuffer struct {
	buf      []rune      // accumulated input text for the current line
	timer    *time.Timer // fires after inputLineFlushTimeout of inactivity; nil when idle
	timerGen uint64      // generation counter: incremented each time a new timer is started
	paneID   string
	source   string
	session  string
}

// inputHistoryRingBuffer is a fixed-capacity circular buffer for InputHistoryEntry.
// It avoids O(N) slice copies on every append by overwriting the oldest entry
// when full, using a head index and a count to track the logical window.
//
// Not safe for concurrent use; callers must hold inputHistoryMu.
type inputHistoryRingBuffer struct {
	buf   []InputHistoryEntry // fixed-size backing array
	head  int                 // index of the oldest entry (next to be overwritten when full)
	count int                 // number of valid entries (0..cap)
}

// newInputHistoryRingBuffer allocates a ring buffer with the given capacity.
// Capacity values <= 0 are clamped to 1 to prevent modulo-by-zero panics.
func newInputHistoryRingBuffer(capacity int) inputHistoryRingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return inputHistoryRingBuffer{
		buf: make([]InputHistoryEntry, capacity),
	}
}

// push appends an entry to the ring buffer. When full, the oldest entry is
// overwritten. Amortized O(1) with no allocation after initial construction.
func (rb *inputHistoryRingBuffer) push(entry InputHistoryEntry) {
	bufCap := len(rb.buf)
	if bufCap == 0 {
		return
	}
	if rb.count < bufCap {
		rb.buf[(rb.head+rb.count)%bufCap] = entry
		rb.count++
	} else {
		rb.buf[rb.head] = entry
		rb.head = (rb.head + 1) % bufCap
	}
}

// snapshot returns a newly allocated slice containing all entries in
// chronological order (oldest first). The returned slice is independent of the
// ring buffer's internal storage.
func (rb *inputHistoryRingBuffer) snapshot() []InputHistoryEntry {
	if rb.count == 0 {
		return []InputHistoryEntry{}
	}

	out := make([]InputHistoryEntry, rb.count)
	bufCap := len(rb.buf)

	first := min(bufCap-rb.head, rb.count)
	copy(out, rb.buf[rb.head:rb.head+first])

	if rest := rb.count - first; rest > 0 {
		copy(out[first:], rb.buf[:rest])
	}
	return out
}

// len returns the number of valid entries currently stored.
func (rb *inputHistoryRingBuffer) len() int {
	return rb.count
}
