package wsserver

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testListenAddr is the address used for all test hubs. Port 0 lets the OS
// assign an ephemeral port, avoiding cross-test port conflicts.
const testListenAddr = "127.0.0.1:0"

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// waitForCondition polls fn every 10ms until it returns true or the timeout
// expires. Returns true if the condition was met, false on timeout.
// This replaces the flaky time.Sleep-based waitForSubscription (I-10).
func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) bool {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case <-ticker.C:
			if fn() {
				return true
			}
		case <-deadline.C:
			return false
		}
	}
}

// waitForConnection polls until the hub has an active connection or times out.
func waitForConnection(t *testing.T, hub *Hub) {
	t.Helper()
	if !waitForCondition(t, 2*time.Second, func() bool {
		return hub.HasActiveConnection()
	}) {
		t.Fatal("timed out waiting for hub to register connection")
	}
}

// waitForNoConnection polls until the hub has no active connection or times out.
func waitForNoConnection(t *testing.T, hub *Hub) {
	t.Helper()
	if !waitForCondition(t, 2*time.Second, func() bool {
		return !hub.HasActiveConnection()
	}) {
		t.Fatal("timed out waiting for hub to clear connection")
	}
}

// waitForSubscribed polls until the hub's subscribed map contains the given
// paneID or the timeout expires.
func waitForSubscribed(t *testing.T, hub *Hub, paneID string) {
	t.Helper()
	if !waitForCondition(t, 2*time.Second, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		return hub.subscribed[paneID]
	}) {
		t.Fatalf("timed out waiting for subscription to paneID %q", paneID)
	}
}

// waitForUnsubscribed polls until the hub's subscribed map no longer contains
// the given paneID or the timeout expires.
func waitForUnsubscribed(t *testing.T, hub *Hub, paneID string) {
	t.Helper()
	if !waitForCondition(t, 2*time.Second, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		return !hub.subscribed[paneID]
	}) {
		t.Fatalf("timed out waiting for unsubscription of paneID %q", paneID)
	}
}

// dialHub is a test helper that dials the Hub's WebSocket endpoint.
// It returns the connection and a cleanup function.
func dialHub(t *testing.T, hub *Hub) *websocket.Conn {
	t.Helper()

	u, err := url.Parse(hub.URL())
	if err != nil {
		t.Fatalf("failed to parse hub URL %q: %v", hub.URL(), err)
	}

	conn, _, dialErr := websocket.DefaultDialer.Dial(u.String(), nil)
	if dialErr != nil {
		t.Fatalf("failed to dial hub: %v", dialErr)
	}
	return conn
}

// sendSubscribe sends a subscribe message to the hub through the WebSocket connection.
func sendSubscribe(t *testing.T, conn *websocket.Conn, paneIDs []string) {
	t.Helper()
	msg := subscribeMsg{Action: "subscribe", PaneIDs: paneIDs}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal subscribe message: %v", err)
	}
	if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
		t.Fatalf("failed to write subscribe message: %v", writeErr)
	}
}

// sendUnsubscribe sends an unsubscribe message to the hub through the WebSocket connection.
func sendUnsubscribe(t *testing.T, conn *websocket.Conn, paneIDs []string) {
	t.Helper()
	msg := subscribeMsg{Action: "unsubscribe", PaneIDs: paneIDs}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal unsubscribe message: %v", err)
	}
	if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
		t.Fatalf("failed to write unsubscribe message: %v", writeErr)
	}
}

// sendRawText sends a raw text message to the hub (for testing invalid JSON, etc.).
func sendRawText(t *testing.T, conn *websocket.Conn, text string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
		t.Fatalf("failed to write raw text message: %v", err)
	}
}

// readErrorResponse reads a JSON error response from the connection.
// Returns the parsed errorMsg or fails the test.
func readErrorResponse(t *testing.T, conn *websocket.Conn) errorMsg {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Fatalf("expected TextMessage (%d), got %d", websocket.TextMessage, msgType)
	}
	var errResp errorMsg
	if jsonErr := json.Unmarshal(msg, &errResp); jsonErr != nil {
		t.Fatalf("failed to unmarshal error response %q: %v", msg, jsonErr)
	}
	return errResp
}

// startHub creates and starts a Hub for testing, registering cleanup.
func startHub(t *testing.T) *Hub {
	t.Helper()
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		if err := hub.Stop(); err != nil {
			t.Errorf("hub.Stop() returned error: %v", err)
		}
		cancel()
	})
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("hub.Start() returned error: %v", err)
	}
	return hub
}

