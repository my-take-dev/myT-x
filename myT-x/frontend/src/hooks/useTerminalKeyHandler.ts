import type {Dispatch, MutableRefObject, SetStateAction} from "react";
import type {Terminal} from "@xterm/xterm";
import {ClipboardGetText} from "../../wailsjs/runtime/runtime";
import {writeClipboardText} from "../utils/clipboardUtils";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";
import {createConsecutiveFailureCounter, notifyAndLog} from "../utils/notifyUtils";
import {shouldRecoverTerminalFocus, type TerminalFocusRecoveryReason} from "../utils/terminalFocus";
import {shouldLetXtermHandleImeEvent} from "../utils/terminalIme";
import {pasteTextSafely} from "../utils/terminalPaste";
import {resolveActivePaneID} from "../utils/session";

// Used as rate limiters (threshold=1): every failure fires, but cooldown prevents
// toast spam within 5s. recordSuccess is called on success to reset cooldown state
// for consistency with all other failure counters in the codebase.
const inputFailureCounter = createConsecutiveFailureCounter(1, 5_000);
const clipboardWriteFailureCounter = createConsecutiveFailureCounter(1, 5_000);
const clipboardReadFailureCounter = createConsecutiveFailureCounter(1, 5_000);

function notifyInputFailureThrottled(err: unknown): void {
    inputFailureCounter.recordFailure(() => {
        notifyAndLog("Send input", "warn", err, "TerminalKeyHandler");
    });
}

function notifyClipboardFailureThrottled(err: unknown): void {
    clipboardWriteFailureCounter.recordFailure(() => {
        notifyAndLog("Copy to clipboard", "warn", err, "TerminalKeyHandler");
    });
}

function notifyPasteFailureThrottled(err: unknown): void {
    clipboardReadFailureCounter.recordFailure(() => {
        notifyAndLog("Paste from clipboard", "warn", err, "TerminalKeyHandler");
    });
}

/** Shared mutable state between terminal event sub-systems within a single useEffect lifetime. */
export interface TerminalEventShared {
    disposed: boolean;
    pageHidden: boolean;
    isComposing: boolean;
    composingOutput: string[];
}

/** IME lifecycle functions created by the entry-point hook, consumed by the key handler. */
export interface ImeControls {
    readonly gate: {
        markCompositionStart(): void;
        filterInput(input: string): string | null;
    };
    readonly compositionTextarea: HTMLTextAreaElement | null;
    readonly finishComposition: (commitEnded: boolean, eventData?: string) => void;
}

export interface KeyHandlerParams {
    readonly term: Terminal;
    readonly shared: TerminalEventShared;
    readonly ime: ImeControls;
    readonly paneId: string;
    readonly isComposingRef: MutableRefObject<boolean>;
    readonly syncInputModeRef: MutableRefObject<boolean>;
    readonly setSearchOpen: Dispatch<SetStateAction<boolean>>;
    readonly setPrefixMode: (active: boolean) => void;
}

/**
 * Sets up keyboard input, IME composition listeners, clipboard copy/paste,
 * context menu, focus recovery, and copy-on-select within a terminal pane.
 *
 * Called from the single useEffect in useTerminalEvents. Returns a cleanup function.
 */
