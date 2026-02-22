/**
 * paneDataStream manages a single WebSocket connection for streaming
 * terminal pane output from the Go backend to React terminal components.
 *
 * Architecture: Module-scope singleton (not React state) to avoid unnecessary
 * re-renders. Similar pattern to webglUnavailable flag in useTerminalSetup.ts.
 *
 * Protocol:
 * - Server -> Client: Binary frames [1byte:paneIDLen][paneID][data]
 * - Client -> Server: JSON text {"action":"subscribe","paneIds":["..."]}
 *
 * Reconnection: Exponential backoff (100ms -> 5s cap), max 10 retries.
 * After max retries, a user-visible error notification is displayed.
 */

import {useNotificationStore} from "../stores/notificationStore";

// --- Constants ---

// Initial reconnection delay. Short enough for fast recovery from
// transient disconnects (e.g., page reload race).
const INITIAL_BACKOFF_MS = 100;

// Maximum reconnection delay. Prevents excessive wait times while
// still spacing out retries to avoid hammering the server.
const MAX_BACKOFF_MS = 5000;

// Maximum reconnection attempts before showing a persistent error.
// At exponential backoff, 10 retries span ~30 seconds total.
const MAX_RECONNECT_RETRIES = 10;

// Custom event type dispatched when max reconnect retries are exhausted.
// External code can listen for this to implement recovery UI.
const MAX_RETRIES_EVENT = "paneDataStream:maxRetriesReached";

// --- Module state ---

let ws: WebSocket | null = null;
let wsUrl: string | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempt = 0;
let intentionalDisconnect = false;

// reconnectCallback is invoked each time the WebSocket successfully (re)connects.
// Callers can register a callback to reset stream-mode decoders or other per-connection
// state that must be flushed when a new connection replaces the previous one.
// Only one callback is stored; a new call to setReconnectCallback replaces the previous.
let reconnectCallback: (() => void) | null = null;

// paneHandlers stores at most one handler per paneID. Re-registering a paneID
// silently replaces the previous handler (previous handler is discarded on re-register).
// Callers must unregister before re-registering if the old handler should be invoked.
const paneHandlers = new Map<string, (data: Uint8Array) => void>();

// paneIdDecoder is used exclusively to decode the pane ID prefix from each binary frame.
// stream: false is correct here because the pane ID is always a complete, self-contained
// UTF-8 string delivered within a single frame — there are no cross-frame multi-byte splits.
const paneIdDecoder = new TextDecoder("utf-8");

// --- Public API ---

/**
 * connect establishes a WebSocket connection to the given URL.
 * If already connected, the existing connection is closed first.
 *
 * After successful connection, all currently registered pane handlers
 * are automatically subscribed on the server.
 */
export function connect(url: string): void {
    wsUrl = url;
    intentionalDisconnect = false;
    reconnectAttempt = 0;
    clearReconnectTimer();
    closeExisting();
    openConnection(url);
}

/**
 * disconnect closes the WebSocket connection and cancels any pending
 * reconnection timer. After calling disconnect(), automatic reconnection
 * is suppressed until connect() is called again.
 *
 * All registered pane handlers are cleared to allow GC of React components.
 *
 * NOTE: After disconnect(), paneHandlers is empty. To resume receiving pane
 * output after calling connect() again, each consumer must re-register its
 * handler via registerPaneHandler(). The useTerminalEvents hook handles this
 * automatically: its useEffect cleanup unmounts the handler, and re-mounting
 * (triggered by the paneId dependency) calls registerPaneHandler() anew.
 */
export function disconnect(): void {
    intentionalDisconnect = true;
    clearReconnectTimer();
    closeExisting();
    paneHandlers.clear(); // I-3: prevent GC leak after unmount
    wsUrl = null;
}

/**
 * registerPaneHandler registers a callback that receives raw binary terminal
 * output for the given pane ID. If a WebSocket connection is active, the pane
 * is automatically subscribed on the server.
 *
 * Returns an unregister function that removes the handler and unsubscribes.
 */
export function registerPaneHandler(
    paneId: string,
    handler: (data: Uint8Array) => void,
): () => void {
    paneHandlers.set(paneId, handler);
    sendMessage("subscribe", [paneId]);
    return () => {
        unregisterPaneHandler(paneId);
    };
}

