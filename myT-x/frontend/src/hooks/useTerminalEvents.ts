import {type Dispatch, type MutableRefObject, type SetStateAction, useEffect} from "react";
import type {Terminal} from "@xterm/xterm";
import {
    ClipboardGetText,
    ClipboardSetText,
    EventsOn,
} from "../../wailsjs/runtime/runtime";
import {api} from "../api";
import {registerPaneHandler, isConnected as isWsConnected, setReconnectCallback} from "../services/paneDataStream";
import {useTmuxStore} from "../stores/tmuxStore";
import {isImeTransitionalEvent} from "../utils/ime";
import {resolveActivePaneID} from "../utils/session";

interface UseTerminalEventsOptions {
    paneId: string;
    terminalRef: MutableRefObject<Terminal | null>;
    syncInputModeRef: MutableRefObject<boolean>;
    setSearchOpen: Dispatch<SetStateAction<boolean>>;
    setScrollAtBottom: Dispatch<SetStateAction<boolean>>;
    /**
     * IME composition 中かどうかを示す共有 ref。
     * compositionstart/compositionend リスナーが更新し、
     * useTerminalResize がリサイズ保留判定のために参照する。
     */
    isComposingRef: MutableRefObject<boolean>;
}

/**
 * WebSocket 経由のペインバイナリストリーム受信・Wails IPC フォールバック・
 * onData キー入力・IME composition・コンテキストメニュー（コピー/ペースト）・
 * カスタムキーハンドラ・スクロール位置インジケータを管理する。
 *
 * C-1: Dual-listener approach — paneDataStream の registerPaneHandler で
 * WebSocket バイナリストリームを受信しつつ、Wails IPC の EventsOn("pane:data:<id>")
 * もフォールバックとして常時登録する。WebSocket が接続中でデータ受信実績がある間は
 * IPC データを無視し、WebSocket 切断時は自動的に IPC フォールバックが有効になる。
 * クリーンアップ時に両方のリスナーを解除する。
 *
 * INVARIANT: useTerminalSetup must be called before this hook so that
 * terminalRef.current is populated. This hook also registers composition
 * event listeners that update isComposingRef — useTerminalResize depends
 * on isComposingRef being kept up-to-date. See TerminalPane.tsx for the
 * full hook ordering contract.
 *
 * WARNING: Hook call order matters — do not reorder the hook calls in TerminalPane.tsx
 * without reviewing cross-hook dependencies.
 *
 * 依存配列: [paneId] — paneId が変わるたびに再登録する。
 */
