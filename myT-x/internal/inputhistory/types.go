package inputhistory

import (
	"os"
	"time"
)

const (
	// Dir is the subdirectory under the config directory where JSONL history files are stored.
	Dir = "input-history"

	// MaxFiles caps the number of retained history files.
	// 50 files at ~10K entries each covers weeks of typical usage.
	// Used by the legacy per-process path only; scoped history uses RetentionDays.
	MaxFiles = 50

	// LoadWindowDays is the number of local calendar days loaded per scope.
	// History remains on disk for RetentionDays, but the UI loads only this window.
	LoadWindowDays = 7

	// RetentionDays is the number of local calendar days retained on disk.
	RetentionDays = 30

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

	// cleanupDelay keeps startup cleanup out of the startup and write paths.
	cleanupDelay = 10 * time.Second

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
	Seq       uint64 `json:"seq"`     // auto-incremented per storage scope
	Timestamp string `json:"ts"`      // format: "20060102150405"
	PaneID    string `json:"pane_id"` // tmux pane identifier (e.g. "%1")
	Input     string `json:"input"`   // cleaned user input text
	Source    string `json:"source"`  // input source (e.g. "keyboard", "sync-input")
	Session   string `json:"session"` // session identifier
}

// Snapshot represents input history loaded for one session-info storage scope.
// Field order matches the Wails-generated frontend model.
type Snapshot struct {
	ScopeKey string  `json:"scope_key"`
	Entries  []Entry `json:"entries"`
}

type serviceOptions struct {
	resolveSessionWorkDir func(sessionName string) (string, error)
	configDir             func() (string, error)
	now                   func() time.Time
	cleanupDelay          time.Duration
}

// Option customizes input history service dependencies.
type Option func(*serviceOptions)

// WithSessionScopeResolver configures session-name to workDir scoped storage.
func WithSessionScopeResolver(
	resolveSessionWorkDir func(sessionName string) (string, error),
	configDir func() (string, error),
) Option {
	return func(opts *serviceOptions) {
		opts.resolveSessionWorkDir = resolveSessionWorkDir
		opts.configDir = configDir
	}
}

// WithClock overrides time for tests.
func WithClock(now func() time.Time) Option {
	return func(opts *serviceOptions) {
		opts.now = now
	}
}

// WithCleanupDelay overrides the delayed startup cleanup interval for tests.
func WithCleanupDelay(delay time.Duration) Option {
	return func(opts *serviceOptions) {
		opts.cleanupDelay = delay
	}
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

type scopeState struct {
	key      string
	dir      string
	file     *os.File
	path     string
	fileDate string
	entries  ringBuffer
	loaded   bool
	seq      uint64
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
