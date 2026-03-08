package tmux

import (
	"log/slog"
	"sync"
)

const defaultPaneOutputHistoryCapacity = 256 * 1024 // 256 KiB

// PaneOutputHistory is a thread-safe ring buffer that accumulates
// terminal output from a pane's ReadLoop. Used by capture-pane to
// retrieve recent output.
type PaneOutputHistory struct {
	mu       sync.Mutex
	buf      []byte
	writePos int
	size     int
	capacity int
}

// NewPaneOutputHistory creates a ring buffer with the given byte capacity.
func NewPaneOutputHistory(capacity int) *PaneOutputHistory {
	return &PaneOutputHistory{
		buf:      make([]byte, capacity),
		capacity: capacity,
	}
}

// Write appends data to the ring buffer. Called from the ReadLoop callback.
// If data exceeds remaining capacity, oldest bytes are overwritten.
func (h *PaneOutputHistory) Write(data []byte) {
	if len(data) == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.capacity <= 0 || len(h.buf) == 0 {
		slog.Debug("[DEBUG-BUFFER] pane output history write ignored after release", "dataLen", len(data))
		return
	}

	// If data is larger than capacity, only keep the tail
	if len(data) >= h.capacity {
		copy(h.buf, data[len(data)-h.capacity:])
		h.writePos = 0
		h.size = h.capacity
		return
	}

	// Write data, wrapping around if needed
	n := copy(h.buf[h.writePos:], data)
	if n < len(data) {
		copy(h.buf, data[n:])
	}
	h.writePos = (h.writePos + len(data)) % h.capacity
	h.size += len(data)
	if h.size > h.capacity {
		h.size = h.capacity
	}
}

// Capture returns all buffered output in chronological order.
// Returns a new slice (safe to retain).
func (h *PaneOutputHistory) Capture() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.size == 0 || h.capacity <= 0 || len(h.buf) == 0 {
		if h.capacity <= 0 || len(h.buf) == 0 {
			slog.Debug("[DEBUG-BUFFER] pane output history capture ignored after release")
		}
		return nil
	}

	result := make([]byte, h.size)
	if h.size < h.capacity {
		// Buffer hasn't wrapped yet: data is [0..writePos)
		copy(result, h.buf[:h.size])
	} else {
		// Buffer has wrapped: data is [writePos..capacity) + [0..writePos)
		firstLen := h.capacity - h.writePos
		copy(result, h.buf[h.writePos:])
		copy(result[firstLen:], h.buf[:h.writePos])
	}
	return result
}

// Reset clears all buffered output.
func (h *PaneOutputHistory) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.writePos = 0
	h.size = 0
}

// Release drops the backing buffer so detached read-loop closures cannot retain
// the ring buffer allocation after a pane has been removed.
func (h *PaneOutputHistory) Release() {
	h.mu.Lock()
	defer h.mu.Unlock()
	clear(h.buf)
	h.buf = nil
	h.writePos = 0
	h.size = 0
	h.capacity = 0
}