// ---------------------------------------------------------------------------
// Lifecycle tests (#117: goroutine lifecycle - start, stop, context cancel)
// ---------------------------------------------------------------------------

func TestStartAndStop(t *testing.T) {
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()

	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	if hub.URL() == "" {
		t.Fatal("URL() returned empty string after Start()")
	}

	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}
}

func TestStartDoubleCallReturnsError(t *testing.T) {
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()

	if err := hub.Start(ctx); err != nil {
		t.Fatalf("first Start() returned error: %v", err)
	}
	defer func() {
		if err := hub.Stop(); err != nil {
			t.Errorf("Stop() returned error: %v", err)
		}
	}()

	// Second call must return an error immediately without leaking resources.
	if err := hub.Start(ctx); err == nil {
		t.Fatal("second Start() should return an error, got nil")
	}
}

func TestStopIdempotent(t *testing.T) {
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()

	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// Stop twice: must not panic and must not return error (#125 idempotency).
	if err := hub.Stop(); err != nil {
		t.Fatalf("first Stop() returned error: %v", err)
	}
	if err := hub.Stop(); err != nil {
		t.Fatalf("second Stop() returned error: %v", err)
	}
}

func TestContextCancelShutdown(t *testing.T) {
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx, cancel := context.WithCancel(context.Background())

	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	conn := dialHub(t, hub)
	waitForConnection(t, hub)

	// Cancel the context; the server's BaseContext propagates cancellation.
	cancel()

	// Hub.Stop must still work cleanly after context cancellation.
	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() after cancel returned error: %v", err)
	}

	// Connection should be closed by the hub.
	if setErr := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); setErr != nil {
		t.Fatalf("SetReadDeadline failed: %v", setErr)
	}
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Fatal("expected read to fail after context cancel + stop, but succeeded")
	}

	if closeErr := conn.Close(); closeErr != nil {
		t.Logf("conn.Close() error (expected): %v", closeErr)
	}
}

// ---------------------------------------------------------------------------
// Connection tests
// ---------------------------------------------------------------------------

func TestConnectAndSubscribe(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	testData := []byte("terminal output data")
	hub.BroadcastPaneData("%0", testData)

	// Read the binary frame from the client side.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected BinaryMessage (%d), got %d", websocket.BinaryMessage, msgType)
	}

	// Decode and verify the frame contents.
	gotPaneID, gotData, decodeErr := DecodePaneData(msg)
	if decodeErr != nil {
		t.Fatalf("DecodePaneData returned error: %v", decodeErr)
	}
	if gotPaneID != "%0" {
		t.Errorf("paneID = %q, want %q", gotPaneID, "%0")
	}
	if string(gotData) != string(testData) {
		t.Errorf("data = %q, want %q", gotData, testData)
	}
}

func TestHasActiveConnection(t *testing.T) {
	hub := startHub(t)

	// Before connection: no active connection.
	if hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = true before any connection")
	}

	conn := dialHub(t, hub)
	waitForConnection(t, hub)

	// After connection: active.
	if !hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = false after connecting")
	}

	// Close connection.
	if err := conn.Close(); err != nil {
		t.Logf("conn.Close() error: %v", err)
	}

	// Wait for the hub to detect the disconnection via the read pump.
	waitForNoConnection(t, hub)

	// After disconnect: no active connection (#118: verify final state).
	if hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = true after disconnection")
	}
}

func TestConnectionReplacement(t *testing.T) {
	hub := startHub(t)

	// First connection.
	conn1 := dialHub(t, hub)
	waitForConnection(t, hub)

	// Second connection replaces the first.
	conn2 := dialHub(t, hub)
	// Wait for hub to register conn2 (it replaces conn1).
	waitForCondition(t, 2*time.Second, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		return hub.conn != nil && hub.conn != conn1
	})

	// conn1 should be closed by the hub. Reading from it should fail.
	if err := conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline on conn1 failed: %v", err)
	}
	_, _, err := conn1.ReadMessage()
	if err == nil {
		t.Fatal("expected conn1 to be closed by hub, but read succeeded")
	}
	// Close conn1 from our side (already closed by hub, ignore error).
	if closeErr := conn1.Close(); closeErr != nil {
		t.Logf("conn1.Close() error (expected): %v", closeErr)
	}

	// conn2 should work. Subscribe and receive data.
	sendSubscribe(t, conn2, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	hub.BroadcastPaneData("%0", []byte("for conn2"))

	if err := conn2.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline on conn2 failed: %v", err)
	}
	msgType, msg, readErr := conn2.ReadMessage()
	if readErr != nil {
		t.Fatalf("conn2 ReadMessage returned error: %v", readErr)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected BinaryMessage, got %d", msgType)
	}

	gotPaneID, gotData, decodeErr := DecodePaneData(msg)
	if decodeErr != nil {
		t.Fatalf("DecodePaneData returned error: %v", decodeErr)
	}
	if gotPaneID != "%0" {
		t.Errorf("paneID = %q, want %q", gotPaneID, "%0")
	}
	if string(gotData) != "for conn2" {
		t.Errorf("data = %q, want %q", gotData, "for conn2")
	}

	if closeErr := conn2.Close(); closeErr != nil {
		t.Logf("conn2.Close() error: %v", closeErr)
	}
}

