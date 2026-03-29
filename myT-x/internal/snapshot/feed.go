package snapshot

// feed.go — feedBytePool and paneFeedItem for zero-alloc PTY chunk recycling.

import "sync"

// paneFeedItem carries a pane output chunk through the feed channel.
type paneFeedItem struct {
	paneID  string
	chunk   []byte
	poolPtr *[]byte // original pool pointer for zero-alloc return
}

// feedBytePool recycles byte slices used to copy PTY output chunks,
// reducing GC pressure on the high-frequency feed path.
var feedBytePool = sync.Pool{
	New: func() any {
		// 8 KiB initial capacity: aligns with typical high-speed model chunk sizes,
		// reducing grow-copy cycles on the hot feed path.
		buf := make([]byte, 0, 8192)
		return &buf
	},
}

// maxPoolBufSize is the upper bound for buffers returned to feedBytePool.
// 128 KiB upper bound: accommodates occasional large chunks from high-throughput
// models without polluting the pool with rare oversized allocations.
const maxPoolBufSize = 128 * 1024

// getFeedBuffer retrieves a byte slice from feedBytePool, growing it if needed.
// Returns (usable slice, pool pointer). The pool pointer is kept in sync with
// the returned slice internally, so the caller does not need to update it.
// The pool pointer must be passed to putFeedBuffer after use to recycle the buffer.
func getFeedBuffer(size int) ([]byte, *[]byte) {
	bp := feedBytePool.Get().(*[]byte)
	buf := *bp
	if cap(buf) < size {
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}
	*bp = buf // keep pool pointer in sync with possibly-grown slice
	return buf, bp
}

// putFeedBuffer returns a buffer to feedBytePool for reuse.
// The caller must not use the buffer after calling this function.
// Buffers exceeding maxPoolBufSize are discarded to avoid pool pollution.
func putFeedBuffer(bp *[]byte) {
	if bp == nil {
		return
	}
	if cap(*bp) > maxPoolBufSize {
		return
	}
	*bp = (*bp)[:0]
	feedBytePool.Put(bp)
}
