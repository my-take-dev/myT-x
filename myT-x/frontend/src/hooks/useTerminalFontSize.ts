import {type MutableRefObject, useEffect} from "react";
import type {FitAddon} from "@xterm/addon-fit";
import type {Terminal} from "@xterm/xterm";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";

interface UseTerminalFontSizeOptions {
    paneId: string;
    containerRef: MutableRefObject<HTMLDivElement | null>;
    terminalRef: MutableRefObject<Terminal | null>;
    fitAddonRef: MutableRefObject<FitAddon | null>;
    /**
     * フォントサイズの最新コミット値を同期的に追跡する ref。
     * ホイールハンドラは長寿命クロージャ内で動作するため、
     * React の再レンダリングを待たずに最新値を参照できるよう ref で管理する。
     */
    fontSizeRef: MutableRefObject<number>;
}

/**
 * Ctrl+ホイールによるフォントサイズ変更と、ストア値が変化した際の
 * Terminal への反映を担う。
 *
 * fontSize 変更後に fitAddon.fit() を呼び、さらに ResizePane を
 * 明示的に呼び出す。fitAddon.fit() 後は DOM サイズが変わらないため
 * ResizeObserver が発火しないケースがあり、バックエンドに新しい
 * cols/rows を確実に伝えるため直接 API を呼び出す。
 *
 * INVARIANT: useTerminalSetup must be called before this hook so that
 * terminalRef.current is populated. See TerminalPane.tsx for the full
 * hook ordering contract.
 *
 * WARNING: Hook call order matters — do not reorder the hook calls in TerminalPane.tsx
 * without reviewing cross-hook dependencies.
 */
export function useTerminalFontSize({
                                        paneId,
                                        containerRef,
                                        terminalRef,
                                        fitAddonRef,
                                        fontSizeRef,
                                    }: UseTerminalFontSizeOptions): void {
    const fontSize = useTmuxStore((s) => s.fontSize);
    const setFontSize = useTmuxStore((s) => s.setFontSize);

    // --- Ctrl+ホイール: フォントサイズ変更 ---
    useEffect(() => {
        const mountEl = containerRef.current;
        if (!mountEl) {
            return;
        }

        let fontSizeTimer: ReturnType<typeof window.setTimeout> | null = null;
        let pendingFontSize: number | null = null;

        const handleWheel = (e: WheelEvent) => {
            if (!e.ctrlKey) return;
            e.preventDefault();
            const delta = e.deltaY < 0 ? 1 : -1;
            // fontSizeRef.current を使うことで、React の再レンダリング完了前でも
            // 最新のコミット済みフォントサイズを基準に計算できる。
            const base: number = pendingFontSize ?? fontSizeRef.current;
            pendingFontSize = Math.max(8, Math.min(32, base + delta));
            if (fontSizeTimer !== null) {
                window.clearTimeout(fontSizeTimer);
            }
            fontSizeTimer = window.setTimeout(() => {
                fontSizeTimer = null;
                if (pendingFontSize === null) {
                    return;
                }
                const newSize = pendingFontSize;
                pendingFontSize = null;
                // fontSizeRef を同期更新することで、次のホイールイベントが
                // React の useEffect 適用前でも正確な base 値を参照できる。
                fontSizeRef.current = newSize;
                setFontSize(newSize);
            }, 50);
        };

        mountEl.addEventListener("wheel", handleWheel, {passive: false});

        return () => {
            if (fontSizeTimer !== null) {
                window.clearTimeout(fontSizeTimer);
            }
            pendingFontSize = null;
            mountEl.removeEventListener("wheel", handleWheel);
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps -- containerRef, fontSizeRef, and setFontSize are stable refs/callbacks that never change identity
    }, [paneId]);

    // --- ストア値変化を Terminal に反映 ---
    // fontSizeRef は React の再レンダリングより先に更新することで、
    // ホイールハンドラが次のイベント時に陳腐化した値を使わないようにする。
    useEffect(() => {
        const term = terminalRef.current;
        if (!term) return;
        fontSizeRef.current = fontSize;
        term.options.fontSize = fontSize;

        const fitAddon = fitAddonRef.current;
        if (!fitAddon) return;
        fitAddon.fit();

        // fitAddon.fit() 後は DOM サイズ変更がないため ResizeObserver が
        // 発火しない可能性がある。フォントサイズ変更で cols/rows が変化した
        // 場合にバックエンドへ確実に通知するため明示的に API を呼ぶ。
        void api.ResizePane(paneId, term.cols, term.rows).catch((err) => {
            console.warn(`[DEBUG-terminal] fontSize-resize failed for pane=${paneId}`, err);
        });
        // terminalRef, fitAddonRef, fontSizeRef are MutableRefObject whose identity
        // is stable across renders. Including them in deps would have no effect since
        // React's shallow comparison will always see the same object reference.
        // eslint-disable-next-line react-hooks/exhaustive-deps -- terminalRef, fitAddonRef, fontSizeRef are stable MutableRefObject refs
    }, [fontSize, paneId]);
}