// ---------------------------------------------------------------------------
// Subscription tests
// ---------------------------------------------------------------------------

func TestSubscribeUnsubscribe(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	sendUnsubscribe(t, conn, []string{"%0"})
	waitForUnsubscribed(t, hub, "%0")

	hub.BroadcastPaneData("%0", []byte("should not arrive after unsubscribe"))

	// Set a short read deadline; we expect no message.
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected read timeout (no message), but got a message")
	}
}

func TestSubscribeMultiplePanes(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	// Subscribe to multiple panes in a single message.
	sendSubscribe(t, conn, []string{"%0", "%1", "%2"})
	waitForSubscribed(t, hub, "%0")
	waitForSubscribed(t, hub, "%1")
	waitForSubscribed(t, hub, "%2")

	// Broadcast to each pane and verify each arrives.
	for _, paneID := range []string{"%0", "%1", "%2"} {
		hub.BroadcastPaneData(paneID, []byte("data-"+paneID))
	}

	received := make(map[string]string)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	for i := range 3 {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage[%d] returned error: %v", i, err)
		}
		gotPaneID, gotData, decErr := DecodePaneData(msg)
		if decErr != nil {
			t.Fatalf("DecodePaneData[%d] returned error: %v", i, decErr)
		}
		received[gotPaneID] = string(gotData)
	}

	for _, paneID := range []string{"%0", "%1", "%2"} {
		want := "data-" + paneID
		if got := received[paneID]; got != want {
			t.Errorf("pane %q: data = %q, want %q", paneID, got, want)
		}
	}
}

func TestSubscribeEmptyPaneIDs(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	// Subscribe with empty paneIDs list: must not panic (#119 empty input).
	sendSubscribe(t, conn, []string{})
	waitForConnection(t, hub)

	// Verify subscribed map is empty.
	hub.mu.RLock()
	subCount := len(hub.subscribed)
	hub.mu.RUnlock()
	if subCount != 0 {
		t.Errorf("subscribed count = %d after empty subscribe, want 0", subCount)
	}
}

// ---------------------------------------------------------------------------
// Broadcast tests
// ---------------------------------------------------------------------------

func TestBroadcastWithoutConnection(t *testing.T) {
	hub := startHub(t)

	// BroadcastPaneData with no connected client must not panic (#118).
	hub.BroadcastPaneData("%0", []byte("data"))
}

func TestBroadcastUnsubscribedPane(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	// Subscribe to %0, but broadcast to %1.
	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	hub.BroadcastPaneData("%1", []byte("should not arrive"))

	// Set a short read deadline; we expect no message.
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected read timeout (no message), but got a message")
	}
}

func TestBroadcastEmptyData(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	// Broadcast empty data: BroadcastPaneData treats len(data)==0 as a no-op,
	// so no message should arrive. Must not panic (#119 boundary).
	hub.BroadcastPaneData("%0", []byte{})

	// Set a short read deadline; we expect no message because empty data is skipped.
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected read timeout (no message for empty data broadcast), but got a message")
	}
}

func TestBroadcastPaneDataWriteError(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)

	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	// Verify connection is active before the error.
	if !hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = false before write error test")
	}

	// Close via websocket protocol and verify cleanup.
	if err := conn.Close(); err != nil {
		t.Logf("conn.Close() error: %v", err)
	}

	// Wait for the hub's read pump to detect the disconnection.
	waitForNoConnection(t, hub)

	// BroadcastPaneData on a closed connection should handle the error gracefully
	// and clean up the connection state without panicking.
	hub.BroadcastPaneData("%0", []byte("data after disconnect"))

	// After write error cleanup, HasActiveConnection should be false (#118).
	if hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = true after connection closed and BroadcastPaneData write error")
	}
}

