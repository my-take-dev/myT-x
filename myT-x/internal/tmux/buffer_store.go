package tmux

import (
	"fmt"
	"sync"
	"time"
)

// maxBufferCount limits the number of paste buffers to prevent unbounded memory growth.
const maxBufferCount = 50

// PasteBuffer represents a single tmux paste buffer.
type PasteBuffer struct {
	Name      string
	Data      []byte
	CreatedAt time.Time
}

// BufferStore manages a global stack of paste buffers, matching tmux behavior.
// Buffers are ordered newest-first. Thread-safe via internal RWMutex.
type BufferStore struct {
	mu      sync.RWMutex
	buffers []*PasteBuffer
	named   map[string]int // name → index in buffers slice
	nextID  int
}

// NewBufferStore creates an empty buffer store.
func NewBufferStore() *BufferStore {
	return &BufferStore{
		named: make(map[string]int),
	}
}

// Set creates or updates a buffer. If name is empty, an auto-generated name is used.
// When appendMode is true, data is appended to an existing buffer (created if not found).
// Oldest buffers are evicted when maxBufferCount is exceeded.
func (bs *BufferStore) Set(name string, data []byte, appendMode bool) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if name == "" {
		name = fmt.Sprintf("buffer%04d", bs.nextID)
		bs.nextID++
	}

	if idx, ok := bs.named[name]; ok && idx < len(bs.buffers) {
		buf := bs.buffers[idx]
		if appendMode {
			buf.Data = append(buf.Data, data...)
		} else {
			buf.Data = make([]byte, len(data))
			copy(buf.Data, data)
		}
		buf.CreatedAt = time.Now()
		return
	}

	newBuf := &PasteBuffer{
		Name:      name,
		Data:      make([]byte, len(data)),
		CreatedAt: time.Now(),
	}
	copy(newBuf.Data, data)

	// Prepend (newest first).
	bs.buffers = append([]*PasteBuffer{newBuf}, bs.buffers...)
	bs.rebuildIndex()

	// Evict oldest if over limit.
	if len(bs.buffers) > maxBufferCount {
		evicted := bs.buffers[maxBufferCount:]
		bs.buffers = bs.buffers[:maxBufferCount]
		for _, buf := range evicted {
			delete(bs.named, buf.Name)
		}
	}
}

// Get retrieves a buffer by name. Returns nil, false if not found.
func (bs *BufferStore) Get(name string) (*PasteBuffer, bool) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	idx, ok := bs.named[name]
	if !ok || idx >= len(bs.buffers) {
		return nil, false
	}
	buf := bs.buffers[idx]
	return &PasteBuffer{
		Name:      buf.Name,
		Data:      append([]byte(nil), buf.Data...),
		CreatedAt: buf.CreatedAt,
	}, true
}

// Latest returns the most recent buffer (top of stack). Returns nil, false if empty.
func (bs *BufferStore) Latest() (*PasteBuffer, bool) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	if len(bs.buffers) == 0 {
		return nil, false
	}
	buf := bs.buffers[0]
	return &PasteBuffer{
		Name:      buf.Name,
		Data:      append([]byte(nil), buf.Data...),
		CreatedAt: buf.CreatedAt,
	}, true
}

// Delete removes a buffer by name. Returns true if the buffer existed.
func (bs *BufferStore) Delete(name string) bool {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	idx, ok := bs.named[name]
	if !ok || idx >= len(bs.buffers) {
		return false
	}
	bs.buffers = append(bs.buffers[:idx], bs.buffers[idx+1:]...)
	bs.rebuildIndex()
	return true
}

// List returns a snapshot of all buffers (newest first).
// Returned buffers are copies safe for use outside the lock.
func (bs *BufferStore) List() []*PasteBuffer {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	result := make([]*PasteBuffer, len(bs.buffers))
	for i, buf := range bs.buffers {
		result[i] = &PasteBuffer{
			Name:      buf.Name,
			Data:      append([]byte(nil), buf.Data...),
			CreatedAt: buf.CreatedAt,
		}
	}
	return result
}

// Rename changes a buffer's name. Returns error if old name not found or new name already exists.
func (bs *BufferStore) Rename(oldName, newName string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	idx, ok := bs.named[oldName]
	if !ok || idx >= len(bs.buffers) {
		return fmt.Errorf("buffer not found: %s", oldName)
	}
	if _, exists := bs.named[newName]; exists {
		return fmt.Errorf("buffer already exists: %s", newName)
	}
	bs.buffers[idx].Name = newName
	delete(bs.named, oldName)
	bs.named[newName] = idx
	return nil
}

// rebuildIndex reconstructs the named index from the buffers slice.
// Must be called under write lock.
func (bs *BufferStore) rebuildIndex() {
	clear(bs.named)
	for i, buf := range bs.buffers {
		bs.named[buf.Name] = i
	}
}
