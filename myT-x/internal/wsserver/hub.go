package wsserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// writeDeadline is the maximum time allowed for a single WebSocket write to
// complete. 5 seconds is generous for localhost single-client writes; if a
// WebView freezes longer than this, the connection is considered dead.
const writeDeadline = 5 * time.Second

// readDeadline is the maximum time the server waits for any read activity
// (including pong responses) before considering the connection dead.
// 90 seconds allows for ~3 missed pings (pingInterval=30s) before timeout.
const readDeadline = 90 * time.Second

// pingInterval is the interval between server-initiated WebSocket pings.
// 30 seconds is a standard keepalive interval that detects dead connections
// within ~90 seconds (readDeadline = 3 * pingInterval).
const pingInterval = 30 * time.Second

// maxReadMessageSize limits the maximum size of incoming WebSocket messages.
// 32 KiB is sufficient for subscribe/unsubscribe JSON payloads which are
// typically under 1 KiB. This prevents OOM from malformed or oversized messages.
const maxReadMessageSize = 32 * 1024

// wsUpgrader is a package-level Upgrader to avoid repeated allocation on each
// connection upgrade (S-22). The Upgrader is stateless and safe for reuse.
var wsUpgrader = websocket.Upgrader{
	// CheckOrigin allows all origins because the server binds to 127.0.0.1
	// only. Localhost-only binding prevents external access; origin check
	// is redundant for desktop apps but kept permissive for WebView
	// compatibility.
	CheckOrigin:    func(r *http.Request) bool { return true },
	ReadBufferSize: 1024,
	// WriteBufferSize 32 KiB: matches OutputFlushManager's maxBytes threshold
	// so that typical flush payloads fit in a single websocket frame buffer.
	WriteBufferSize: 32 * 1024,
}

// HubOptions configures the WebSocket server.
// Struct with a single field is intentional for future extensibility
// (e.g., TLS config, buffer sizes, timeouts).
type HubOptions struct {
	// Addr is the listen address. Use "127.0.0.1:0" for OS-assigned port.
	// 127.0.0.1 binding restricts access to localhost only, which is safe for
	// a desktop application where frontend and backend run on the same machine.
	Addr string
}

// Hub manages a single WebSocket connection for streaming pane terminal output
// from the Go backend to the React frontend via binary frames.
//
// Design: Single-connection model (desktop app = 1 WebView client).
// New connections replace existing ones to handle page reloads gracefully.
//
// Lock ordering (never acquire in reverse):
//
//	writeMu -> mu
//
// mu protects connection state and subscription map.
// writeMu serializes gorilla/websocket WriteMessage calls (not concurrency-safe).
//
// Write failure policy: Any write failure (BroadcastPaneData, sendError, pingLoop)
// disconnects the client via clearIfCurrent+closeConn. The client must reconnect.
type Hub struct {
	opts HubOptions

	// mu protects conn and subscribed. See lock ordering comment on Hub.
	mu         sync.RWMutex
	conn       *websocket.Conn
	subscribed map[string]bool // paneID -> subscribed

	// writeMu serializes WriteMessage calls. gorilla/websocket does not support
	// concurrent writes; all callers of WriteMessage must hold this lock.
	// Independent of mu: never hold mu when acquiring writeMu (lock ordering).
	writeMu sync.Mutex

	listener net.Listener
	server   *http.Server
	url      string // "ws://127.0.0.1:<port>/ws", set after Start

	// closeOnce ensures Stop is idempotent. Once Stop has been called,
	// the Hub cannot be reused; create a new Hub instance instead.
	closeOnce sync.Once
}

// subscribeAction and unsubscribeAction are the valid values for subscribeMsg.Action.
const (
	subscribeAction   = "subscribe"
	unsubscribeAction = "unsubscribe"
)

// subscribeMsg is the JSON payload for client subscribe/unsubscribe requests.
// Action must be subscribeAction or unsubscribeAction; PaneIDs lists affected pane identifiers.
type subscribeMsg struct {
	Action  string   `json:"action"`
	PaneIDs []string `json:"paneIds"`
}

// errorMsg is the JSON payload for server error notifications sent to the client.
type errorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewHub creates a Hub with the given options.
// The hub is not started until Start is called.
func NewHub(opts HubOptions) *Hub {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:0"
	}
	return &Hub{
		opts:       opts,
		subscribed: make(map[string]bool),
	}
}