// ---------------------------------------------------------------------------
// I-8: sendError / invalid JSON / unknown action tests (#113 table-driven)
// ---------------------------------------------------------------------------

func TestInvalidMessages(t *testing.T) {
	tests := []struct {
		name       string
		rawMessage string
		wantType   string // expected "type" in error response
		wantSubstr string // substring expected in error "message"
	}{
		{
			name:       "invalid JSON - garbage",
			rawMessage: `{not valid json`,
			wantType:   "error",
			wantSubstr: "invalid JSON",
		},
		{
			name:       "invalid JSON - empty string",
			rawMessage: ``,
			wantType:   "error",
			wantSubstr: "invalid JSON",
		},
		{
			name:       "invalid JSON - plain text",
			rawMessage: `hello world`,
			wantType:   "error",
			wantSubstr: "invalid JSON",
		},
		{
			name:       "invalid JSON - array instead of object",
			rawMessage: `[1,2,3]`,
			wantType:   "error",
			wantSubstr: "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := startHub(t)
			conn := dialHub(t, hub)
			defer func() {
				if err := conn.Close(); err != nil {
					t.Logf("conn.Close() error: %v", err)
				}
			}()

			waitForConnection(t, hub)

			sendRawText(t, conn, tt.rawMessage)

			// The hub should send an error response.
			errResp := readErrorResponse(t, conn)
			if errResp.Type != tt.wantType {
				t.Errorf("error type = %q, want %q", errResp.Type, tt.wantType)
			}
			if tt.wantSubstr != "" && !strings.Contains(errResp.Message, tt.wantSubstr) {
				t.Errorf("error message = %q, want substring %q", errResp.Message, tt.wantSubstr)
			}

			// Connection should still be alive after error response (non-fatal).
			// Verify by subscribing successfully after the error.
			sendSubscribe(t, conn, []string{"%0"})
			waitForSubscribed(t, hub, "%0")
		})
	}
}

func TestUnknownAction(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{name: "unknown action - random", action: "foobar"},
		{name: "unknown action - empty", action: ""},
		{name: "unknown action - typo", action: "subscrib"},
		{name: "unknown action - case mismatch", action: "Subscribe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := startHub(t)
			conn := dialHub(t, hub)
			defer func() {
				if err := conn.Close(); err != nil {
					t.Logf("conn.Close() error: %v", err)
				}
			}()

			waitForConnection(t, hub)

			// Send a valid JSON message with an unknown action.
			msg := subscribeMsg{Action: tt.action, PaneIDs: []string{"%0"}}
			data, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("failed to marshal message: %v", err)
			}
			if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
				t.Fatalf("failed to write message: %v", writeErr)
			}

			// Unknown action is silently ignored by handleSubscription (logged only).
			// The connection must remain alive. Verify by subscribing successfully.
			sendSubscribe(t, conn, []string{"%0"})
			waitForSubscribed(t, hub, "%0")

			// Verify the unknown action did NOT subscribe the pane under any key.
			hub.mu.RLock()
			subCount := len(hub.subscribed)
			hub.mu.RUnlock()
			// Only "%0" from the valid subscribe above should be present.
			if subCount != 1 {
				t.Errorf("subscribed count = %d, want 1 (only the valid subscribe)", subCount)
			}
		})
	}
}

func TestSendErrorBehavior(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()

	waitForConnection(t, hub)

	// Get server-side connection to pass to sendError (sendError writes to the
	// server-side conn, not the client-side conn).
	hub.mu.RLock()
	serverConn := hub.conn
	hub.mu.RUnlock()
	if serverConn == nil {
		t.Fatal("server-side connection is nil after waitForConnection")
	}

	// Directly call sendError with the server-side conn and verify the client receives it.
	hub.sendError(serverConn, "test error message")

	errResp := readErrorResponse(t, conn)
	if errResp.Type != "error" {
		t.Errorf("error type = %q, want %q", errResp.Type, "error")
	}
	if errResp.Message != "test error message" {
		t.Errorf("error message = %q, want %q", errResp.Message, "test error message")
	}
}

func TestSendErrorOnClosedConnection(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	waitForConnection(t, hub)

	// Capture the server-side connection before it gets cleared.
	hub.mu.RLock()
	serverConn := hub.conn
	hub.mu.RUnlock()

	// Close the client connection first.
	if err := conn.Close(); err != nil {
		t.Logf("conn.Close() error: %v", err)
	}
	waitForNoConnection(t, hub)

	// sendError on a closed server-side conn must not panic.
	hub.sendError(serverConn, "error after close")
}