export function setupKeyHandler({
    term,
    shared,
    ime,
    paneId,
    isComposingRef,
    syncInputModeRef,
    setSearchOpen,
    setPrefixMode,
}: KeyHandlerParams): () => void {

    let focusRecoverRAFId: number | null = null;
    let focusRecoverTimer: ReturnType<typeof window.setTimeout> | null = null;
    let copyOnSelectTimer: ReturnType<typeof window.setTimeout> | null = null;

    // --- Focus recovery helpers ---

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

    const shouldAttemptTerminalFocusRecovery = (reason: TerminalFocusRecoveryReason): boolean => {
        if (shared.disposed || shared.pageHidden || !document.hasFocus() || !isCurrentPaneActive()) {
            return false;
        }
        if (reason === "composition-blur" && !shared.isComposing) {
            return false;
        }
        return shouldRecoverTerminalFocus(reason, document.activeElement, term.element ?? null, ime.compositionTextarea);
    };

    const scheduleTerminalFocusRecovery = (reason: TerminalFocusRecoveryReason): void => {
        clearFocusRecoveryTimers();
        focusRecoverRAFId = window.requestAnimationFrame(() => {
            focusRecoverRAFId = null;
            focusRecoverTimer = window.setTimeout(() => {
                focusRecoverTimer = null;
                if (shouldAttemptTerminalFocusRecovery(reason)) {
                    try {
                        term.focus();
                    } catch (err) {
                        if (import.meta.env.DEV) {
                            console.warn("[DEBUG-terminal] focus recovery failed", err);
                        }
                    }
                    return;
                }
                if (shared.isComposing && !shared.pageHidden && document.hasFocus()) {
                    // IME変換中に意図的に他要素へフォーカスが移った場合は、
                    // stale isComposing ロックを避けるため状態を確定させる。
                    ime.finishComposition(false);
                }
            }, 0);
        });
    };

    // --- Event handlers ---

    // compositionend が発火しない異常時（フォーカス喪失等）の安全弁。
    // 即 finish せず、1フレーム後に activeElement を確認してから復元可否を判定する。
    const onBlur = () => {
        if (shared.isComposing) {
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
        shared.pageHidden = document.hidden;
        if (shared.pageHidden || !isCurrentPaneActive()) {
            return;
        }
        scheduleTerminalFocusRecovery("visibilitychange");
    };

    // Layer 1: Track compositionend timing for capture-phase input interception.
    // xterm.js v6 _inputEvent fires triggerDataEvent for insertText input events
    // without updating _dataAlreadySent, causing double onData with _finalizeComposition.
    // Intercepting insertText events within 50ms of compositionend prevents this path.
    let recentCompositionEndAt = 0;

    const onCompositionStart = () => {
        // Reset Layer 1 timestamp — a new composition invalidates the previous
        // compositionend window so stale timestamps cannot block unrelated input events.
        recentCompositionEndAt = 0;
        // Layer 3: Clear stale composingOutput from a previous cycle where
        // compositionend may not have fired (e.g., focus loss). New backend
        // output during THIS composition will be re-buffered normally.
        // NOTE: We drop the buffer rather than flushing because flushComposedOutput()
        // lives in useTerminalEvents scope and is not accessible here. The data is
        // stale (previous cycle's buffered backend output) and safe to discard.
        if (shared.composingOutput.length > 0) {
            if (import.meta.env.DEV) {
                const totalBytes = shared.composingOutput.reduce((sum, s) => sum + s.length, 0);
                console.warn("[DEBUG-ime] stale composingOutput cleared at compositionstart",
                    {buffered: shared.composingOutput.length, totalChars: totalBytes});
            }
            shared.composingOutput.length = 0;
        }
        shared.isComposing = true;
        isComposingRef.current = true;
        ime.gate.markCompositionStart();
    };
    const onCompositionEnd = (e: Event) => {
        recentCompositionEndAt = performance.now();
        const compositionEvent = e as CompositionEvent;
        ime.finishComposition(true, compositionEvent.data ?? "");
    };

    // --- Register composition / window / document listeners ---

    if (ime.compositionTextarea) {
        ime.compositionTextarea.addEventListener("compositionstart", onCompositionStart);
        ime.compositionTextarea.addEventListener("compositionend", onCompositionEnd);
        ime.compositionTextarea.addEventListener("blur", onBlur);
    }
    window.addEventListener("focus", onWindowFocus);
    document.addEventListener("visibilitychange", onVisibilityChange);

    // --- Copy on Select ---
    // Do NOT cancel the timer on deselection (click-away). The selection value
    // is captured at schedule time and copied after debounce, preventing the
    // race condition where select -> immediate click would lose the copy.
    const selectionDisposable = term.onSelectionChange(() => {
        const selection = term.getSelection();
        if (!selection) {
            return;
        }
        if (copyOnSelectTimer !== null) window.clearTimeout(copyOnSelectTimer);
        copyOnSelectTimer = window.setTimeout(() => {
            copyOnSelectTimer = null;
            if (shared.disposed) return;
            void writeClipboardText(selection).then(() => {
                clipboardWriteFailureCounter.recordSuccess();
            }).catch((err) => {
                console.warn("[terminal] copy-on-select clipboard write failed", err);
                notifyClipboardFailureThrottled(err);
            });
        }, 100);
    });

    // --- Clipboard paste ---

    const pasteClipboardText = (source: "keyboard" | "contextmenu") => {
        if (shared.disposed) return;
        // When paste occurs during IME composition, cancel the composition
        // and prioritize the paste. The paste path ultimately triggers onData
        // (via pasteTextSafely -> target.paste()), and composing=true would
        // cause imeInputGate to suppress it.
        // NOTE: finishComposition(false) resets our app-level state only.
        // The browser's native IME session remains until the next
        // compositionend event, at which point state re-synchronizes.
        // If a stale compositionend fires after cancel, markCompositionEnd runs
        // from non-composing state; harmless because the dedupe window simply
        // expires without matching any payload.
        if (shared.isComposing) {
            ime.finishComposition(false);
        }
        void ClipboardGetText()
            .then((text) => {
                if (shared.disposed) return;
                clipboardReadFailureCounter.recordSuccess();
                const pasted = pasteTextSafely(term, text);
                if (text.length > 0 && !pasted) {
                    console.warn("[terminal] paste discarded after normalization", {paneId, source, textLen: text.length});
                }
            })
            .catch((err) => {
                if (shared.disposed) return;
                console.warn(`[terminal] clipboard read failed for pane=${paneId} source=${source}`, err);
                notifyPasteFailureThrottled(err);
            });
    };

    // --- 右クリック: 選択あり->コピー / 選択なし->ペースト ---
    const termEl = term.element;
    const handleContextMenu = (e: MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        const selection = term.getSelection();
        if (selection) {
            void writeClipboardText(selection).then(() => {
                clipboardWriteFailureCounter.recordSuccess();
            }).catch((err) => {
                console.warn("[terminal] context-menu clipboard write failed", err);
                notifyClipboardFailureThrottled(err);
            });
            term.clearSelection();
        } else {
            pasteClipboardText("contextmenu");
        }
    };
    if (termEl) {
        termEl.addEventListener("contextmenu", handleContextMenu);
    }

    // --- Layer 1: Capture-phase input interception ---
    // xterm.js v6 _inputEvent processes insertText input events and fires
    // triggerDataEvent WITHOUT updating _dataAlreadySent. When WebView2 fires
    // an insertText input event after compositionend, this creates a second
    // onData that bypasses xterm.js internal dedup. Intercept these events
    // on the parent element (capture phase fires parent→child) before they
    // reach xterm's textarea listener.
    //
    // The 50ms threshold covers the synchronous insertText fired immediately
    // after compositionend (typically <5ms). Layer 2 (COMMIT_DEDUPE_WINDOW_MS
    // = 150ms) handles slower duplicates arriving via deferred setTimeout paths.
    const handleInputCapture = (e: Event): void => {
        const ev = e as InputEvent;
        if (ev.inputType === "insertText"
            && recentCompositionEndAt > 0
            && performance.now() - recentCompositionEndAt < 50) {
            e.stopImmediatePropagation();
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-ime] blocked insertText input event after compositionend",
                    {data: ev.data, elapsed: performance.now() - recentCompositionEndAt});
            }
        }
    };
    if (termEl) {
        termEl.addEventListener("input", handleInputCapture, true);
    }

    // --- Custom key handler ---

    term.attachCustomKeyEventHandler((event) => {
        // Allow xterm's internal CompositionHelper to handle IME-related events.
        // xterm's contract here is "true = let xterm process the event".
        if (shouldLetXtermHandleImeEvent(event, shared.isComposing)) {
            return true;
        }
        // Only process keydown events for shortcuts; let xterm handle keyup/keypress normally.
        if (event.type !== "keydown") {
            return true;
        }

        // Ctrl+B: tmux prefix mode
        const key = event.key.toLowerCase();

        if (event.ctrlKey && key === "b") {
            setPrefixMode(true);
            return false;
        }

        // Ctrl+F / Ctrl+Shift+F: 検索バーを開く
        if (event.ctrlKey && key === "f") {
            setSearchOpen(true);
            return false;
        }

        // Smart Ctrl+C: 選択あり->コピー、選択なし->SIGINT 送信
        if (event.ctrlKey && key === "c") {
            const selection = term.getSelection();
            if (selection) {
                void writeClipboardText(selection).then(() => {
                    clipboardWriteFailureCounter.recordSuccess();
                }).catch((err) => {
                    console.warn("[terminal] ctrl+c clipboard write failed", err);
                    notifyClipboardFailureThrottled(err);
                });
                term.clearSelection();
                return false;
            }
            return true;
        }

        // Ctrl+V: クリップボードからペースト（ブラケットペースト対応）
        if (event.ctrlKey && key === "v") {
            event.preventDefault();
            event.stopPropagation();
            pasteClipboardText("keyboard");
            return false;
        }
        return true;
    });

    // --- キー入力送信 ---
    const inputDisposable = term.onData((input) => {
        const filteredInput = ime.gate.filterInput(input);
        if (filteredInput === null || filteredInput.length === 0) {
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-ime] suppressed onData payload", {paneId, input});
            }
            return;
        }
        if (import.meta.env.DEV && filteredInput !== input) {
            console.warn("[DEBUG-ime] rewrote onData payload", {paneId, input, filteredInput});
        }
        if (syncInputModeRef.current) {
            void api.SendSyncInput(paneId, filteredInput).catch((err) => {
                // Always warn — input loss is user-visible and must be diagnosable
                // even in production builds (not gated by DEV).
                console.warn(`[terminal] sync input failed for pane=${paneId}`, err);
                notifyInputFailureThrottled(err);
            });
        } else {
            void api.SendInput(paneId, filteredInput).catch((err) => {
                // (same rationale as SendSyncInput above)
                console.warn(`[terminal] input failed for pane=${paneId}`, err);
                notifyInputFailureThrottled(err);
            });
        }
    });

    // --- Cleanup ---

    return () => {
        if (copyOnSelectTimer !== null) {
            window.clearTimeout(copyOnSelectTimer);
        }
        clearFocusRecoveryTimers();

        if (ime.compositionTextarea) {
            ime.compositionTextarea.removeEventListener("compositionstart", onCompositionStart);
            ime.compositionTextarea.removeEventListener("compositionend", onCompositionEnd);
            ime.compositionTextarea.removeEventListener("blur", onBlur);
        }
        termEl?.removeEventListener("contextmenu", handleContextMenu);
        termEl?.removeEventListener("input", handleInputCapture, true);
        window.removeEventListener("focus", onWindowFocus);
        document.removeEventListener("visibilitychange", onVisibilityChange);

        selectionDisposable.dispose();
        inputDisposable.dispose();
    };
}