/**
 * unregisterPaneHandler removes the handler for the given pane ID and
 * sends an unsubscribe message to the server.
 */
export function unregisterPaneHandler(paneId: string): void {
    paneHandlers.delete(paneId);
    sendMessage("unsubscribe", [paneId]);
}

/**
 * isConnected returns whether the WebSocket connection is currently open.
 */
export function isConnected(): boolean {
    return ws !== null && ws.readyState === WebSocket.OPEN;
}

/**
 * setReconnectCallback registers a callback that is invoked each time the
 * WebSocket successfully (re)connects (onopen fires). Use this to reset
 * per-connection streaming state, e.g. flush a stream-mode TextDecoder or
 * clear a "WS active" flag so that IPC fallback suppression is re-evaluated.
 *
 * Only one callback is supported; a subsequent call replaces the previous one.
 * Pass null to clear the callback.
 */
export function setReconnectCallback(cb: (() => void) | null): void {
    reconnectCallback = cb;
}

// --- Internal functions ---

function openConnection(url: string): void {
    try {
        const socket = new WebSocket(url);
        socket.binaryType = "arraybuffer";

        socket.onopen = () => {
            reconnectAttempt = 0;
            slog("[DEBUG-WS] connected", url);
            // Notify listeners that the connection has (re)established so they
            // can reset per-connection state (e.g. flush streaming TextDecoders,
            // clear wsActive flags). Fired before re-subscribe to ensure state
            // is clean when the first new frames arrive. (I-18, I-20)
            if (reconnectCallback !== null) {
                try {
                    reconnectCallback();
                } catch (err) {
                    if (import.meta.env.DEV) {
                        console.warn("[DEBUG-WS] reconnectCallback threw", err);
                    }
                }
            }
            // Re-subscribe all currently registered panes.
            const paneIds = Array.from(paneHandlers.keys());
            if (paneIds.length > 0) {
                sendMessageVia(socket, "subscribe", paneIds);
            }
        };

        socket.onmessage = (event: MessageEvent) => {
            try { // #95: try/catch in async event handler
                handleBinaryFrame(event);
            } catch (err) {
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-WS] onmessage handler error", err); // #84: catch内でユーザー通知
                }
            }
        };

        socket.onclose = (event: CloseEvent) => {
            // Guard against stale close events from replaced connections.
            // When connect() calls closeExisting() then openConnection(), the old
            // socket's onclose fires asynchronously and must not overwrite the new ws.
            if (ws !== socket) {
                slog("[DEBUG-WS] stale onclose ignored (connection was replaced)");
                return;
            }
            slog("[DEBUG-WS] disconnected", event.code, event.reason);
            ws = null;
            if (!intentionalDisconnect) {
                scheduleReconnect();
            }
        };

        socket.onerror = (event: Event) => {
            slog("[DEBUG-WS] error", event);
            // onclose will fire after onerror, which triggers reconnection.
        };

        ws = socket;
    } catch (err) {
        slog("[DEBUG-WS] connection creation failed", err);
        ws = null;
        if (!intentionalDisconnect) {
            scheduleReconnect();
        }
    }
}

/**
 * handleBinaryFrame parses and dispatches a single binary WebSocket frame.
 * Invalid frames are logged and silently dropped (S-19, S-13).
 */