// ---------------------------------------------------------------------------
// I-15: Abrupt disconnection simulation tests
// ---------------------------------------------------------------------------

func TestAbruptDisconnection(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)

	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	// Verify connection is active.
	if !hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = false before abrupt disconnect")
	}

	// Abrupt TCP-level close: bypasses WebSocket close handshake.
	// This simulates network failure, process crash, etc.
	rawConn := conn.UnderlyingConn()
	if err := rawConn.Close(); err != nil {
		t.Fatalf("rawConn.Close() error: %v", err)
	}

	// Hub should detect the broken connection via read pump error and clean up.
	waitForNoConnection(t, hub)

	// Verify final state (#118): no active connection and subscriptions cleared.
	if hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = true after abrupt disconnection")
	}

	hub.mu.RLock()
	subCount := len(hub.subscribed)
	hub.mu.RUnlock()
	if subCount != 0 {
		t.Errorf("subscribed count = %d after abrupt disconnect, want 0", subCount)
	}

	// BroadcastPaneData after abrupt disconnect must not panic.
	hub.BroadcastPaneData("%0", []byte("data after abrupt disconnect"))
}

func TestAbruptDisconnectionDuringBroadcast(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)

	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	// Abrupt close at TCP level.
	rawConn := conn.UnderlyingConn()
	if err := rawConn.Close(); err != nil {
		t.Fatalf("rawConn.Close() error: %v", err)
	}

	// Immediately broadcast before the read pump has time to detect the failure.
	// BroadcastPaneData should handle the write error gracefully.
	hub.BroadcastPaneData("%0", []byte("data during abrupt disconnect"))

	// Eventually the hub should clean up.
	waitForNoConnection(t, hub)

	// Verify final state (#118).
	if hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = true after abrupt disconnect + broadcast")
	}
}

// ---------------------------------------------------------------------------
// Shutdown and cleanup tests
// ---------------------------------------------------------------------------

func TestGracefulShutdown(t *testing.T) {
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()

	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	conn := dialHub(t, hub)
	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	// Stop the hub: should close the connection and shut down the server.
	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// Connection should be closed. Reading should fail.
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected read to fail after hub shutdown, but succeeded")
	}

	// Cleanup: close conn from our side (hub already closed it).
	if closeErr := conn.Close(); closeErr != nil {
		t.Logf("conn.Close() error (expected): %v", closeErr)
	}
}

func TestStopSubscribedMapIsNotNil(t *testing.T) {
	hub := startHub(t)

	conn := dialHub(t, hub)
	sendSubscribe(t, conn, []string{"%0"})
	waitForSubscribed(t, hub, "%0")

	// Stop the hub. The subscribed map must be reset to empty map, not nil.
	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// Accessing h.subscribed after Stop must not panic.
	// BroadcastPaneData uses h.subscribed[paneID] under RLock; verify no panic.
	hub.BroadcastPaneData("%0", []byte("after stop"))

	// Verify the map is not nil (#118 final state).
	hub.mu.RLock()
	isNil := hub.subscribed == nil
	hub.mu.RUnlock()
	if isNil {
		t.Fatal("subscribed map is nil after Stop(), want empty non-nil map")
	}

	// Close conn from our side (hub already closed it).
	if closeErr := conn.Close(); closeErr != nil {
		t.Logf("conn.Close() error (expected): %v", closeErr)
	}
}

// ---------------------------------------------------------------------------
// NewHub default options test (#119 boundary)
// ---------------------------------------------------------------------------

func TestNewHubDefaultAddr(t *testing.T) {
	hub := NewHub(HubOptions{})
	if hub.opts.Addr != testListenAddr {
		t.Errorf("default Addr = %q, want %q", hub.opts.Addr, testListenAddr)
	}
	if hub.subscribed == nil {
		t.Fatal("subscribed map is nil after NewHub, want non-nil")
	}
}

// ---------------------------------------------------------------------------
// I-16: Port conflict test
// ---------------------------------------------------------------------------

