package terminal

import (
	"bytes"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type paneOutputChunk struct {
	paneID string
	data   []byte
}

const (
	// tuiRedrawMinWriteSize filters out small TUI control sequences (8, 19 bytes).
	tuiRedrawMinWriteSize = 32

	// tuiRedrawMinSamples prevents false positives from short bursts.
	tuiRedrawMinSamples = 10

	// tuiRedrawConcentrationThreshold: top-2 sizes must account for this
	// fraction of large writes. Tool approval=~1.0, AI response=~0.35.
	tuiRedrawConcentrationThreshold = 0.80

	// tuiRedrawANSIThreshold requires redraw candidates to mostly contain ANSI
	// escape sequences so plain-text output chunks are not mistaken for TUI redraws.
	tuiRedrawANSIThreshold = 0.80
)

type paneOutputState struct {
	buf          *bytes.Buffer
	lastWriteAt  time.Time
	pendingSince time.Time

	// TUI redraw pattern detection: frequency of large write sizes.
	largeSizeFreq  map[int]int
	largeSizeTotal int
	largeANSIWrites int
}

// OutputFlushManager batches pane output with a single background worker.
// It replaces per-pane ticker goroutines with one shared loop.
type OutputFlushManager struct {
	mu sync.Mutex

	interval       time.Duration
	maxBytes       int
	maxBufferedAge time.Duration
	emit           func(string, []byte)

	panes map[string]*paneOutputState

	started  bool
	stopped  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	wakeCh   chan struct{}
	stopOnce sync.Once
}

// NewOutputFlushManager creates a shared output flusher.
func NewOutputFlushManager(interval time.Duration, maxBytes int, emit func(string, []byte)) *OutputFlushManager {
	if interval <= 0 {
		interval = 16 * time.Millisecond
	}
	if maxBytes <= 0 {
		// Default 32 KiB: see outputFlushThreshold rationale in app_events.go.
		maxBytes = 32 * 1024
	}
	if emit == nil {
		emit = func(string, []byte) {}
	}
	// maxBufferedAge caps the maximum time data sits unbatched. 64ms minimum
	// ensures at least ~15 fps equivalent flush rate even under low throughput.
	// nextInterval() backs off the ticker when traffic is low (flushed <= 2
	// per interval), doubling the interval to reduce wakeups during idle periods.
	maxBufferedAge := max(interval*4, 64*time.Millisecond)
	return &OutputFlushManager{
		interval:       interval,
		maxBytes:       maxBytes,
		maxBufferedAge: maxBufferedAge,
		emit:           emit,
		panes:          map[string]*paneOutputState{},
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
		wakeCh:         make(chan struct{}, 1),
	}
}

// Start starts the shared flush loop.
func (m *OutputFlushManager) Start() {
	m.mu.Lock()
	if m.started || m.stopped {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	go m.loop()
}

func (m *OutputFlushManager) loop() {
	defer close(m.doneCh)

	currentInterval := m.interval
	timer := time.NewTimer(currentInterval)
	defer timer.Stop()

	for {
		select {
		case <-m.stopCh:
			m.flushAll()
			return
		case <-m.wakeCh:
			flushed := m.flushReady(true)
			currentInterval = m.nextInterval(flushed)
			resetTimer(timer, currentInterval)
		case <-timer.C:
			flushed := m.flushReady(false)
			currentInterval = m.nextInterval(flushed)
			timer.Reset(currentInterval)
		}
	}
}

func resetTimer(timer *time.Timer, interval time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

func (m *OutputFlushManager) nextInterval(flushed int) time.Duration {
	if flushed <= 2 {
		return m.interval * 2
	}
	return m.interval
}

// IsPaneQuiet returns true when the pane has had no output for at least the
// given threshold duration, indicating it is likely waiting for user input.
// Returns true if the pane has never produced output (no state tracked).
func (m *OutputFlushManager) IsPaneQuiet(paneID string, threshold time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.panes[paneID]
	if state == nil {
		slog.Debug("[DEBUG-FLUSH] IsPaneQuiet: no state for pane, treating as quiet", "paneID", paneID)
		return true // no output tracked → treat as quiet
	}

	// Primary: time since last write.
	if time.Since(state.lastWriteAt) >= threshold {
		m.resetRedrawCountersLocked(state)
		return true
	}

	// Secondary: TUI redraw pattern detection.
	// If recent output consists almost entirely of 1-2 repeating large-write
	// sizes, the pane is likely in a TUI redraw loop (e.g., tool approval UI).
	if m.isTUIRedrawPatternLocked(state) {
		concentration := m.topTwoConcentrationLocked(state)
		ansiRatio := m.ansiWriteConcentrationLocked(state)
		slog.Debug("[DEBUG-QUIET] TUI redraw pattern detected, treating as quiet",
			"paneID", paneID,
			"largeSizeTotal", state.largeSizeTotal,
			"largeANSIWrites", state.largeANSIWrites,
			"distinctSizes", len(state.largeSizeFreq),
			"topTwoConcentration", fmt.Sprintf("%.1f%%", concentration*100),
			"ansiRatio", fmt.Sprintf("%.1f%%", ansiRatio*100),
			"top10", m.topNSizesLocked(state, 10),
		)
		m.resetRedrawCountersLocked(state)
		return true
	}

	// Log pattern stats when pane is busy — helps tune concentration threshold.
	if state.largeSizeTotal >= tuiRedrawMinSamples {
		concentration := m.topTwoConcentrationLocked(state)
		ansiRatio := m.ansiWriteConcentrationLocked(state)
		slog.Debug("[DEBUG-QUIET] pane busy pattern stats",
			"paneID", paneID,
			"largeSizeTotal", state.largeSizeTotal,
			"largeANSIWrites", state.largeANSIWrites,
			"distinctSizes", len(state.largeSizeFreq),
			"topTwoConcentration", fmt.Sprintf("%.1f%%", concentration*100),
			"ansiRatio", fmt.Sprintf("%.1f%%", ansiRatio*100),
			"top10", m.topNSizesLocked(state, 10),
		)
	}

	// Reset counters so each check window analyzes only the latest interval.
	m.resetRedrawCountersLocked(state)

	return false
}

// isTUIRedrawPatternLocked checks if the accumulated write-size distribution
// matches a TUI redraw pattern: top-2 most frequent large-write sizes account
// for >= tuiRedrawConcentrationThreshold of all large writes.
func (m *OutputFlushManager) isTUIRedrawPatternLocked(state *paneOutputState) bool {
	if state.largeSizeTotal < tuiRedrawMinSamples {
		return false
	}
	return m.topTwoConcentrationLocked(state) >= tuiRedrawConcentrationThreshold &&
		m.ansiWriteConcentrationLocked(state) >= tuiRedrawANSIThreshold
}

// topTwoConcentrationLocked returns the fraction of large writes accounted
// for by the two most frequent write sizes.
func (m *OutputFlushManager) topTwoConcentrationLocked(state *paneOutputState) float64 {
	if state.largeSizeTotal == 0 {
		return 0
	}
	var top1, top2 int
	for _, count := range state.largeSizeFreq {
		if count > top1 {
			top2 = top1
			top1 = count
		} else if count > top2 {
			top2 = count
		}
	}
	return float64(top1+top2) / float64(state.largeSizeTotal)
}

func (m *OutputFlushManager) ansiWriteConcentrationLocked(state *paneOutputState) float64 {
	if state.largeSizeTotal == 0 {
		return 0
	}
	return float64(state.largeANSIWrites) / float64(state.largeSizeTotal)
}

// topNSizesLocked returns a string showing the top-N most frequent write sizes
// and their counts, sorted by count descending. For diagnostic logging.
func (m *OutputFlushManager) topNSizesLocked(state *paneOutputState, n int) string {
	type sizeCount struct {
		size  int
		count int
	}

	entries := make([]sizeCount, 0, len(state.largeSizeFreq))
	for size, count := range state.largeSizeFreq {
		entries = append(entries, sizeCount{size, count})
	}

	// Simple insertion sort for small N — avoid sort package import.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].count > entries[j-1].count; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	if len(entries) > n {
		entries = entries[:n]
	}

	var b bytes.Buffer
	for i, e := range entries {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%dB=%d", e.size, e.count)
	}
	return b.String()
}

// resetRedrawCountersLocked clears the TUI redraw pattern counters.
func (m *OutputFlushManager) resetRedrawCountersLocked(state *paneOutputState) {
	state.largeSizeFreq = nil
	state.largeSizeTotal = 0
	state.largeANSIWrites = 0
}

// Write appends output for one pane.
func (m *OutputFlushManager) Write(paneID string, data []byte) {
	if paneID == "" || len(data) == 0 {
		return
	}
	shouldWake := false
	now := time.Now()

	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	state := m.panes[paneID]
	if state == nil {
		buf := outputBufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		state = &paneOutputState{buf: buf}
		m.panes[paneID] = state
	}
	if state.buf.Len() == 0 {
		state.pendingSince = now
	}
	state.lastWriteAt = now

	// Track large-write size for TUI redraw pattern detection.
	if len(data) >= tuiRedrawMinWriteSize {
		if state.largeSizeFreq == nil {
			state.largeSizeFreq = make(map[int]int)
		}
		state.largeSizeFreq[len(data)]++
		state.largeSizeTotal++
		if bytes.IndexByte(data, 0x1b) >= 0 {
			state.largeANSIWrites++
		}
	}

	_, _ = state.buf.Write(data)
	if state.buf.Len() >= m.maxBytes {
		shouldWake = true
	}
	m.mu.Unlock()

	if shouldWake {
		select {
		case m.wakeCh <- struct{}{}:
		default:
		}
	}
}

// RetainPanes removes buffers not present in existing and flushes pending data.
func (m *OutputFlushManager) RetainPanes(existing map[string]struct{}) []string {
	if len(existing) == 0 {
		return m.detachAll()
	}

	removed := make([]string, 0)
	chunks := make([]paneOutputChunk, 0)

	m.mu.Lock()
	for paneID, state := range m.panes {
		if _, ok := existing[paneID]; ok {
			continue
		}
		removed = append(removed, paneID)
		if state != nil {
			if chunk, ok := m.flushStateLocked(paneID, state); ok {
				chunks = append(chunks, chunk)
			}
			m.releaseStateLocked(state)
		}
		delete(m.panes, paneID)
	}
	m.mu.Unlock()

	m.emitChunks(chunks)
	return removed
}

// RemovePane removes one pane buffer and flushes pending data.
func (m *OutputFlushManager) RemovePane(paneID string) {
	if paneID == "" {
		return
	}
	var chunk paneOutputChunk
	var hasChunk bool

	m.mu.Lock()
	state := m.panes[paneID]
	if state != nil {
		chunk, hasChunk = m.flushStateLocked(paneID, state)
		m.releaseStateLocked(state)
		delete(m.panes, paneID)
	}
	m.mu.Unlock()

	if hasChunk {
		m.emit(chunk.paneID, chunk.data)
	}
}

func (m *OutputFlushManager) detachAll() []string {
	removed := make([]string, 0)
	chunks := make([]paneOutputChunk, 0)

	m.mu.Lock()
	for paneID, state := range m.panes {
		removed = append(removed, paneID)
		if state != nil {
			if chunk, ok := m.flushStateLocked(paneID, state); ok {
				chunks = append(chunks, chunk)
			}
			m.releaseStateLocked(state)
		}
		delete(m.panes, paneID)
	}
	m.mu.Unlock()

	m.emitChunks(chunks)
	return removed
}

func (m *OutputFlushManager) flushReady(forceLargeOnly bool) int {
	now := time.Now()
	chunks := make([]paneOutputChunk, 0)

	m.mu.Lock()
	for paneID, state := range m.panes {
		if state == nil {
			continue
		}
		if chunk, ok := m.shouldFlushStateLocked(paneID, state, now, forceLargeOnly); ok {
			chunks = append(chunks, chunk)
		}
	}
	m.mu.Unlock()

	m.emitChunks(chunks)
	return len(chunks)
}

func (m *OutputFlushManager) flushAll() {
	chunks := make([]paneOutputChunk, 0)

	m.mu.Lock()
	for paneID, state := range m.panes {
		if state == nil {
			continue
		}
		if chunk, ok := m.flushStateLocked(paneID, state); ok {
			chunks = append(chunks, chunk)
		}
		m.releaseStateLocked(state)
		delete(m.panes, paneID)
	}
	m.mu.Unlock()
	m.emitChunks(chunks)
}

func (m *OutputFlushManager) shouldFlushStateLocked(
	paneID string,
	state *paneOutputState,
	now time.Time,
	forceLargeOnly bool,
) (paneOutputChunk, bool) {
	if state.buf == nil || state.buf.Len() == 0 {
		return paneOutputChunk{}, false
	}
	if forceLargeOnly {
		if state.buf.Len() < m.maxBytes {
			return paneOutputChunk{}, false
		}
		return m.flushStateLocked(paneID, state)
	}

	quietFor := now.Sub(state.lastWriteAt)
	pendingFor := now.Sub(state.pendingSince)
	if state.buf.Len() < m.maxBytes && quietFor < m.interval && pendingFor < m.maxBufferedAge {
		return paneOutputChunk{}, false
	}
	return m.flushStateLocked(paneID, state)
}

func (m *OutputFlushManager) flushStateLocked(
	paneID string,
	state *paneOutputState,
) (paneOutputChunk, bool) {
	if state == nil || state.buf == nil || state.buf.Len() == 0 {
		return paneOutputChunk{}, false
	}
	data := append([]byte(nil), state.buf.Bytes()...)
	state.buf.Reset()
	state.pendingSince = time.Time{}
	return paneOutputChunk{paneID: paneID, data: data}, len(data) > 0
}

func (m *OutputFlushManager) releaseStateLocked(state *paneOutputState) {
	if state == nil || state.buf == nil {
		return
	}
	state.buf.Reset()
	outputBufferPool.Put(state.buf)
	state.buf = nil
}

func (m *OutputFlushManager) emitChunks(chunks []paneOutputChunk) {
	for _, chunk := range chunks {
		if len(chunk.data) == 0 {
			continue
		}
		m.emit(chunk.paneID, chunk.data)
	}
}

// Stop stops the manager and flushes pending data.
func (m *OutputFlushManager) Stop() {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		m.stopped = true
		started := m.started
		m.mu.Unlock()

		if !started {
			m.flushAll()
			close(m.doneCh)
			return
		}
		close(m.stopCh)
		<-m.doneCh
	})
}