function handleBinaryFrame(event: MessageEvent): void {
    if (!(event.data instanceof ArrayBuffer)) {
        if (import.meta.env.DEV) {
            console.warn("[DEBUG-WS] non-binary frame received, ignoring"); // S-19: log unexpected frame type
        }
        return;
    }
    const view = new Uint8Array(event.data);
    // Minimum frame: 1 byte length prefix
    if (view.length < 1) {
        if (import.meta.env.DEV) {
            console.warn("[DEBUG-WS] invalid frame: too short", view.length); // S-19: log malformed frame
        }
        return;
    }
    const paneIdLen = view[0];

    // I-07 / S-13: defensive check for zero-length paneId.
    // Go side EncodePaneData already guards against this, but we check here too
    // as a belt-and-suspenders defence against malformed frames from any source.
    if (paneIdLen === 0) {
        if (import.meta.env.DEV) {
            console.warn("[DEBUG-WS] invalid frame: paneIdLen is 0, dropping frame");
        }
        return;
    }

    if (view.length < 1 + paneIdLen) {
        if (import.meta.env.DEV) {
            console.warn("[DEBUG-WS] invalid frame: declared paneIdLen", paneIdLen, "exceeds frame size", view.length); // S-19
        }
        return;
    }
    const paneId = paneIdDecoder.decode(view.subarray(1, 1 + paneIdLen));
    const data = view.subarray(1 + paneIdLen);
    const handler = paneHandlers.get(paneId);
    if (handler) {
        handler(data);
    }
}

function closeExisting(): void {
    if (ws !== null) {
        // I-4: null ws BEFORE close() so that synchronous onclose sees ws===null
        // and the stale guard (ws !== socket) fires correctly.
        const closing = ws;
        ws = null;
        try {
            if (closing.readyState !== WebSocket.CLOSED && closing.readyState !== WebSocket.CLOSING) {
                closing.close(1000, "client disconnect");
            }
        } catch (_) {
            // Ignore close errors during cleanup.
        }
    }
}

function scheduleReconnect(): void {
    if (intentionalDisconnect || wsUrl === null) {
        return;
    }
    if (reconnectAttempt >= MAX_RECONNECT_RETRIES) {
        slog("[DEBUG-WS] max reconnect retries reached", reconnectAttempt);

        // S-17: notify user and dispatch recoverable event
        if (import.meta.env.DEV) {
            console.warn("[DEBUG-WS] max reconnect retries exhausted. User action required.");
        }
        if (typeof window !== "undefined") {
            try {
                window.dispatchEvent(new CustomEvent(MAX_RETRIES_EVENT, {
                    detail: {attempts: reconnectAttempt},
                }));
            } catch (err) {
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-WS] failed to dispatch max-retries event", err);
                }
            }
        }

        // #87: UI notification for persistent connection failure.
        try { // #84: catch内でユーザー通知
            useNotificationStore.getState().addNotification(
                "ターミナル出力の接続に失敗しました。アプリを再起動してください。",
                "error",
            );
        } catch (err) {
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-WS] failed to show notification", err);
            }
        }
        return;
    }

    const delay = Math.min(
        INITIAL_BACKOFF_MS * Math.pow(2, reconnectAttempt),
        MAX_BACKOFF_MS,
    );
    reconnectAttempt++;
    slog("[DEBUG-WS] reconnecting in", delay, "ms (attempt", reconnectAttempt, ")");

    clearReconnectTimer();
    reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        if (wsUrl !== null && !intentionalDisconnect) {
            openConnection(wsUrl);
        }
    }, delay);
}

function clearReconnectTimer(): void {
    if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }
}

// S-23: DRY — unified send helper for subscribe/unsubscribe (#109)

/**
 * sendMessage sends a JSON control message to the server via the current
 * WebSocket connection (if open).
 */
function sendMessage(action: string, paneIds: string[]): void {
    if (ws !== null && ws.readyState === WebSocket.OPEN) {
        sendMessageVia(ws, action, paneIds);
    }
}

/**
 * sendMessageVia sends a JSON control message via the specified socket.
 * Used during onopen to send through the newly created socket reference
 * before ws is assigned.
 */
function sendMessageVia(socket: WebSocket, action: string, paneIds: string[]): void {
    try {
        socket.send(JSON.stringify({action, paneIds}));
    } catch (err) {
        slog("[DEBUG-WS]", action, "send failed", err);
    }
}

/** Thin wrapper for debug logging. Kept for development; remove at release. */
function slog(...args: unknown[]): void {
    if (import.meta.env.DEV) {
        console.debug(...args);
    }
}

// I-22: HMR cleanup — close the module-scope WebSocket when Vite hot-replaces
// this module during development. Without this, the old connection lingers until
// the browser GCs it, causing duplicate message handlers and confusing logs.
if (import.meta.hot) {
    import.meta.hot.dispose(() => disconnect());
}