export function useTerminalEvents({
                                      paneId,
                                      terminalRef,
                                      syncInputModeRef,
                                      setSearchOpen,
                                      setScrollAtBottom,
                                      isComposingRef,
                                  }: UseTerminalEventsOptions): void {
    const setPrefixMode = useTmuxStore((s) => s.setPrefixMode);

    useEffect(() => {
        const term = terminalRef.current;
        if (!term) {
            return;
        }

        let disposed = false;
        let rafWriteID: number | null = null;
        let scrollWriteTimer: ReturnType<typeof window.setTimeout> | null = null;
        let copyOnSelectTimer: ReturnType<typeof window.setTimeout> | null = null;
        let focusRecoverRAFId: number | null = null;
        let focusRecoverTimer: ReturnType<typeof window.setTimeout> | null = null;
        // M-06: Skip rendering when page tab is hidden. Backend continues processing
        // pane output; rendering resumes automatically when the tab becomes visible
        // and the next pane:data event arrives. See task/速度改善.md.
        let pageHidden = document.hidden;
        const pendingWrites: string[] = [];

        // --- IME composition バッファリング ---
        let isComposing = false;
        let composingOutput: string[] = [];
        const compositionTextarea = term.textarea ?? null;

        const flushComposedOutput = () => {
            if (composingOutput.length === 0) {
                return;
            }
            const buffered = composingOutput.join("");
            composingOutput.length = 0;
            // useTerminalSetup のクリーンアップが先に実行され term.dispose() が
            // 呼ばれた後にこの関数が到達する競合窓がある。try-catch で安全に吸収する。
            try {
                term.write(buffered);
            } catch (err) {
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-terminal] flushComposedOutput failed (terminal may be disposed)", err);
                }
            }
        };

        const finishComposition = () => {
            isComposing = false;
            isComposingRef.current = false;
            flushComposedOutput();
        };

        const onCompositionStart = () => {
            isComposing = true;
            isComposingRef.current = true;
        };
        const onCompositionEnd = () => {
            finishComposition();
        };

        const isCurrentPaneActive = (): boolean => {
            const state = useTmuxStore.getState();
            if (!state.activeSession) {
                return false;
            }
            const activeSessionSnapshot = state.sessions.find((session) => session.name === state.activeSession) ?? null;
            return resolveActivePaneID(activeSessionSnapshot) === paneId;
        };

        const clearFocusRecoveryTimers = () => {
            if (focusRecoverRAFId !== null) {
                window.cancelAnimationFrame(focusRecoverRAFId);
                focusRecoverRAFId = null;
            }
            if (focusRecoverTimer !== null) {
                window.clearTimeout(focusRecoverTimer);
                focusRecoverTimer = null;
            }
        };

        const shouldRestoreTerminalFocus = (forComposition: boolean): boolean => {
            if (disposed || pageHidden || !document.hasFocus() || !isCurrentPaneActive()) {
                return false;
            }
            if (forComposition && !isComposing) {
                return false;
            }
            const activeElement = document.activeElement;
            if (activeElement === compositionTextarea) {
                return false;
            }
            if (activeElement === null || activeElement === document.body) {
                return true;
            }
            const terminalElement = term.element;
            return terminalElement instanceof HTMLElement && terminalElement.contains(activeElement);
        };

        const scheduleTerminalFocusRecovery = (reason: "window-focus" | "visibilitychange" | "composition-blur"): void => {
            clearFocusRecoveryTimers();
            focusRecoverRAFId = window.requestAnimationFrame(() => {
                focusRecoverRAFId = null;
                focusRecoverTimer = window.setTimeout(() => {
                    focusRecoverTimer = null;
                    const restoreForComposition = reason === "composition-blur";
                    if (shouldRestoreTerminalFocus(restoreForComposition)) {
                        try {
                            term.focus();
                        } catch (err) {
                            if (import.meta.env.DEV) {
                                console.warn("[DEBUG-terminal] focus recovery failed", err);
                            }
                        }
                        return;
                    }
                    if (isComposing && !pageHidden && document.hasFocus()) {
                        // IME変換中に意図的に他要素へフォーカスが移った場合は、
                        // stale isComposing ロックを避けるため状態を確定させる。
                        finishComposition();
                    }
                }, 0);
            });
        };

        // compositionend が発火しない異常時（フォーカス喪失等）の安全弁。
        // 即 finish せず、1フレーム後に activeElement を確認してから復元可否を判定する。
        const onBlur = () => {
            if (isComposing) {
                scheduleTerminalFocusRecovery("composition-blur");
            }
        };

        const onWindowFocus = () => {
            if (!isCurrentPaneActive()) {
                return;
            }
            scheduleTerminalFocusRecovery("window-focus");
        };

        const onVisibilityChange = () => {
            pageHidden = document.hidden;
            if (pageHidden || !isCurrentPaneActive()) {
                return;
            }
            scheduleTerminalFocusRecovery("visibilitychange");
        };

        if (compositionTextarea) {
            compositionTextarea.addEventListener("compositionstart", onCompositionStart);
            compositionTextarea.addEventListener("compositionend", onCompositionEnd);
            compositionTextarea.addEventListener("blur", onBlur);
        }
        window.addEventListener("focus", onWindowFocus);
        document.addEventListener("visibilitychange", onVisibilityChange);

        // --- Copy on Select ---
        const selectionDisposable = term.onSelectionChange(() => {
            if (copyOnSelectTimer !== null) window.clearTimeout(copyOnSelectTimer);
            copyOnSelectTimer = window.setTimeout(() => {
                copyOnSelectTimer = null;
                const selection = term.getSelection();
                if (selection) {
                    void ClipboardSetText(selection).catch((err) => {
                        if (import.meta.env.DEV) {
                            console.warn("[DEBUG-copy] clipboard write failed", err);
                        }
                    });
                }
            }, 100);
        });

        // --- 右クリック: 選択あり->コピー / 選択なし->ペースト ---
        const termEl = term.element;
        const handleContextMenu = (e: MouseEvent) => {
            e.preventDefault();
            e.stopPropagation();
            const selection = term.getSelection();
            if (selection) {
                void ClipboardSetText(selection).catch((err) => {
                    if (import.meta.env.DEV) {
                        console.warn("[DEBUG-copy] clipboard write failed", err);
                    }
                });
                term.clearSelection();
            } else {
                void ClipboardGetText()
                    .then((text) => {
                        if (text) {
                            term.paste(text);
                        }
                    })
                    .catch((err) => {
                        if (import.meta.env.DEV) {
                            console.error("[DEBUG-paste] clipboard read failed", err);
                        }
                    });
            }
        };
        if (termEl) {
            termEl.addEventListener("contextmenu", handleContextMenu);
        }

        term.attachCustomKeyEventHandler((event) => {
            // Block keyboard events during IME composition to prevent double input.
            // return false = suppress xterm key handling, let browser IME handle it.
            if (isComposing || isImeTransitionalEvent(event)) {
                return false;
            }
            // Only process keydown events for shortcuts; let xterm handle keyup/keypress normally.
            if (event.type !== "keydown") {
                return true;
            }

            // Ctrl+B: tmux prefix mode
            if (event.ctrlKey && (event.key === "b" || event.key === "B")) {
                setPrefixMode(true);
                return false;
            }

            // Ctrl+F / Ctrl+Shift+F: 検索バーを開く
            if (event.ctrlKey && (event.key === "f" || event.key === "F")) {
                setSearchOpen(true);
                return false;
            }

            // Smart Ctrl+C: 選択あり->コピー、選択なし->SIGINT 送信
            if (event.ctrlKey && (event.key === "c" || event.key === "C")) {
                const selection = term.getSelection();
                if (selection) {
                    void ClipboardSetText(selection).catch((err) => {
                        if (import.meta.env.DEV) {
                            console.warn("[DEBUG-copy] clipboard write failed", err);
                        }
                    });
                    term.clearSelection();
                    return false;
                }
                return true;
            }

            // Ctrl+V: クリップボードからペースト（ブラケットペースト対応）
            if (event.ctrlKey && (event.key === "v" || event.key === "V")) {
                // Keep native paste event path and only suppress xterm key translation (^V).
                return false;
            }
            return true;
        });

        // --- キー入力送信 ---
        const inputDisposable = term.onData((input) => {
            if (syncInputModeRef.current) {
                void api.SendSyncInput(paneId, input).catch((err) => {
                    if (import.meta.env.DEV) {
                        console.warn(`[DEBUG-terminal] sync input failed for pane=${paneId}`, err);
                    }
                });
            } else {
                void api.SendInput(paneId, input).catch((err) => {
                    if (import.meta.env.DEV) {
                        console.warn(`[DEBUG-terminal] input failed for pane=${paneId}`, err);
                    }
                });
            }
        });

        // --- バックエンドからの出力を RAF でバッチ書き込み ---
        const flushPendingWrites = () => {
            rafWriteID = null;
            if (disposed || pendingWrites.length === 0) {
                return;
            }
            if (isComposing) {
                // During IME composition, buffer to composingOutput.
                // join("") is acceptable here: human typing speed, low frequency.
                composingOutput.push(pendingWrites.join(""));
                pendingWrites.length = 0;
                return;
            }
            // xterm.js buffers write() calls internally and processes them in a
            // single parseBuffer pass. Multiple write() calls avoid the intermediate
            // join("") string allocation while achieving identical rendering output.
            // See H-04 in task/速度改善.md.
            try {
                for (let i = 0; i < pendingWrites.length; i++) {
                    term.write(pendingWrites[i]);
                }
            } catch (err) {
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-terminal] flushPendingWrites failed (terminal may be disposed)", err);
                }
            }
            pendingWrites.length = 0;
        };

        const enqueuePendingWrite = (data: string) => {
            // I-28: Guard against writes after cleanup.
            if (disposed) return;

            if (pageHidden) {
                // M-06: Skip RAF scheduling when page/tab is hidden, but preserve
                // data in the xterm.js internal buffer so that background command
                // output is not lost (Wails desktop app: window minimised ≠ idle).
                // term.write() is safe to call off-screen; xterm.js queues internally.
                if (isComposing) {
                    composingOutput.push(data);
                    return;
                }
                try {
                    term.write(data);
                } catch (err) {
                    if (import.meta.env.DEV) {
                        console.warn("[DEBUG-terminal] hidden write failed (terminal may be disposed)", err);
                    }
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
            }
        });

        // --- スクロール位置インジケータ ---
        const updateScrollState = () => {
            if (disposed) return;
            const buf = term.buffer.active;
            const atBottom = buf.viewportY >= buf.baseY;
            setScrollAtBottom(atBottom);
        };
        const scheduleScrollWriteUpdate = () => {
            if (disposed || scrollWriteTimer !== null) {
                return;
            }
            scrollWriteTimer = window.setTimeout(() => {
                scrollWriteTimer = null;
                updateScrollState();
            }, 100);
        };
        const scrollDisposable = term.onScroll(updateScrollState);
        const writeDisposable = term.onWriteParsed(scheduleScrollWriteUpdate);

        return () => {
            disposed = true;

            if (rafWriteID !== null) {
                window.cancelAnimationFrame(rafWriteID);
            }
            if (scrollWriteTimer !== null) {
                window.clearTimeout(scrollWriteTimer);
            }
            if (copyOnSelectTimer !== null) {
                window.clearTimeout(copyOnSelectTimer);
            }
            clearFocusRecoveryTimers();

            isComposing = false;
            isComposingRef.current = false;
            composingOutput.length = 0;
            pendingWrites.length = 0;

            if (compositionTextarea) {
                compositionTextarea.removeEventListener("compositionstart", onCompositionStart);
                compositionTextarea.removeEventListener("compositionend", onCompositionEnd);
                compositionTextarea.removeEventListener("blur", onBlur);
            }
            termEl?.removeEventListener("contextmenu", handleContextMenu);
            window.removeEventListener("focus", onWindowFocus);
            document.removeEventListener("visibilitychange", onVisibilityChange);

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
            // a stale closure after this component unmounts. (I-18 / I-20 cleanup)
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

            selectionDisposable.dispose();
            scrollDisposable.dispose();
            writeDisposable.dispose();
            inputDisposable.dispose();
        };
        // Zustand store actions (setPrefixMode) and React setState dispatchers
        // (setSearchOpen, setScrollAtBottom) are stable references across renders.
        // Including them in the dependency array would cause no functional change
        // but would generate a false ESLint warning about missing deps.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [paneId]);
}
