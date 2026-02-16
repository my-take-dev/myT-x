package terminal

import (
	"bytes"
	"sync"
	"time"
)

type paneOutputChunk struct {
	paneID string
	data   []byte
}

type paneOutputState struct {
	buf          *bytes.Buffer
	lastWriteAt  time.Time
	pendingSince time.Time
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
		maxBytes = 8 * 1024
	}
	if emit == nil {
		emit = func(string, []byte) {}
	}
	maxBufferedAge := interval * 4
	if maxBufferedAge < 64*time.Millisecond {
		maxBufferedAge = 64 * time.Millisecond
	}
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
