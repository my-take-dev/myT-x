package inputhistory

import "time"

const (
	// Dir is the subdirectory under the config directory where JSONL history files are stored.
	Dir = "input-history"

	// MaxFiles caps the number of retained history files.
	// 50 files at ~10K entries each covers weeks of typical usage.
	MaxFiles = 50

	// maxEntries is the in-memory ring buffer capacity.
	// 10000 entries provides a long scrollback while bounding memory to ~4 MB.
	maxEntries = 10000

	// emitMinInterval throttles "app:input-history-updated" ping events
	// to prevent Wails IPC saturation under rapid input.
	emitMinInterval = 50 * time.Millisecond

	// lineFlushTimeout is the inactivity timeout for flushing an incomplete
	// line buffer. If no Enter is received within this duration, the partial
	// buffer is written as-is (e.g. for password prompts or interactive mode).
	lineFlushTimeout = 5 * time.Second

	// MaxInputLen caps the rune count of a single history entry.
	// 4000 runes accommodates long paste operations while preventing unbounded growth.
	MaxInputLen = 4000

	// ShutdownFlushSentinel is the generation value used for forced shutdown
	// flushes. Using ^uint64(0) (math.MaxUint64) avoids collision with normal
	// timer generations, which increment from 0 upward and cannot realistically
	// reach this value within any process lifetime.
	ShutdownFlushSentinel = ^uint64(0)
)

// Entry represents a single input history record.
type Entry struct {
	Seq       uint64 `json:"seq"`     // auto-incremented per WriteEntry call
	Timestamp string `json:"ts"`      // format: "20060102150405"
	PaneID    string `json:"pane_id"` // tmux pane identifier (e.g. "%1")
	Input     string `json:"input"`   // cleaned user input text
	Source    string `json:"source"`  // input source (e.g. "keyboard", "sync-input")
	Session   string `json:"session"` // session identifier
}

// lineBuffer accumulates keystrokes for a single pane until Enter is received.
// Not safe for concurrent use; callers must hold lineBufMu.
type lineBuffer struct {
	buf      []rune
	timer    *time.Timer
	timerGen uint64
	paneID   string
	source   string
	session  string
}

// stopTimer stops and nils the flush timer. Safe to call when timer is nil.
func (lb *lineBuffer) stopTimer() {
	if lb.timer != nil {
		lb.timer.Stop()
		lb.timer = nil
	}
}

// ringBuffer is a fixed-capacity circular buffer for Entry values.
// Not safe for concurrent use; callers must hold mu.
type ringBuffer struct {
	buf   []Entry
	head  int
	count int
}

func newRingBuffer(capacity int) ringBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return ringBuffer{buf: make([]Entry, capacity)}
}

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