// Start begins listening on the configured address and serves WebSocket
// connections. The context is used for the server's BaseContext: when ctx is
// cancelled, active request handlers receive cancellation. The server itself
// must be stopped explicitly via Stop.
//
// Returns an error if the listener cannot be created (e.g. port in use).
//
// Thread safety: Start must be called exactly once during application startup
// (before any concurrent access). It is not safe to call from multiple goroutines.
func (h *Hub) Start(ctx context.Context) error {
	if h.server != nil {
		return fmt.Errorf("wsserver: already started")
	}

	ln, err := net.Listen("tcp", h.opts.Addr)
	if err != nil {
		return fmt.Errorf("wsserver: listen: %w", err)
	}
	h.listener = ln

	port := ln.Addr().(*net.TCPAddr).Port
	h.url = fmt.Sprintf("ws://127.0.0.1:%d/ws", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.handleWS)

	h.server = &http.Server{
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		if serveErr := h.server.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("[DEBUG-WS] server error", "error", serveErr)
		}
	}()

	slog.Info("[DEBUG-WS] server started", "url", h.url)
	return nil
}

// Stop gracefully shuts down the HTTP server and closes any active WebSocket
// connection. Safe to call multiple times (idempotent via sync.Once).
func (h *Hub) Stop() error {
	var stopErr error
	h.closeOnce.Do(func() {
		// Close active connection first.
		h.mu.Lock()
		conn := h.conn
		h.conn = nil
		h.subscribed = make(map[string]bool)
		h.mu.Unlock()

		if conn != nil {
			if err := conn.Close(); err != nil {
				slog.Debug("[DEBUG-WS] connection close during stop", "error", err)
			}
		}

		// Shutdown HTTP server with timeout.
		if h.server != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.server.Shutdown(shutdownCtx); err != nil {
				stopErr = fmt.Errorf("wsserver: shutdown: %w", err)
			}
		}

		slog.Info("[DEBUG-WS] server stopped")
	})
	return stopErr
}

// URL returns the WebSocket URL for frontend connection
// (e.g. "ws://127.0.0.1:54321/ws").
// Returns empty string if the server has not started.
func (h *Hub) URL() string {
	return h.url
}

// HasActiveConnection reports whether a WebSocket client is currently connected.
// Used by the emit callback to decide between WebSocket and Wails IPC fallback.
func (h *Hub) HasActiveConnection() bool {
	h.mu.RLock()
	active := h.conn != nil
	h.mu.RUnlock()
	return active
}

// clearIfCurrent clears the hub's connection and subscription state only if the
// provided conn is still the current connection. Returns true if it was cleared.
// Caller must NOT hold h.mu (this method acquires it).
func (h *Hub) clearIfCurrent(conn *websocket.Conn) bool {
	h.mu.Lock()
	isCurrent := h.conn == conn
	if isCurrent {
		h.conn = nil
		h.subscribed = make(map[string]bool)
	}
	h.mu.Unlock()
	return isCurrent
}

// closeConn closes a WebSocket connection. The close may fail if the connection
// was already closed by another goroutine (e.g. page reload replacing the old
// connection) -- this is expected and logged at Debug level.
// NOTE: Double-close is safe here; gorilla/websocket.Close returns an error on
// already-closed connections but has no other side effects (S-5).
func (h *Hub) closeConn(conn *websocket.Conn, reason string) {
	if closeErr := conn.Close(); closeErr != nil {
		slog.Debug("[DEBUG-WS] connection close", "reason", reason, "error", closeErr)
	}
}

// setWriteDeadlineOrClose sets a write deadline on the connection. If setting
// the deadline fails, the connection is in an indeterminate state and must be
// closed to prevent indefinite blocking (#4, I-6).
// Returns false if the deadline could not be set (connection was closed).
func (h *Hub) setWriteDeadlineOrClose(conn *websocket.Conn, d time.Duration) bool {
	if err := conn.SetWriteDeadline(time.Now().Add(d)); err != nil {
		slog.Warn("[DEBUG-WS] SetWriteDeadline failed, closing connection", "error", err)
		h.clearIfCurrent(conn)
		// Close outside mu to prevent deadlock (#54).
		h.closeConn(conn, "SetWriteDeadline failure")
		return false
	}
	return true
}

// clearWriteDeadline resets the write deadline after a successful write.
// Failure to clear is non-fatal: the next write will set a fresh deadline.
// NOTE: Error is logged but does not close the connection because the write
// itself succeeded and the next setWriteDeadlineOrClose will handle it (#9).
func (h *Hub) clearWriteDeadline(conn *websocket.Conn) {
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		slog.Debug("[DEBUG-WS] clearWriteDeadline failed (non-fatal)", "error", err)
	}
}

