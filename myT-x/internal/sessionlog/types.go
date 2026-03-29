package sessionlog

import "time"

const (
	// Dir is the subdirectory under the config directory where JSONL session log files are stored.
	Dir = "session-logs"

	// MaxFiles caps the number of retained session log files.
	// 100 files provides extensive history while bounding disk usage.
	MaxFiles = 100

	// MaxEntries is the in-memory ring buffer capacity.
	// 10000 entries provides a long scrollback while bounding memory to ~4 MB.
	MaxEntries = 10000

	// emitMinInterval throttles "app:session-log-updated" ping events
	// to prevent Wails IPC saturation under rapid logging.
	emitMinInterval = 50 * time.Millisecond

	// FrontendLogMaxMsgLen caps the rune count of frontend-submitted
	// messages to prevent unbounded JSONL file growth.
	FrontendLogMaxMsgLen = 2000

	// FrontendLogMaxSourceLen caps the rune count of frontend-submitted
	// source identifiers to prevent unbounded JSONL file growth.
	FrontendLogMaxSourceLen = 200
)

// Entry represents a single log entry in the session error log.
// The Seq field provides a monotonically increasing sequence number assigned
// at write time, enabling stable frontend deduplication without relying on
// the weak composite key (ts|level|msg|source) which collides at second precision.
type Entry struct {
	Seq uint64 `json:"seq"` // cumulative auto-incrementing counter (assigned by WriteEntry, never resets)
	// NOTE: uint64 values above 2^53-1 (Number.MAX_SAFE_INTEGER) lose precision
	// in JavaScript. Seq is a cumulative counter unrelated to buffer capacity;
	// however, even at 100 entries/sec it would take ~3000 years to reach 2^53-1,
	// so overflow is not a practical concern for long-running sessions.
	Timestamp string `json:"ts"`    // "20060102150405" format
	Level     string `json:"level"` // "error", "warn"
	Message   string `json:"msg"`
	Source    string `json:"source"` // slog group or component name
}

// ringBuffer is a fixed-capacity circular buffer for Entry values.
// It avoids O(N) slice copies on every append by overwriting the oldest entry
// when full, using a head index and a count to track the logical window.
//
// Not safe for concurrent use; callers must hold mu.
type ringBuffer struct {
	buf   []Entry // fixed-size backing array
	head  int     // index of the oldest entry (next to be overwritten when full)
	count int     // number of valid entries (0..cap)
}

// newRingBuffer allocates a ring buffer with the given capacity.
// Capacity values <= 0 are clamped to 1 to prevent modulo-by-zero panics.
func newRingBuffer(capacity int) ringBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return ringBuffer{
		buf: make([]Entry, capacity),
	}
}

// push appends an entry to the ring buffer. When full, the oldest entry is
// overwritten. Amortized O(1) with no allocation after initial construction.
func (rb *ringBuffer) push(entry Entry) {
	bufCap := len(rb.buf)
	if bufCap == 0 {
		return
	}
	if rb.count < bufCap {
		rb.buf[(rb.head+rb.count)%bufCap] = entry
		rb.count++
		return
	}
	rb.buf[rb.head] = entry
	rb.head = (rb.head + 1) % bufCap
}

// snapshot returns a newly allocated slice containing all entries in
// chronological order (oldest first). The returned slice is independent of the
// ring buffer's internal storage.
func (rb *ringBuffer) snapshot() []Entry {
	if rb.count == 0 {
		return []Entry{}
	}

	out := make([]Entry, rb.count)
	bufCap := len(rb.buf)
	first := min(bufCap-rb.head, rb.count)
	copy(out, rb.buf[rb.head:rb.head+first])

	if rest := rb.count - first; rest > 0 {
		copy(out[first:], rb.buf[:rest])
	}
	return out
}
