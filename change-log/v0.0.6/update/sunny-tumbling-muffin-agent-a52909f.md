# WebSocket Binary Streaming Migration Plan

## Summary

Migrate high-frequency pane data (`pane:data:<paneID>`) from Wails EventsEmit IPC to a
dedicated WebSocket binary streaming channel. All other events (snapshots, config, worker-panic,
etc.) remain on Wails IPC. The goal is to eliminate the JSON serialization overhead on the
hottest data path in the application.

---

## Current Architecture (Verified)

```
tmux PTY -> ReadLoop -> CommandRouter -> enqueuePaneOutput()
  -> paneFeedCh (4096 buffer) -> paneFeedWorker -> paneStates.FeedTrimmed()
  -> OutputFlushManager (16ms/32KB batching) -> emit callback
  -> runtime.EventsEmit("pane:data:"+paneID, string(flushed))
  -> Wails IPC (JSON serialization) -> Frontend
```

**Key observation**: The emit callback in `ensureOutputFlusher()` (app_events.go:166-178)
is the single point where pane data enters the Wails IPC channel. This is the exact
injection point we replace.

**Frontend reception**: `useTerminalEvents.ts` registers `EventsOn("pane:data:"+paneId, ...)`
per pane. The callback feeds data through `enqueuePendingWrite` -> RAF batching -> `term.write()`.

---

## Target Architecture

```
tmux PTY -> ReadLoop -> CommandRouter -> enqueuePaneOutput()
  -> paneFeedCh (4096 buffer) -> paneFeedWorker -> paneStates.FeedTrimmed()
  -> OutputFlushManager (16ms/32KB batching) -> emit callback
  -> wsserver.Hub.BroadcastPaneData(paneID, flushed)     <<<< CHANGED
  -> WebSocket binary frame -> Frontend

Frontend:
  paneDataStream.ts (singleton WebSocket client)
  -> dispatches binary frames to per-pane callbacks
  -> useTerminalEvents.ts subscribes via paneDataStream
```

---

## Binary Protocol Design

### Binary frames (server -> client): Pane data
```
[1 byte: paneIDLen] [N bytes: paneID (UTF-8)] [remaining bytes: terminal data]
```
- paneIDLen: uint8, max 255 bytes (pane IDs like "%0" are 2 bytes)
- paneID: raw UTF-8 string, NOT null-terminated
- data: raw terminal output bytes (no encoding)
- Total overhead: 1 + len(paneID) bytes per frame (typically 3 bytes for "%0")

### Text frames (client -> server): Control messages
```json
{"action":"subscribe","paneIds":["%0","%1"]}
{"action":"unsubscribe","paneIds":["%0"]}
{"action":"ping"}
```

### Text frames (server -> client): Control responses
```json
{"type":"pong"}
{"type":"error","message":"invalid subscribe payload"}
```

### Design Rationale
- Binary for data: zero JSON overhead on the hot path
- Text for control: human-readable, easy to debug, infrequent
- Single WebSocket: desktop app = 1 client, no need for multiplexing complexity

---

## Phase 1: Parallel Development (3 Independent Agents)

### Agent A: `internal/workerutil/` Package

**Purpose**: Extract the duplicated panic recovery loop into a reusable helper.
Currently identical loops exist in `startPaneFeedWorker` (app_pane_feed.go) and
`startIdleMonitor` (app_lifecycle.go). The WebSocket server will be a third worker
needing the same pattern.

**CROSS-REF**: The existing code has explicit comments:
> "If a third worker is added, extract this into a runWithPanicRecovery helper."
> (app_pane_feed.go:73, app_lifecycle.go:176)

#### Files to Create

1. **`myT-x/internal/workerutil/recovery.go`**