// BroadcastPaneData sends a binary-encoded pane data frame to the connected
// client, but only if the client has subscribed to the given pane ID.
//
// Called from OutputFlushManager's flush goroutine at ~60Hz per active pane.
// Thread-safe: uses writeMu to serialize writes as required by gorilla/websocket.
//
// If no client is connected, the pane is not subscribed, or data is empty,
// the call is a no-op.
// Write errors close the connection and log the error (write failure policy).
func (h *Hub) BroadcastPaneData(paneID string, data []byte) {
	if len(data) == 0 {
		return
	}

	h.mu.RLock()
	conn := h.conn
	subscribed := h.subscribed[paneID]
	h.mu.RUnlock()

	// NOTE (TOCTOU window): Between RUnlock and writeMu.Lock, the connection
	// may be replaced by a new client (e.g., page reload). This is acceptable
	// because: (1) writeMessage on a closed conn returns an error which triggers
	// clearIfCurrent, and (2) clearIfCurrent checks pointer identity so it won't
	// clear a newer connection. The worst case is one failed write attempt on
	// a stale connection, which is harmless.

	if conn == nil {
		// NOTE: Logged at Debug to avoid flooding when no client is connected.
		// This path is hit at high frequency (~60Hz * active panes) (S-2).
		slog.Debug("[DEBUG-WS] broadcast skipped: no connection", "paneId", paneID)
		return
	}
	if !subscribed {
		// Not subscribed: silent skip. This is called at high frequency per pane,
		// so logging here would be excessive.
		return
	}

	frame, encErr := EncodePaneData(paneID, data)
	if encErr != nil {
		slog.Warn("[DEBUG-WS] failed to encode pane data", "error", encErr, "paneID", paneID)
		return
	}

	h.writeMu.Lock()
	if !h.setWriteDeadlineOrClose(conn, writeDeadline) {
		h.writeMu.Unlock()
		return
	}
	err := conn.WriteMessage(websocket.BinaryMessage, frame)
	h.clearWriteDeadline(conn)
	h.writeMu.Unlock()

	if err != nil {
		slog.Warn("[DEBUG-WS] write failed, closing connection", "paneId", paneID, "error", err)
		h.clearIfCurrent(conn)
		// Close outside mu lock to prevent deadlock (#54).
		h.closeConn(conn, "write error in BroadcastPaneData")
	}
}

// handleWS upgrades HTTP to WebSocket and runs the read pump for the connection.
// Only one connection is active at a time; new connections replace old ones
// to handle page reloads gracefully.
func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("[DEBUG-WS] upgrade failed", "error", err)
		return
	}

	// I-14: Limit incoming message size to prevent OOM from oversized payloads.
	conn.SetReadLimit(maxReadMessageSize)

	// I-13: Configure read deadline and pong handler for dead connection detection.
	// The read deadline is extended on every pong received from the client.
	if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		slog.Warn("[DEBUG-WS] SetReadDeadline failed on new connection", "error", err)
		h.closeConn(conn, "initial SetReadDeadline failure")
		return
	}
	conn.SetPongHandler(func(string) error {
		// Extend read deadline on each pong received from the client.
		return conn.SetReadDeadline(time.Now().Add(readDeadline))
	})

	// Replace existing connection (page reload scenario).
	h.mu.Lock()
	oldConn := h.conn
	h.conn = conn
	h.subscribed = make(map[string]bool)
	h.mu.Unlock()

	if oldConn != nil {
		// Close old connection outside lock to prevent deadlock (#54).
		h.closeConn(oldConn, "replaced by new connection")
	}

	slog.Info("[DEBUG-WS] client connected", "remoteAddr", conn.RemoteAddr())

	// Start ping sender goroutine for dead connection detection (I-13).
	pingDone := make(chan struct{})
	go h.pingLoop(conn, pingDone)

	// readPump: handle subscribe/unsubscribe JSON messages from the client.
	defer func() {
		// Panic recovery for connection handler (#60).
		if rec := recover(); rec != nil {
			slog.Error("[DEBUG-PANIC] wsserver handleWS recovered",
				"panic", rec,
				"stack", string(debug.Stack()), // S-20: include stack trace
			)
		}

		// Signal ping goroutine to stop.
		close(pingDone)

		h.clearIfCurrent(conn)

		// NOTE: conn.Close() may be called multiple times here if the connection
		// was already closed by BroadcastPaneData or Stop. gorilla/websocket's
		// Close is safe to call on already-closed connections (S-5).
		h.closeConn(conn, "read pump exit")
		slog.Info("[DEBUG-WS] client disconnected")
	}()

	for {
		msgType, msg, readErr := conn.ReadMessage()
		if readErr != nil {
			if websocket.IsUnexpectedCloseError(readErr, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("[DEBUG-WS] read error", "error", readErr)
			}
			return
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var subMsg subscribeMsg
		if jsonErr := json.Unmarshal(msg, &subMsg); jsonErr != nil {
			slog.Debug("[DEBUG-WS] invalid JSON from client", "error", jsonErr)
			h.sendError(conn, fmt.Sprintf("invalid JSON: %s", jsonErr))
			continue
		}
		h.handleSubscription(conn, subMsg)
	}
}

