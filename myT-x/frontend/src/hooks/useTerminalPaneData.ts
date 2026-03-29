import type {Terminal} from "@xterm/xterm";
import {EventsOn} from "../../wailsjs/runtime/runtime";
import {registerPaneHandler, isConnected as isWsConnected, setReconnectCallback} from "../services/paneDataStream";
import type {TerminalEventShared} from "./useTerminalKeyHandler";

export interface PaneDataParams {
    readonly term: Terminal;
    readonly shared: TerminalEventShared;
    readonly paneId: string;
}

/**
 * Sets up the dual-listener pane data stream (WebSocket + Wails IPC fallback)
 * and RAF-batched write pipeline for a terminal pane.
 *
 * Called from the single useEffect in useTerminalEvents. Returns a cleanup function.
 *
 * C-1: Dual-listener approach — paneDataStream の registerPaneHandler で
 * WebSocket バイナリストリームを受信しつつ、Wails IPC の EventsOn("pane:data:<id>")
 * もフォールバックとして常時登録する。WebSocket が接続中でデータ受信実績がある間は
 * IPC データを無視し、WebSocket 切断時は自動的に IPC フォールバックが有効になる。
 */
export function setupPaneDataStream({term, shared, paneId}: PaneDataParams): () => void {
    let rafWriteID: number | null = null;
    const pendingWrites: string[] = [];

    // --- バックエンドからの出力を RAF でバッチ書き込み ---
    const flushPendingWrites = () => {
        rafWriteID = null;
        if (shared.disposed || pendingWrites.length === 0) {
            return;
        }
        if (shared.isComposing) {
            // During IME composition, buffer to composingOutput.
            // join("") is acceptable here: human typing speed, low frequency.
            shared.composingOutput.push(pendingWrites.join(""));
            pendingWrites.length = 0;
            return;
        }
        // xterm.js buffers write() calls internally and processes them in a
        // single parseBuffer pass. Multiple write() calls avoid the intermediate
        // join("") string allocation while achieving identical rendering output.
        // H-04: xterm.js batches writes internally, so multiple write() calls
        // avoid an intermediate join("") allocation while producing identical output.
        try {
            for (let i = 0; i < pendingWrites.length; i++) {
                term.write(pendingWrites[i]);
            }
        } catch (err) {
            console.warn("[terminal] flushPendingWrites failed (terminal may be disposed)", err);
        }
        pendingWrites.length = 0;
    };

    const enqueuePendingWrite = (data: string) => {
        // I-28: Guard against writes after cleanup.
        if (shared.disposed) return;

        if (shared.pageHidden) {
            // M-06: Skip RAF scheduling when page/tab is hidden, but preserve
            // data in the xterm.js internal buffer so that background command
            // output is not lost (Wails desktop app: window minimised != idle).
            // term.write() is safe to call off-screen; xterm.js queues internally.
            if (shared.isComposing) {
                shared.composingOutput.push(data);
                return;
            }
            try {
                term.write(data);
            } catch (err) {
                console.warn("[terminal] hidden write failed (terminal may be disposed)", err);
            }
            return;
        }

        pendingWrites.push(data);
        if (rafWriteID !== null) {
            return;
        }
        rafWriteID = window.requestAnimationFrame(flushPendingWrites);
    };

    // --- C-1: Dual-listener approach for pane data ---
    // Register BOTH WebSocket handler AND Wails IPC listener to ensure
    // pane data is received regardless of WebSocket connection state.
    //
    // Priority: When WebSocket is connected and delivering data, IPC
    // events are ignored to prevent duplicate rendering. When WebSocket
    // is disconnected/failed, the IPC fallback is already in place and
    // will deliver data sent by the backend via Wails EventsEmit.
    //
    // S-37: Design tradeoff — there is a brief window after component mount
    // where wsActive is false and paneDataStream has not yet delivered its
    // first frame. During that window both WS and IPC can process frames,
    // potentially causing a single initial burst to be written twice.
    // This is an acceptable tradeoff: the duplicate is at most one flush
    // worth of data, it resolves itself on the next WS frame, and it is
    // far preferable to dropping data during the startup race.

    // Flag: set to true when WebSocket has delivered at least one frame.
    // Once WebSocket proves working, IPC data is suppressed to avoid duplicates.
    // I-20: wsActive is reset to false in the reconnect callback so that if the
    // WS reconnects after a disconnection, the IPC suppression is re-evaluated
    // from scratch — preventing stale "wsActive=true" from blocking IPC data
    // during the window between reconnection and the first new WS frame.
    let wsActive = false;

    // WebSocket binary stream: registerPaneHandler receives raw Uint8Array
    // from paneDataStream module. Decode to string for xterm.js write().
    // I-18: paneTextDecoder uses stream: true across frames within a single WS
    // connection. On WS reconnect the old decoder may hold incomplete multi-byte
    // bytes from the previous session, so we reset it via the reconnect callback.
    // Using 'let' (not 'const') allows replacement with a fresh instance on reconnect.
    let paneTextDecoder = new TextDecoder("utf-8");

    // I-18 / I-20: Register a callback so that when paneDataStream (re)connects,
    // we flush any buffered bytes from the old stream-mode decoder and create a
    // fresh one, and reset wsActive so IPC suppression is re-evaluated cleanly.
    //
    // DESIGN CONSTRAINT: setReconnectCallback holds a single global callback
    // (last-write-wins). In a multi-pane environment only the last registered
    // pane's callback is active. This is acceptable because paneDataStream
    // reconnects affect all panes simultaneously, and each pane's WS handler
    // independently resets wsActive on receiving new data. If per-pane
    // reconnect logic is ever needed, migrate to a registry pattern similar
    // to registerPaneHandler.
    setReconnectCallback(() => {
        // Flush remaining bytes from the previous decoder session. This causes the
        // decoder to emit any incomplete sequence as U+FFFD and then clear its buffer,
        // preventing stale bytes from corrupting the start of the new stream.
        try {
            const flushed = paneTextDecoder.decode(new Uint8Array(), {stream: false});
            if (flushed.length > 0) {
                enqueuePendingWrite(flushed);
            }
        } catch (err) {
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-terminal] decoder flush failed during reconnect", err);
            }
        }
        paneTextDecoder = new TextDecoder("utf-8");
        wsActive = false; // I-20: re-enable IPC fallback until WS proves delivery
    });

    const unregisterPane = registerPaneHandler(paneId, (rawData: Uint8Array) => {
        wsActive = true;
        // stream: true preserves incomplete multi-byte sequences across chunk boundaries,
        // preventing U+FFFD replacement characters when UTF-8 is split across WebSocket frames.
        const text = paneTextDecoder.decode(rawData, {stream: true});
        enqueuePendingWrite(text);
    });

    // Wails IPC fallback: always registered so that if WebSocket never
    // connects or disconnects, pane data still arrives via EventsEmit.
    // The backend sends "pane:data:<paneId>" events when no WebSocket
    // subscription exists for the pane (or as a fallback path).
    const cancelIpcListener = EventsOn(`pane:data:${paneId}`, (data: unknown) => {
        // Suppress IPC data when WebSocket is actively delivering.
        // Check both the wsActive flag (has WS delivered data?) and
        // isWsConnected() (is WS currently open?) to handle the case
        // where WS was active but just disconnected — in that case,
        // wsActive is true but isWsConnected() is false, so we should
        // accept IPC data again.
        if (wsActive && isWsConnected()) {
            return;
        }
        // Reset wsActive when WS is no longer connected so that
        // subsequent IPC data flows through immediately.
        if (wsActive && !isWsConnected()) {
            wsActive = false;
        }
        if (typeof data === "string") {
            enqueuePendingWrite(data);
        } else {
            console.warn(`[terminal] IPC received non-string data for pane=${paneId}, ignoring`, typeof data);
        }
    });

    // --- Cleanup ---

    return () => {
        if (rafWriteID !== null) {
            window.cancelAnimationFrame(rafWriteID);
        }
        pendingWrites.length = 0;

        // S-11: Flush any incomplete multi-byte sequence buffered by the
        // streaming TextDecoder. decode() with no arguments forces the
        // decoder to emit remaining bytes and resets internal state,
        // preventing a memory leak from held buffers.
        try {
            paneTextDecoder.decode();
        } catch (err) {
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-terminal] decoder final flush failed", err);
            }
        }

        // Clear the reconnect callback so that paneDataStream does not call into
        // a stale closure after this effect cleans up. (I-18 / I-20 cleanup)
        setReconnectCallback(null);

        // Unsubscribe pane from WebSocket stream and remove handler.
        unregisterPane();

        // Remove Wails IPC fallback listener (#C-1).
        try {
            cancelIpcListener();
        } catch (err) {
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-terminal] IPC listener cleanup failed", err);
            }
        }
    };
}
