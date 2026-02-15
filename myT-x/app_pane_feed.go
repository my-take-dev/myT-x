package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// feedBytePool recycles byte slices used to copy PTY output chunks,
// reducing GC pressure on the high-frequency feed path.
var feedBytePool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

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

// maxPoolBufSize is the upper bound for buffers returned to feedBytePool.
// Buffers larger than this are discarded to prevent pool pollution from rare large chunks.
const maxPoolBufSize = 64 * 1024

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

	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		restartDelay := initialPanicRestartBackoff
		for {
			panicked := false
			func() {
				defer func() {
					if recoverBackgroundPanic("pane-feed-worker", recover()) {
						panicked = true
					}
				}()
				for {
					select {
					case <-ctx.Done():
						return
					case item := <-ch:
						paneStates.Feed(item.paneID, item.chunk)
						putFeedBuffer(item.poolPtr)
					}
				}
			}()
			if !panicked || ctx.Err() != nil {
				return
			}
			slog.Warn("[DEBUG-PANIC] restarting worker after panic",
				"worker", "pane-feed-worker",
				"restartDelay", restartDelay,
			)
			a.emitRuntimeEventWithContext(a.runtimeContext(), "tmux:worker-panic", map[string]any{
				"worker": "pane-feed-worker",
			})
			select {
			case <-ctx.Done():
				return
			case <-time.After(restartDelay):
			}
			restartDelay = nextPanicRestartBackoff(restartDelay)
		}
	}()
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
	select {
	case a.paneFeedCh <- item:
	default:
		slog.Debug("[DEBUG-feed] channel full, falling back to direct feed", "paneId", paneID, "chunkLen", len(chunk))
		a.paneStates.Feed(paneID, pooled)
		putFeedBuffer(bp)
	}
}