// pingLoop sends periodic WebSocket pings to detect dead connections (I-13).
// Runs as a goroutine per connection; exits when done is closed or ping fails.
func (h *Hub) pingLoop(conn *websocket.Conn, done <-chan struct{}) {
	defer func() {
		// Panic recovery for long-lived goroutine (#60).
		// On panic, clean up the connection so it doesn't remain open without
		// pings, which would prevent dead connection detection.
		if rec := recover(); rec != nil {
			slog.Error("[DEBUG-PANIC] wsserver pingLoop recovered",
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			h.clearIfCurrent(conn)
			h.closeConn(conn, "pingLoop panic recovery")
		}
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			h.writeMu.Lock()
			if !h.setWriteDeadlineOrClose(conn, writeDeadline) {
				h.writeMu.Unlock()
				return
			}
			pingErr := conn.WriteMessage(websocket.PingMessage, nil)
			h.clearWriteDeadline(conn)
			h.writeMu.Unlock()

			if pingErr != nil {
				slog.Debug("[DEBUG-WS] ping failed, connection likely dead", "error", pingErr)
				h.clearIfCurrent(conn)
				h.closeConn(conn, "ping failure")
				return
			}
		}
	}
}

// handleSubscription applies a subscribe or unsubscribe action to the
// connection's pane subscription set.
func (h *Hub) handleSubscription(conn *websocket.Conn, msg subscribeMsg) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// S-4: Only process subscriptions for the current connection. If a page
	// reload replaced this connection, discard stale messages.
	if h.conn != conn {
		slog.Debug("[DEBUG-WS] subscription from stale connection, skipping")
		return
	}

	switch msg.Action {
	case subscribeAction:
		for _, id := range msg.PaneIDs {
			if id == "" {
				// Frontend bug: empty pane IDs should not be sent.
				// Logged at Debug to avoid log flooding on high-frequency paths.
				slog.Debug("[DEBUG-WS] empty paneId in subscribe request, skipping")
				continue
			}
			h.subscribed[id] = true
			slog.Debug("[DEBUG-WS] subscribed", "paneId", id)
		}
	case unsubscribeAction:
		for _, id := range msg.PaneIDs {
			if id == "" {
				// Frontend bug: empty pane IDs should not be sent.
				// Logged at Debug to avoid log flooding on high-frequency paths.
				slog.Debug("[DEBUG-WS] empty paneId in unsubscribe request, skipping")
				continue
			}
			delete(h.subscribed, id)
			slog.Debug("[DEBUG-WS] unsubscribed", "paneId", id)
		}
	default:
		slog.Debug("[DEBUG-WS] unknown action", "action", msg.Action)
	}
}

// sendError sends a JSON error message to the client. On write failure,
// the connection is cleaned up per write failure policy (see Hub doc).
func (h *Hub) sendError(conn *websocket.Conn, message string) {
	errPayload := errorMsg{
		Type:    "error",
		Message: message,
	}
	payload, err := json.Marshal(errPayload)
	if err != nil {
		slog.Debug("[DEBUG-WS] failed to marshal error message", "error", err)
		return
	}

	h.writeMu.Lock()
	// I-1: Set write deadline to prevent indefinite blocking on frozen WebView.
	if !h.setWriteDeadlineOrClose(conn, writeDeadline) {
		h.writeMu.Unlock()
		return
	}
	writeErr := conn.WriteMessage(websocket.TextMessage, payload)
	h.clearWriteDeadline(conn)
	h.writeMu.Unlock()

	if writeErr != nil {
		slog.Debug("[DEBUG-WS] failed to send error to client", "error", writeErr)
		h.clearIfCurrent(conn)
		h.closeConn(conn, "write error in sendError")
	}
}
