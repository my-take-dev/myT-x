package main

import (
	"context"
	"log/slog"
	"sync"

	"myT-x/internal/workerutil"
)

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

func (a *App) startPaneFeedWorker(parent context.Context) {
	if a.paneStates == nil || a.paneFeedCh == nil {
		return
	}

	ctx, cancel := context.WithCancel(parent)
	a.paneFeedStop = cancel
	ch := a.paneFeedCh
	paneStates := a.paneStates

	workerutil.RunWithPanicRecovery(ctx, "pane-feed-worker", &a.bgWG, func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case item := <-ch:
				// NOTE: If FeedTrimmed panics, putFeedBuffer will not be called
				// and the pool buffer will leak (one per panic). GC will eventually
				// collect it, so this is acceptable.
				paneStates.FeedTrimmed(item.paneID, item.chunk)
				putFeedBuffer(item.poolPtr)
			}
		}
	}, a.defaultRecoveryOptions())
}

func (a *App) stopPaneFeedWorker() {
	if a.paneFeedStop != nil {
		a.paneFeedStop()
		a.paneFeedStop = nil
	}
	// Best-effort drain so pooled buffers are returned promptly after stop.
	for {
		select {
		case item := <-a.paneFeedCh:
			putFeedBuffer(item.poolPtr)
		default:
			return
		}
	}
}

func (a *App) enqueuePaneStateFeed(paneID string, chunk []byte) {
	if a.paneStates == nil || len(chunk) == 0 {
		return
	}
	pooled, bp := getFeedBuffer(len(chunk))
	copy(pooled, chunk)
	item := paneFeedItem{
		paneID:  paneID,
		chunk:   pooled,
		poolPtr: bp,
	}
	// NOTE: When the channel is full, the direct-feed fallback bypasses the worker,
	// so ordering between the fallback chunk and the worker's next dequeue is not
	// guaranteed. This is acceptable because xterm.js buffers and renders output
	// asynchronously; minor reordering within a single flush cycle is invisible
	// to the user and does not corrupt terminal state.
	select {
	case a.paneFeedCh <- item:
	default:
		slog.Debug("[DEBUG-FEED] channel full, falling back to direct feed", "paneId", paneID, "chunkLen", len(chunk))
		a.paneStates.FeedTrimmed(paneID, pooled)
		putFeedBuffer(bp)
	}
}
