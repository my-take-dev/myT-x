import {type MutableRefObject, useEffect} from "react";
import type {FitAddon} from "@xterm/addon-fit";
import type {Terminal} from "@xterm/xterm";
import {api} from "../api";

interface UseTerminalResizeOptions {
    paneId: string;
    containerRef: MutableRefObject<HTMLDivElement | null>;
    terminalRef: MutableRefObject<Terminal | null>;
    fitAddonRef: MutableRefObject<FitAddon | null>;
    /**
     * IME composition 中かどうかを示す共有 ref。
     * useTerminalEvents が compositionstart/compositionend を監視して更新し、
     * 本フックはリサイズ保留判定のために参照のみ行う。
     */
    isComposingRef: MutableRefObject<boolean>;
}

/**
 * ResizeObserver による DOM サイズ変動検知と、Ctrl+ホイールによる
 * フォントサイズ変更は useTerminalFontSize が担うため、こちらは
 * コンテナリサイズのみを担当する。
 *
 * S-09: Implicit hook call order dependency:
 *   - useTerminalSetup must run first to populate terminalRef.current.
 *   - useTerminalEvents must run first to register compositionstart/compositionend
 *     listeners that update isComposingRef. This hook reads isComposingRef to
 *     defer ResizeObserver callbacks during IME composition.
 *   - Both hooks share the same [paneId] dependency, so their effects execute
 *     in declaration order within TerminalPane.tsx. React guarantees this order.
 *   See TerminalPane.tsx for the full hook ordering contract.
 *
 * WARNING: Hook call order matters — do not reorder the hook calls in TerminalPane.tsx
 * without reviewing cross-hook dependencies.
 *
 * 依存配列: [paneId] — ペインが変わるたびに再登録する。
 */
export function useTerminalResize({
                                      paneId,
                                      containerRef,
                                      terminalRef,
                                      fitAddonRef,
                                      isComposingRef,
                                  }: UseTerminalResizeOptions): void {
    useEffect(() => {
        let disposed = false;
        let resizeTimer: ReturnType<typeof window.setTimeout> | null = null;
        let pendingResize = false;
        let lastResizeCols = -1;
        let lastResizeRows = -1;

        const flushResize = () => {
            const term = terminalRef.current;
            const fitAddon = fitAddonRef.current;
            if (!term || !fitAddon) {
                return;
            }
            fitAddon.fit();
            if (term.cols === lastResizeCols && term.rows === lastResizeRows) {
                return;
            }
            lastResizeCols = term.cols;
            lastResizeRows = term.rows;
            void api.ResizePane(paneId, term.cols, term.rows).catch((err) => {
                console.warn(`[DEBUG-terminal] resize failed for pane=${paneId}`, err);
            });
        };

        const scheduleResize = () => {
            // IME composition 中はリサイズを保留する。
            // isComposingRef は useTerminalEvents が compositionstart/compositionend で更新する。
            if (isComposingRef.current) {
                pendingResize = true;
                return;
            }
            if (resizeTimer !== null) {
                window.clearTimeout(resizeTimer);
            }
            resizeTimer = window.setTimeout(() => {
                resizeTimer = null;
                if (disposed) {
                    return;
                }
                // composition 終了後に保留リサイズをフラッシュ
                if (pendingResize) {
                    pendingResize = false;
                }
                flushResize();
            }, 100);
        };

        // IME composition 終了後に保留リサイズを即座にフラッシュするハンドラ。
        // useTerminalEvents が compositionend で isComposingRef.current = false に
        // 設定した後、同イベントがここにも伝播する。ResizeObserver の次回発火を
        // 待たずに即座にリサイズを反映する。
        const onCompositionEnd = () => {
            if (!pendingResize || disposed) {
                return;
            }
            pendingResize = false;
            // compositionend 直後は IME UI がまだ閉じきっていない場合があるため
            // 1フレーム遅延させてからフラッシュする。
            if (resizeTimer !== null) {
                window.clearTimeout(resizeTimer);
            }
            resizeTimer = window.setTimeout(() => {
                resizeTimer = null;
                if (disposed) {
                    return;
                }
                flushResize();
            }, 100);
        };

        // compositionend は Terminal の textarea 上で発火するため、
        // terminalRef.current.textarea を監視する。
        const compositionTarget = terminalRef.current?.textarea ?? null;
        if (compositionTarget) {
            compositionTarget.addEventListener("compositionend", onCompositionEnd);
        }

        const mountEl = containerRef.current;
        const observer = new ResizeObserver(() => {
            scheduleResize();
        });
        if (mountEl) {
            observer.observe(mountEl);
        }

        return () => {
            disposed = true;
            if (resizeTimer !== null) {
                window.clearTimeout(resizeTimer);
            }
            pendingResize = false;
            if (compositionTarget) {
                compositionTarget.removeEventListener("compositionend", onCompositionEnd);
            }
            observer.disconnect();
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps -- isComposingRef is a stable ref that never changes identity
    }, [paneId]);
}
