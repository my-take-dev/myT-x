package main

// SessionLogEntry represents a single log entry in the session error log.
// The Seq field provides a monotonically increasing sequence number assigned
// at write time, enabling stable frontend deduplication without relying on
// the weak composite key (ts|level|msg|source) which collides at second precision.
type SessionLogEntry struct {
	Seq uint64 `json:"seq"` // cumulative auto-incrementing counter (assigned by writeSessionLogEntry, never resets)
	// NOTE: uint64 values above 2^53-1 (Number.MAX_SAFE_INTEGER) lose precision
	// in JavaScript. Seq is a cumulative counter unrelated to buffer capacity;
	// however, even at 100 entries/sec it would take ~3000 years to reach 2^53-1,
	// so overflow is not a practical concern for long-running sessions.
	Timestamp string `json:"ts"`    // "20060102150405" format
	Level     string `json:"level"` // "error", "warn"
	Message   string `json:"msg"`
	Source    string `json:"source"` // slog group or component name
}

// sessionLogRingBuffer is a fixed-capacity circular buffer for SessionLogEntry.
// It avoids O(N) slice copies on every append by overwriting the oldest entry
// when full, using a head index and a count to track the logical window.
//
// Not safe for concurrent use; callers must hold sessionLogMu.
type sessionLogRingBuffer struct {
	buf   []SessionLogEntry // fixed-size backing array
	head  int               // index of the oldest entry (next to be overwritten when full)
	count int               // number of valid entries (0..cap)
}

// newSessionLogRingBuffer allocates a ring buffer with the given capacity.
// Capacity values <= 0 are clamped to 1 to prevent modulo-by-zero panics.
func newSessionLogRingBuffer(capacity int) sessionLogRingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return sessionLogRingBuffer{
		buf: make([]SessionLogEntry, capacity),
	}
}

// push appends an entry to the ring buffer. When full, the oldest entry is
// overwritten. Amortized O(1) with no allocation after initial construction.
func (rb *sessionLogRingBuffer) push(entry SessionLogEntry) {
	bufCap := len(rb.buf)
	if bufCap == 0 {
		return
	}
	if rb.count < bufCap {
		// Buffer has room: write at (head + count) mod bufCap.
		rb.buf[(rb.head+rb.count)%bufCap] = entry
		rb.count++
	} else {
		// Buffer full: overwrite the oldest entry at head, advance head.
		rb.buf[rb.head] = entry
		rb.head = (rb.head + 1) % bufCap
	}
}

// snapshot returns a newly allocated slice containing all entries in
// chronological order (oldest first). The returned slice is independent of the
// ring buffer's internal storage.
func (rb *sessionLogRingBuffer) snapshot() []SessionLogEntry {
	if rb.count == 0 {
		return []SessionLogEntry{}
	}

	out := make([]SessionLogEntry, rb.count)
	bufCap := len(rb.buf)

	// Number of entries from head to end of backing array (first segment).
	first := min(bufCap-rb.head, rb.count)
	copy(out, rb.buf[rb.head:rb.head+first])

	// Remaining entries wrap around to the beginning of the backing array.
	if rest := rb.count - first; rest > 0 {
		copy(out[first:], rb.buf[:rest])
	}
	return out
}

// len returns the number of valid entries currently stored.
func (rb *sessionLogRingBuffer) len() int {
	return rb.count
}