func TestStartPortConflict(t *testing.T) {
	// Start a hub on a specific port to occupy it.
	hub1 := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()
	if err := hub1.Start(ctx); err != nil {
		t.Fatalf("hub1.Start() returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := hub1.Stop(); err != nil {
			t.Logf("hub1.Stop() error: %v", err)
		}
	})

	// Extract the actual port assigned by the OS.
	u, err := url.Parse(hub1.URL())
	if err != nil {
		t.Fatalf("url.Parse(%q) error: %v", hub1.URL(), err)
	}
	occupiedAddr := net.JoinHostPort("127.0.0.1", u.Port())

	// Start a second hub on the same port. This must fail.
	hub2 := NewHub(HubOptions{Addr: occupiedAddr})
	if startErr := hub2.Start(ctx); startErr == nil {
		// If Start somehow succeeded, stop it to avoid resource leak.
		if stopErr := hub2.Stop(); stopErr != nil {
			t.Logf("hub2.Stop() error: %v", stopErr)
		}
		t.Fatal("hub2.Start() on occupied port should return an error, got nil")
	}
}

// ---------------------------------------------------------------------------
// I-17: pingLoop tests
// ---------------------------------------------------------------------------

func TestPingLoopSuccess(t *testing.T) {
	// Use a hub with a very short ping interval to test ping delivery.
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := hub.Stop(); err != nil {
			t.Logf("Stop() error: %v", err)
		}
	})

	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()
	waitForConnection(t, hub)

	// Set up a handler to detect incoming ping frames on the client side.
	pingReceived := make(chan struct{}, 1)
	conn.SetPingHandler(func(appData string) error {
		select {
		case pingReceived <- struct{}{}:
		default:
		}
		// Send a pong back (default gorilla behavior).
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	// Read in a goroutine so the ping handler fires (gorilla processes control
	// frames during ReadMessage calls).
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Wait for the ping to arrive within pingInterval + generous headroom.
	select {
	case <-pingReceived:
		// Success: the ping frame was received.
	case <-time.After(pingInterval + 5*time.Second):
		t.Fatal("timed out waiting for ping frame from hub")
	}
}

func TestPingLoopConnectionError(t *testing.T) {
	hub := NewHub(HubOptions{Addr: testListenAddr})
	ctx := t.Context()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := hub.Stop(); err != nil {
			t.Logf("Stop() error: %v", err)
		}
	})

	conn := dialHub(t, hub)
	waitForConnection(t, hub)

	// Abrupt TCP close so the next ping write will fail.
	rawConn := conn.UnderlyingConn()
	if err := rawConn.Close(); err != nil {
		t.Fatalf("rawConn.Close() error: %v", err)
	}

	// The pingLoop should detect the broken connection and clean up.
	waitForNoConnection(t, hub)

	if hub.HasActiveConnection() {
		t.Fatal("HasActiveConnection() = true after ping on broken connection")
	}
}

// ---------------------------------------------------------------------------
// S-34: Empty paneID subscribe rejection test
// ---------------------------------------------------------------------------

func TestSubscribeEmptyPaneIDRejected(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()
	waitForConnection(t, hub)

	// Send a subscribe message with an empty paneID in the list.
	sendSubscribe(t, conn, []string{""})

	// Give the hub time to process the message.
	waitForCondition(t, 500*time.Millisecond, func() bool {
		// The condition here is that the subscription was NOT added.
		// We poll briefly to ensure processing had time.
		return true
	})

	hub.mu.RLock()
	subCount := len(hub.subscribed)
	_, hasEmpty := hub.subscribed[""]
	hub.mu.RUnlock()

	if hasEmpty {
		t.Fatal("empty paneID should be rejected, but was added to subscribed map")
	}
	if subCount != 0 {
		t.Errorf("subscribed count = %d after empty paneID subscribe, want 0", subCount)
	}
}

// TestSubscribeEmptyPaneIDMixedWithValid verifies that empty paneIDs in a mixed
// list are rejected while valid ones are accepted.
func TestSubscribeEmptyPaneIDMixedWithValid(t *testing.T) {
	hub := startHub(t)
	conn := dialHub(t, hub)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close() error: %v", err)
		}
	}()
	waitForConnection(t, hub)

	// Send a subscribe with a mix of valid and empty paneIDs.
	sendSubscribe(t, conn, []string{"", "%valid", ""})
	waitForSubscribed(t, hub, "%valid")

	hub.mu.RLock()
	subCount := len(hub.subscribed)
	_, hasEmpty := hub.subscribed[""]
	hub.mu.RUnlock()

	if hasEmpty {
		t.Fatal("empty paneID should be rejected in mixed list")
	}
	if subCount != 1 {
		t.Errorf("subscribed count = %d, want 1 (only valid paneID)", subCount)
	}
}