```go
package workerutil

// WorkerConfig configures the panic recovery retry loop for a background worker.
type WorkerConfig struct {
    // Name identifies the worker in logs and events.
    Name string
    // InitialBackoff is the starting delay before the first restart (e.g. 100ms).
    InitialBackoff time.Duration
    // MaxBackoff caps the exponential backoff (e.g. 5s).
    MaxBackoff time.Duration
    // MaxRetries is the total restart attempts before permanent stop.
    MaxRetries int
    // OnPanic is called after each recovered panic (for event emission).
    // May be nil.
    OnPanic func(worker string, attempt int)
    // OnFatal is called when max retries are exhausted.
    // May be nil.
    OnFatal func(worker string, maxRetries int)
}

// RunWithRecovery executes workFn in a panic recovery loop.
// It blocks until ctx is cancelled or max retries are exhausted.
// workFn should block until its work is complete or ctx is done.
func RunWithRecovery(ctx context.Context, cfg WorkerConfig, workFn func(ctx context.Context))
```

Key implementation details:
- Move `recoverBackgroundPanic`, `nextPanicRestartBackoff` logic into this package
- Keep `initialPanicRestartBackoff`, `maxPanicRestartBackoff`, `maxPanicRestartRetries`
  as package-level defaults but allow override via `WorkerConfig`
