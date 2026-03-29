import {type Dispatch, type MutableRefObject, type SetStateAction, useEffect} from "react";
import type {Terminal} from "@xterm/xterm";
import {createTerminalImeInputGate} from "../utils/terminalIme";
import {useTmuxStore} from "../stores/tmuxStore";
import {
    isTerminalImeRecoveryEvent,
    TERMINAL_IME_RECOVERY_EVENT,
    type TerminalImeRecoveryReason,
} from "../utils/imeRecovery";
import type {TerminalEventShared} from "./useTerminalKeyHandler";
import {setupKeyHandler} from "./useTerminalKeyHandler";
import {setupPaneDataStream} from "./useTerminalPaneData";

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

        // --- Shared mutable state for the useEffect lifetime ---
        const shared: TerminalEventShared = {
            disposed: false,
            // M-06: Skip rendering when page tab is hidden. Backend continues processing
            // pane output; rendering resumes automatically when the tab becomes visible
            // and the next pane:data event arrives.
            pageHidden: document.hidden,
            isComposing: false,
            composingOutput: [],
        };

        // --- IME composition core ---
        const imeInputGate = createTerminalImeInputGate();
        const compositionTextarea = term.textarea ?? null;

        const flushComposedOutput = () => {
            if (shared.composingOutput.length === 0) {
                return;
            }
            const buffered = shared.composingOutput.join("");
            shared.composingOutput.length = 0;
            // useTerminalSetup のクリーンアップが先に実行され term.dispose() が
            // 呼ばれた後にこの関数が到達する競合窓がある。try-catch で安全に吸収する。
            try {
                term.write(buffered);
            } catch (err) {
                console.warn("[terminal] flushComposedOutput failed (terminal may be disposed)", err);
            }
        };

        const finishComposition = (commitEnded: boolean, eventData: string = "") => {
            shared.isComposing = false;
            isComposingRef.current = false;
            try {
                if (commitEnded) {
                    imeInputGate.markCompositionEnd(eventData);
                } else {
                    imeInputGate.cancelComposition();
                }
            } finally {
                flushComposedOutput();
            }
        };

        // --- Setup sub-systems ---

        const keyCleanup = setupKeyHandler({
            term,
            shared,
            ime: {gate: imeInputGate, compositionTextarea, finishComposition},
            paneId,
            isComposingRef,
            syncInputModeRef,
            setSearchOpen,
            setPrefixMode,
        });

        const paneDataCleanup = setupPaneDataStream({term, shared, paneId});

        // --- スクロール位置インジケータ ---
        let scrollWriteTimer: ReturnType<typeof window.setTimeout> | null = null;

        const updateScrollState = () => {
            if (shared.disposed) return;
            const buf = term.buffer.active;
            const atBottom = buf.viewportY >= buf.baseY;
            setScrollAtBottom(atBottom);
        };
        const scheduleScrollWriteUpdate = () => {
            if (shared.disposed || scrollWriteTimer !== null) {
                return;
            }
            scrollWriteTimer = window.setTimeout(() => {
                scrollWriteTimer = null;
                updateScrollState();
            }, 100);
        };
        const scrollDisposable = term.onScroll(updateScrollState);
        const writeDisposable = term.onWriteParsed(scheduleScrollWriteUpdate);

        // --- IME recovery (global reset signal from MenuBar) ---
        let imeResetTimer: ReturnType<typeof window.setTimeout> | null = null;

        const resetIme = (reason: TerminalImeRecoveryReason): void => {
            if (imeResetTimer !== null) {
                window.clearTimeout(imeResetTimer);
                imeResetTimer = null;
            }
            const wasComposing = shared.isComposing;
            if (shared.isComposing) {
                finishComposition(false);
            }
            imeInputGate.dispose();
            try {
                term.blur();
            } catch {
                // Terminal may already be disposed
            }
            imeResetTimer = window.setTimeout(() => {
                imeResetTimer = null;
                if (shared.disposed) return;
                try {
                    term.focus();
                } catch {
                    // Terminal may already be disposed
                }
            }, 100);
            console.warn("[IME-RECOVERY] reset executed", {paneId, reason, wasComposing});
        };

        const handleTerminalImeRecovery = (event: Event): void => {
            if (shared.disposed || !isTerminalImeRecoveryEvent(event) || event.detail.paneId !== paneId) {
                return;
            }
            resetIme(event.detail.reason);
        };
        window.addEventListener(TERMINAL_IME_RECOVERY_EVENT, handleTerminalImeRecovery);

        // --- Cleanup ---

        return () => {
            // 1. Signal disposal first — async callbacks in sub-systems
            //    check this flag to short-circuit, preventing writes to a
            //    disposed terminal or stale state updates.
            shared.disposed = true;

            // 2. Tear down sub-systems (order is not critical since
            //    disposed=true already prevents cross-system interactions).
            keyCleanup();
            paneDataCleanup();

            if (scrollWriteTimer !== null) {
                window.clearTimeout(scrollWriteTimer);
            }
            scrollDisposable.dispose();
            writeDisposable.dispose();

            // IME reset listener + pending timer
            window.removeEventListener(TERMINAL_IME_RECOVERY_EVENT, handleTerminalImeRecovery);
            if (imeResetTimer !== null) {
                window.clearTimeout(imeResetTimer);
            }

            // Final IME state cleanup
            shared.isComposing = false;
            isComposingRef.current = false;
            shared.composingOutput.length = 0;
            imeInputGate.dispose();
        };
        // Zustand store actions (setPrefixMode) and React setState dispatchers
        // (setSearchOpen, setScrollAtBottom) are stable references across renders.
        // Including them in the dependency array would cause no functional change
        // but would generate a false ESLint warning about missing deps.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [paneId]);
}
