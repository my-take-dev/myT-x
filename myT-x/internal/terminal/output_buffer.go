package terminal

import (
	"bytes"
	"sync"
	"time"
)

var outputBufferPool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// OutputBuffer batches terminal output (16ms / 8KB default).
type OutputBuffer struct {
	mu       sync.Mutex
	buf      *bytes.Buffer
	maxBytes int
	interval time.Duration
	emit     func([]byte)

	maxBufferedAge time.Duration
	lastWriteAt    time.Time
	pendingSince   time.Time

	ticker  *time.Ticker
	stopCh  chan struct{}
	once    sync.Once
	stopped bool
}

// NewOutputBuffer creates OutputBuffer.
func NewOutputBuffer(interval time.Duration, maxBytes int, emit func([]byte)) *OutputBuffer {
	if interval <= 0 {
		interval = 16 * time.Millisecond
	}
	if maxBytes <= 0 {
		maxBytes = 8 * 1024
	}
	if emit == nil {
		emit = func([]byte) {}
	}
	maxBufferedAge := interval * 4
	if maxBufferedAge < 64*time.Millisecond {
		maxBufferedAge = 64 * time.Millisecond
	}
	buf := outputBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return &OutputBuffer{
		maxBytes:       maxBytes,
		interval:       interval,
		emit:           emit,
		maxBufferedAge: maxBufferedAge,
		buf:            buf,
		stopCh:         make(chan struct{}),
	}
}

// Start starts periodic flush loop.
func (o *OutputBuffer) Start() {
	o.mu.Lock()
	if o.ticker != nil || o.stopped {
		o.mu.Unlock()
		return
	}
	o.ticker = time.NewTicker(o.interval)
	ticker := o.ticker
	o.mu.Unlock()

	go func() {
		for {
			select {
			case <-o.stopCh:
				return
			case <-ticker.C:
				o.flushOnTick()
			}
		}
	}()
}

// Write appends bytes and flushes if size threshold is reached.
func (o *OutputBuffer) Write(data []byte) {
	if len(data) == 0 {
		return
	}
	shouldFlush := false
	now := time.Now()
	o.mu.Lock()
	if o.stopped || o.buf == nil {
		o.mu.Unlock()
		return
	}
	if o.buf.Len() == 0 {
		o.pendingSince = now
	}
	o.lastWriteAt = now
	o.buf.Write(data)
	if o.buf.Len() >= o.maxBytes {
		shouldFlush = true
	}
	o.mu.Unlock()
	if shouldFlush {
		o.Flush()
	}
}

// Flush flushes current buffer immediately.
func (o *OutputBuffer) Flush() {
	o.flush(true)
}

func (o *OutputBuffer) flushOnTick() {
	o.flush(false)
}

func (o *OutputBuffer) flush(force bool) {
	now := time.Now()
	var raw []byte
	o.mu.Lock()
	if o.buf == nil || o.buf.Len() == 0 {
		o.mu.Unlock()
		return
	}
	if !force {
		quietFor := now.Sub(o.lastWriteAt)
		pendingFor := now.Sub(o.pendingSince)
		if o.buf.Len() < o.maxBytes && quietFor < o.interval && pendingFor < o.maxBufferedAge {
			o.mu.Unlock()
			return
		}
	}
	raw = append(raw, o.buf.Bytes()...)
	o.buf.Reset()
	o.pendingSince = time.Time{}
	o.mu.Unlock()
	o.emit(raw)
}

// Stop stops loop and flushes pending data.
func (o *OutputBuffer) Stop() {
	o.once.Do(func() {
		close(o.stopCh)
	})

	var pending []byte
	var buf *bytes.Buffer
	o.mu.Lock()
	if o.stopped {
		o.mu.Unlock()
		return
	}
	o.stopped = true
	if o.ticker != nil {
		o.ticker.Stop()
		o.ticker = nil
	}
	if o.buf != nil {
		pending = append(pending, o.buf.Bytes()...)
		o.buf.Reset()
		o.pendingSince = time.Time{}
		buf = o.buf
		o.buf = nil
	}
	o.mu.Unlock()

	if len(pending) > 0 {
		o.emit(pending)
	}
	if buf != nil {
		outputBufferPool.Put(buf)
	}
}