- Use `time.NewTimer` + `defer timer.Stop()` (checklist #66)
- Check `ctx.Err()` after each panic recovery before sleeping
- Log with `slog` using `[DEBUG-PANIC]` prefix for consistency

2. **`myT-x/internal/workerutil/recovery_test.go`**

Tests (table-driven, checklist #7):
- `TestRunWithRecovery_NormalExit`: workFn returns normally, no retries
- `TestRunWithRecovery_ContextCancellation`: ctx cancelled mid-work, exits cleanly
- `TestRunWithRecovery_PanicRecovery`: workFn panics N times then succeeds
- `TestRunWithRecovery_MaxRetriesExhausted`: always-panicking workFn, verify OnFatal called
- `TestRunWithRecovery_BackoffProgression`: verify exponential backoff doubles correctly
- `TestRunWithRecovery_ContextCancelDuringSleep`: cancel ctx during backoff sleep
- `TestRunWithRecovery_NilCallbacks`: OnPanic/OnFatal nil does not panic
- `TestNextBackoff`: boundary tests for backoff calculation (migrated from app_panic_recovery_test.go)

#### Applicable Defensive Coding Checklist Items
- #29: `fmt.Errorf(": %w", err)` wrapping
- #49: Goroutines with context
- #60: `defer recover()` in long-lived goroutines
- #61: Exponential backoff + retry limit
- #66: `time.NewTimer` + `defer timer.Stop()` (no `time.After` in select)
- #111: DRY - this IS the DRY extraction
- #138: `sync.Once` for double-close prevention (if needed for stop)

#### What NOT to Touch
- Do NOT modify `app_pane_feed.go` or `app_lifecycle.go` yet (that is Phase 2)
- Do NOT modify `app_panic_recovery.go` yet (Phase 2 will migrate callers then
  either keep it for backward compat or delete it)
- Do NOT touch any frontend code

#### Dependencies
- None (pure Go library package, no imports from main package)

---

### Agent B: `internal/wsserver/` Package

**Purpose**: Standalone WebSocket server that accepts a single client connection,
manages pane subscriptions, and broadcasts binary pane data frames.

#### Files to Create

1. **`myT-x/internal/wsserver/hub.go`**

```go
package wsserver

// Hub manages the WebSocket server lifecycle and client connection.
type Hub struct {
    mu          sync.RWMutex
    conn        *websocket.Conn   // single client (desktop app)
    subscribes  map[string]bool   // paneID -> subscribed
    addr        string            // resolved listen address after Start()
    listener    net.Listener
    httpServer  *http.Server
    upgrader    websocket.Upgrader
    logger      *slog.Logger
    onSubscribe func(paneIDs []string)    // optional callback
    stopped     bool
    stopOnce    sync.Once
    doneCh      chan struct{}
}

// NewHub creates a Hub. Options are functional options.
func NewHub(opts ...Option) *Hub

// Option configures Hub behavior.
type Option func(*Hub)
func WithLogger(logger *slog.Logger) Option
func WithOnSubscribe(fn func([]string)) Option

// Start starts the HTTP server on ":0" (OS-assigned port).
// Returns the resolved address (e.g., "127.0.0.1:54321").
func (h *Hub) Start(ctx context.Context) (addr string, err error)

// Stop gracefully shuts down the server.
// Safe to call multiple times (sync.Once).
func (h *Hub) Stop()

// Addr returns the resolved listen address after Start().
func (h *Hub) Addr() string

// BroadcastPaneData sends a binary frame for the given pane.
// No-op if no client is connected or pane is not subscribed.
// This is the hot-path method called from OutputFlushManager's emit callback.
func (h *Hub) BroadcastPaneData(paneID string, data []byte)

// IsConnected returns whether a client is currently connected.
func (h *Hub) IsConnected() bool
```

Key implementation details:
- Listen on `127.0.0.1:0` (loopback only, OS-assigned port)
- `websocket.Upgrader` with `CheckOrigin: func(r *http.Request) bool { return true }`
  (desktop app, internal communication, CLAUDE.md says no external vulnerability concerns)
- Single connection: if a new client connects while one is active, close the old one
- Binary frame encoding: `[1 byte paneIDLen][paneID bytes][data bytes]`
- Use `WriteMessage(websocket.BinaryMessage, frame)` with a write deadline
- Read pump: parse JSON text messages for subscribe/unsubscribe/ping
- Write pump: channel-based with select on stopCh
- Subscription tracking: `map[string]bool` protected by `sync.RWMutex`
- `BroadcastPaneData` hot path: `RLock` to check subscription + connection, only
  acquire write lock for actual send if needed (or use a buffered write channel)
- Graceful shutdown: `httpServer.Shutdown(ctx)` with timeout

**Write Strategy (critical for performance)**:
- Use a single write goroutine (writePump) with a buffered channel
- `BroadcastPaneData` encodes the binary frame and sends to the write channel
- If channel is full, log and drop (same pattern as paneFeedCh overflow)
- Write pump drains channel and calls `conn.WriteMessage`
- This avoids lock contention on the WebSocket connection from multiple OutputFlushManager flushes

2. **`myT-x/internal/wsserver/protocol.go`**

```go
package wsserver

// EncodePaneDataFrame encodes a pane data binary frame.
// Format: [1 byte: paneIDLen][N bytes: paneID][remaining: data]
func EncodePaneDataFrame(paneID string, data []byte) []byte

// DecodePaneDataFrame decodes a binary frame into paneID and data.
// Returns error if frame is too short or paneIDLen exceeds frame bounds.
func DecodePaneDataFrame(frame []byte) (paneID string, data []byte, err error)

// ControlMessage represents a client->server control message.
type ControlMessage struct {
    Action  string   `json:"action"`
    PaneIDs []string `json:"paneIds,omitempty"`
}

// ServerMessage represents a server->client control message.
type ServerMessage struct {
    Type    string `json:"type"`
    Message string `json:"message,omitempty"`
}

// ParseControlMessage parses a text WebSocket message as a control message.
func ParseControlMessage(data []byte) (ControlMessage, error)
```

3. **`myT-x/internal/wsserver/hub_test.go`**

Tests:
- `TestHubStartStop`: start, verify addr is non-empty, stop, verify idempotent stop
- `TestHubSingleClient`: connect, verify IsConnected, disconnect, verify !IsConnected
- `TestHubReplaceClient`: connect client A, connect client B, verify A is closed
- `TestHubSubscribeUnsubscribe`: send subscribe, broadcast data, verify received;
  send unsubscribe, broadcast data, verify NOT received
- `TestHubBroadcastNoClient`: BroadcastPaneData with no client does not panic
- `TestHubBroadcastUnsubscribedPane`: data for unsubscribed pane is not sent
- `TestHubPingPong`: send ping control, receive pong response
- `TestHubInvalidControlMessage`: send malformed JSON, verify error response
- `TestHubBinaryFrameIntegrity`: broadcast data, read binary frame, decode, verify match
- `TestHubWriteChannelFull`: fill write channel, verify BroadcastPaneData does not block
- `TestHubGracefulShutdown`: connect client, stop hub, verify client gets close frame

4. **`myT-x/internal/wsserver/protocol_test.go`**

Tests:
- `TestEncodePaneDataFrame`: various paneID lengths and data sizes
- `TestDecodePaneDataFrame`: round-trip encode/decode
- `TestDecodePaneDataFrame_EmptyFrame`: error case
- `TestDecodePaneDataFrame_TruncatedPaneID`: paneIDLen exceeds frame length
- `TestDecodePaneDataFrame_EmptyData`: valid frame with zero data bytes
- `TestDecodePaneDataFrame_MaxPaneIDLen`: 255-byte paneID
- `TestParseControlMessage_Subscribe`: valid subscribe
- `TestParseControlMessage_InvalidJSON`: malformed input
- `TestParseControlMessage_UnknownAction`: unknown action field

#### Applicable Defensive Coding Checklist Items
- #29: Error wrapping with `%w`
- #49: Goroutines with context
- #54: No external calls inside mutex (WebSocket write outside lock via channel)
- #60: `defer recover()` in read/write pump goroutines
- #65: `cancel() -> wg.Wait() -> cleanup` ordering in Stop()
- #66: `time.NewTimer` for write deadlines
- #111: DRY
- #136: `defer Close()` for listener, connection
- #138: `sync.Once` for Stop() idempotency

#### What NOT to Touch
- Do NOT modify any files in `main` package
- Do NOT modify frontend code
- Do NOT modify `go.mod` (gorilla/websocket is already an indirect dependency)

#### Dependencies
- `github.com/gorilla/websocket` (already in go.mod as indirect; will become direct)
- No dependency on Agent A's workerutil (Hub manages its own goroutines internally;
  the panic recovery wrapper will be applied in Phase 2 when integrating)

---

### Agent C: Frontend WebSocket Client

**Purpose**: Create a singleton WebSocket client module and refactor `useTerminalEvents.ts`
to receive pane data via WebSocket instead of Wails EventsOn.

#### Files to Create

1. **`myT-x/frontend/src/services/paneDataStream.ts`**

```typescript
// Singleton WebSocket client for binary pane data streaming.
// Replaces EventsOn("pane:data:"+paneId) for high-frequency terminal output.

type PaneDataCallback = (data: Uint8Array) => void;

interface PaneDataStream {
  /** Connect to the WebSocket server. Called once from App init. */
  connect(wsUrl: string): void;

  /** Subscribe to pane data. Returns unsubscribe function. */
  subscribe(paneId: string, callback: PaneDataCallback): () => void;

  /** Disconnect and clean up. */
  disconnect(): void;

  /** Current connection state for diagnostics. */
  readonly state: "disconnected" | "connecting" | "connected" | "reconnecting";
}
```

Key implementation details:

**Connection lifecycle**:
- `connect(wsUrl)`: create WebSocket with `binaryType = "arraybuffer"`
- On open: send subscribe message for all currently registered paneIds
- On close: start reconnection with exponential backoff (100ms -> 5s cap, max 10 retries)
- On error: log, trigger reconnection
- On message (binary): decode frame, dispatch to subscriber callback
- On message (text): parse control response, log errors

**Binary frame decoding** (must match server encoding):
```typescript
function decodePaneDataFrame(buffer: ArrayBuffer): { paneId: string; data: Uint8Array } | null {
  const view = new DataView(buffer);
  if (buffer.byteLength < 1) return null;
  const paneIdLen = view.getUint8(0);
  if (buffer.byteLength < 1 + paneIdLen) return null;
  const paneIdBytes = new Uint8Array(buffer, 1, paneIdLen);
  const paneId = new TextDecoder().decode(paneIdBytes);
  const data = new Uint8Array(buffer, 1 + paneIdLen);
  return { paneId, data };
}
```

**Subscription management**:
- `Map<string, Set<PaneDataCallback>>` for per-pane subscriber tracking
- `subscribe(paneId, cb)`: add to map, send subscribe control message if connected
- Returned unsubscribe function: remove from map, send unsubscribe if no more subscribers
- On reconnect: re-send subscribe for all panes with active subscribers

**Reconnection**:
- Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms, 3200ms, 5000ms (cap), ...
- Max 10 retries, then emit notification via callback
- Reset retry counter on successful connection + first message received
- Use `setTimeout` (not `setInterval`) for backoff
- Clean up timeout on disconnect

**UI notification on failure**:
- Accept an `onFatalDisconnect` callback in connect options
- After max retries exhausted, call this callback with error message
- The callback will use `useNotificationStore.getState().addNotification()`

**TextDecoder caching**:
- Create a single `TextDecoder` instance at module scope (reuse across frames)
- Avoids per-frame allocation

2. **Modifications to `myT-x/frontend/src/hooks/useTerminalEvents.ts`**

**Change**: Replace `EventsOn(paneEvent, ...)` with `paneDataStream.subscribe(paneId, ...)`

Current code (lines ~187-192):
```typescript
const cancelPaneEvent = EventsOn(paneEvent, (data: string) => {
    if (typeof data === "string") {
        enqueuePendingWrite(data);
    }
});
```

New code:
```typescript
const unsubscribePaneData = paneDataStream.subscribe(paneId, (data: Uint8Array) => {
    // xterm.js Terminal.write() accepts string | Uint8Array.
    // Uint8Array avoids TextDecoder overhead entirely.
    enqueuePendingWrite(data);
});
```

**Additional changes to `enqueuePendingWrite` and `flushPendingWrites`**:
- Change `pendingWrites` type from `string[]` to `(string | Uint8Array)[]`
- `term.write()` already accepts both `string` and `Uint8Array`, so no conversion needed
- The IME `composingOutput` path still uses `string[]` (human typing speed, negligible)
- For pageHidden path, `term.write(data)` works with `Uint8Array` directly

**Cleanup change**:
```typescript
// Replace: cancelPaneEvent();
// With:
unsubscribePaneData();
```

3. **Modifications to `myT-x/frontend/src/App.tsx`** (or a new init module)

- Import `paneDataStream` and call `connect()` after fetching WebSocket URL
- Add a Wails-bound API call: `api.GetWebSocketURL()` to discover the WS port
- On mount: `api.GetWebSocketURL().then(url => paneDataStream.connect(url))`
- On unmount: `paneDataStream.disconnect()`

4. **Modifications to `myT-x/frontend/src/api.ts`**

Add import and export of `GetWebSocketURL`:
```typescript
import { GetWebSocketURL } from "../wailsjs/go/main/App";
// ... in api object:
GetWebSocketURL,
```

#### Applicable Defensive Coding Checklist Items
- #87: UI notification for WebSocket connection failures (not just console.warn)
- #95: try/catch in async handlers (reconnection logic)
- #96: RAF/Timer cleanup in useEffect (setTimeout for reconnection)
- #99: No TypeScript non-null assertions (use proper null checks)

#### What NOT to Touch
- Do NOT modify `useBackendSync.ts` (non-pane events stay on Wails IPC)
- Do NOT modify `useTerminalSetup.ts` (terminal creation unchanged)
- Do NOT modify `tmuxStore.ts`
- Do NOT modify `notificationStore.ts` (only import and use it)
- Do NOT modify any Go backend files

#### Dependencies
- The `api.GetWebSocketURL()` call requires the backend method to exist (Phase 2),
  but the frontend can be built with a stub/type annotation from wailsjs binding generation
- No new npm dependencies needed (WebSocket is a browser built-in)

---

## Phase 2: Backend Integration (Sequential, depends on Phase 1)

### Agent D: Backend Wiring

**Purpose**: Wire the wsserver.Hub into the App struct, start/stop it in lifecycle,
change the OutputFlushManager emit callback, add the `GetWebSocketURL()` API method,
and migrate existing workers to use workerutil.

#### Files to Modify

1. **`myT-x/app.go`** - Add Hub field to App struct

Add to App struct:
```go
// WebSocket server for binary pane data streaming.
wsHub     *wsserver.Hub
```

Add import:
```go
"myT-x/internal/wsserver"
```

No other changes to app.go.

2. **`myT-x/app_lifecycle.go`** - Start/stop WebSocket server

**In `startup()`**, after `a.startPaneFeedWorker(ctx)`:
```go
// Start WebSocket server for binary pane data streaming.
hub := wsserver.NewHub(
    wsserver.WithLogger(slog.Default()),
)
wsAddr, wsErr := hub.Start(ctx)
if wsErr != nil {
    runtimeLogger.Errorf(ctx, "WebSocket server failed to start: %v", wsErr)
    a.addPendingConfigLoadWarning(
        "WebSocket server failed to start. Terminal output may be degraded. Error: " + wsErr.Error(),
    )
} else {
    runtimeLogger.Infof(ctx, "WebSocket server listening: %s", wsAddr)
    a.wsHub = hub
}
```

**In `shutdown()`**, after stopping pipe server but before sessions.Close():
```go
if a.wsHub != nil {
    a.wsHub.Stop()
    a.wsHub = nil
}
```

**Migrate `startIdleMonitor`** to use `workerutil.RunWithRecovery`:
```go
func (a *App) startIdleMonitor(parent context.Context) {
    sessions, err := a.requireSessions()
    if err != nil {
        return
    }
    ctx, cancel := context.WithCancel(parent)
    a.idleCancel = cancel
    a.bgWG.Add(1)
    go func() {
        defer a.bgWG.Done()
        workerutil.RunWithRecovery(ctx, workerutil.WorkerConfig{
            Name:           "idle-monitor",
            InitialBackoff: workerutil.DefaultInitialBackoff,
            MaxBackoff:     workerutil.DefaultMaxBackoff,
            MaxRetries:     workerutil.DefaultMaxRetries,
            OnPanic: func(worker string, attempt int) {
                if rtCtx := a.runtimeContext(); rtCtx != nil {
                    a.emitRuntimeEventWithContext(rtCtx, "tmux:worker-panic", map[string]any{
                        "worker": worker,
                    })
                }
            },
            OnFatal: func(worker string, maxRetries int) {
                if fatalCtx := a.runtimeContext(); fatalCtx != nil {
                    a.emitRuntimeEventWithContext(fatalCtx, "tmux:worker-fatal", map[string]any{
                        "worker":     worker,
                        "maxRetries": maxRetries,
                    })
                }
            },
        }, func(ctx context.Context) {
            // ... existing idle monitor loop body ...
        })
    }()
}
```

3. **`myT-x/app_pane_feed.go`** - Migrate to workerutil

Same pattern as idle monitor: replace the inline panic recovery loop with
`workerutil.RunWithRecovery`.

4. **`myT-x/app_events.go`** - Change emit callback in `ensureOutputFlusher()`

**Current** (line 166-178):
```go
flusher := terminal.NewOutputFlushManager(outputFlushInterval, outputFlushThreshold, func(paneID string, flushed []byte) {
    if len(flushed) == 0 {
        return
    }
    ctx := a.runtimeContext()
    if ctx == nil {
        slog.Debug("[output] skip pane flush because runtime context is nil", "paneId", paneID)
        return
    }
    if sessions := a.sessions; sessions != nil && sessions.UpdateActivityByPaneID(paneID) {
        a.requestSnapshot(false)
    }
    slog.Debug("[output] flushing to frontend", "paneId", paneID, "flushedLen", len(flushed))
    a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))
})
```

**New**:
```go
flusher := terminal.NewOutputFlushManager(outputFlushInterval, outputFlushThreshold, func(paneID string, flushed []byte) {
    if len(flushed) == 0 {
        return
    }
    ctx := a.runtimeContext()
    if ctx == nil {
        slog.Debug("[output] skip pane flush because runtime context is nil", "paneId", paneID)
        return
    }
    if sessions := a.sessions; sessions != nil && sessions.UpdateActivityByPaneID(paneID) {
        a.requestSnapshot(false)
    }
    slog.Debug("[output] flushing to frontend", "paneId", paneID, "flushedLen", len(flushed))
    // Primary path: WebSocket binary streaming (zero JSON overhead).
    if hub := a.wsHub; hub != nil && hub.IsConnected() {
        hub.BroadcastPaneData(paneID, flushed)
        return
    }
    // Fallback: Wails IPC (used during WebSocket reconnection or startup race).
    a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))
})
```

**Key design decision**: Keep the Wails IPC fallback. During the brief window between
app startup and frontend WebSocket connection, pane data still flows via EventsEmit.
This ensures no data loss during initialization. The frontend will stop listening on
`EventsOn("pane:data:...")` once WebSocket is connected, but having the backend fallback
is cheap insurance.

5. **`myT-x/app_pane_api.go`** (or new file `myT-x/app_ws_api.go`) - Add API method

```go
// GetWebSocketURL returns the WebSocket URL for binary pane data streaming.
// Returns empty string if the WebSocket server is not available.
func (a *App) GetWebSocketURL() string {
    if a.wsHub == nil {
        return ""
    }
    addr := a.wsHub.Addr()
    if addr == "" {
        return ""
    }
    return "ws://" + addr + "/ws"
}
```

This method is Wails-bound (exported on App), so the frontend can call it via
the auto-generated binding.

6. **`myT-x/go.mod`** - Promote gorilla/websocket to direct dependency

Change:
```
github.com/gorilla/websocket v1.5.3 // indirect
```
To:
```
github.com/gorilla/websocket v1.5.3
```

This happens automatically when `internal/wsserver` imports it directly.

7. **`myT-x/app_panic_recovery.go`** - Keep but deprecate

Add a comment at the top:
```go
// DEPRECATED: Use internal/workerutil.RunWithRecovery instead.
// This file is kept for backward compatibility during migration.
// Remove after all callers are migrated.
```

The constants and functions here are still used by tests. After Phase 2 migration
is complete, they can be removed or re-exported from workerutil.

#### Applicable Defensive Coding Checklist Items
- #49: WebSocket Hub goroutines use app context
- #54: No WebSocket write inside any App mutex
- #65: Hub.Stop() follows cancel -> wait -> cleanup ordering
- #111: DRY - workerutil extraction completes the DRY fix

#### What NOT to Touch
- Do NOT modify `OutputFlushManager` internals (only change the callback passed to it)
- Do NOT modify `panestate.Manager`
- Do NOT modify tmux package
- Do NOT modify frontend (that was Phase 1 Agent C)
- Do NOT modify config.go (no new config fields needed for initial implementation;
  port is auto-assigned)

---

## Phase 3: Build, Test, Self-Review

### Step 1: Build Verification
```bash
cd myT-x && go build ./...
cd myT-x/frontend && npm run build
```

### Step 2: Run All Tests
```bash
cd myT-x && go test ./...
cd myT-x && go test ./internal/workerutil/...
cd myT-x && go test ./internal/wsserver/...
```

### Step 3: Integration Smoke Test
1. Start the app with `wails dev`
2. Open a terminal pane
3. Verify WebSocket connection established (check browser DevTools Network tab)
4. Run `dir /s C:\` or similar high-output command
5. Verify terminal output streams correctly
6. Kill and restart the app, verify reconnection works
7. Verify other events (snapshots, config changes) still work via Wails IPC

### Step 4: Self-Review Checklist
- [ ] All new public functions have tests
- [ ] Error wrapping uses `fmt.Errorf(": %w", err)` consistently
- [ ] No `time.After` in select statements
- [ ] All goroutines have panic recovery
- [ ] Lock ordering documented and followed
- [ ] No external calls inside mutexes
- [ ] `sync.Once` for idempotent cleanup
- [ ] Frontend: no `!` non-null assertions
- [ ] Frontend: all async paths have try/catch
- [ ] Frontend: all timers cleaned up in useEffect cleanup
- [ ] Debug logs retained with `[DEBUG-WS]` prefix

---

## Risk Assessment & Mitigations

### Risk 1: WebSocket connection delay on startup
**Mitigation**: Fallback to Wails IPC in the emit callback. Frontend `useTerminalEvents`
keeps the `EventsOn` listener as a secondary path until WebSocket connects.
**Decision**: Actually, to keep the implementation clean, we will NOT keep dual listeners
in the frontend. Instead, the backend fallback ensures data is emitted via Wails IPC
during the brief startup window. The frontend will call `GetWebSocketURL()` early in
initialization and connect before any pane data flows. In practice, WebSocket connection
on localhost is < 1ms.

### Risk 2: WebSocket disconnection mid-session
**Mitigation**: Frontend reconnection with exponential backoff. During reconnection,
pane data from the backend is dropped (BroadcastPaneData no-ops when no client).
This is acceptable because:
- Disconnection on localhost is extremely rare (no network involved)
- Terminal replay (`GetPaneReplay`) can restore state if needed
- The paneStates manager retains all output regardless of WebSocket state

### Risk 3: Binary frame decoding mismatch
**Mitigation**: Protocol is trivial (1-byte length prefix). Both encoder and decoder
have comprehensive round-trip tests. No versioning needed for initial release.

### Risk 4: Memory pressure from write channel
**Mitigation**: Write channel has bounded capacity (e.g., 256 frames). On overflow,
frames are dropped with a debug log (same pattern as paneFeedCh overflow). At 16ms
flush interval with 10 panes, that is ~625 frames/sec worst case; a 256-frame buffer
provides ~400ms of headroom.

---

## File Summary

### New Files (7)
| File | Agent | Purpose |
|------|-------|---------|
| `myT-x/internal/workerutil/recovery.go` | A | Panic recovery helper |
| `myT-x/internal/workerutil/recovery_test.go` | A | Tests for recovery helper |
| `myT-x/internal/wsserver/hub.go` | B | WebSocket server + Hub |
| `myT-x/internal/wsserver/protocol.go` | B | Binary frame encode/decode |
| `myT-x/internal/wsserver/hub_test.go` | B | Hub integration tests |
| `myT-x/internal/wsserver/protocol_test.go` | B | Protocol unit tests |
| `myT-x/frontend/src/services/paneDataStream.ts` | C | Frontend WS client |

### Modified Files (7)
| File | Agent | Change |
|------|-------|--------|
| `myT-x/app.go` | D | Add `wsHub` field to App struct |
| `myT-x/app_lifecycle.go` | D | Start/stop Hub, migrate idle monitor to workerutil |
| `myT-x/app_pane_feed.go` | D | Migrate to workerutil.RunWithRecovery |
| `myT-x/app_events.go` | D | Change emit callback to use WebSocket primary + IPC fallback |
| `myT-x/app_panic_recovery.go` | D | Deprecation comment (keep for now) |
| `myT-x/frontend/src/hooks/useTerminalEvents.ts` | C | Replace EventsOn with paneDataStream.subscribe |
| `myT-x/frontend/src/api.ts` | C/D | Add GetWebSocketURL binding |

### Possibly New Files (1)
| File | Agent | Purpose |
|------|-------|---------|
| `myT-x/app_ws_api.go` | D | GetWebSocketURL() method (or add to app_pane_api.go) |

---

## Critical Files for Implementation

1. **`D:\myT-x\dev-myT-x\myT-x\app_events.go`** - The single injection point where
   OutputFlushManager's emit callback switches from Wails IPC to WebSocket. Lines 160-180
   contain `ensureOutputFlusher()` which constructs the callback. This is the minimal
   change that completes the entire migration.

2. **`D:\myT-x\dev-myT-x\myT-x\internal\wsserver\hub.go`** - The core new component.
   Must handle connection lifecycle, subscription tracking, binary frame encoding, and
   the write pump pattern. Performance-critical: BroadcastPaneData is called at 60Hz per pane.

3. **`D:\myT-x\dev-myT-x\myT-x\frontend\src\hooks\useTerminalEvents.ts`** - The frontend
   consumption point. Must replace `EventsOn("pane:data:"+paneId)` with the WebSocket
   subscriber while preserving all existing behavior (RAF batching, IME composition,
   visibility optimization, disposed guard).

4. **`D:\myT-x\dev-myT-x\myT-x\frontend\src\services\paneDataStream.ts`** - New singleton
   module. Must handle reconnection, subscription management, binary decoding, and
   error notification. Module-scope singleton (not React state) to avoid re-renders.

5. **`D:\myT-x\dev-myT-x\myT-x\internal\workerutil\recovery.go`** - Enables the DRY
   extraction that three call sites (pane feed worker, idle monitor, and potentially the
   WebSocket Hub's internal goroutines) all need. Prerequisite for clean Phase 2 integration.
