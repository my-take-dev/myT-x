import {type MutableRefObject, useEffect} from "react";
import {FitAddon} from "@xterm/addon-fit";
import {SearchAddon} from "@xterm/addon-search";
import {WebLinksAddon} from "@xterm/addon-web-links";
import {Terminal} from "@xterm/xterm";
import {BrowserOpenURL} from "../../wailsjs/runtime/runtime";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";

// ---------------------------------------------------------------------------
// webglUnavailable: モジュールスコープのフラグ
//
// このフラグはモジュール全体で1つだけ保持される設計。理由:
//  - WebGL コンテキストロスト（onContextLoss）が一度発生すると、同一ブラウザ
//    プロセス内では GPU リソースの枯渇が継続するケースが多い。
//  - ペインごとに個別フラグを持つと再試行を繰り返し、パフォーマンスが低下する。
//  - React の HMR（Hot Module Replacement）環境では Module スコープはリロード時に
//    リセットされるため、開発時の誤検知も最小化される。
// ---------------------------------------------------------------------------
// Module-scope WebGL availability flag. Set to true on context loss or
// load failure. Unlike per-pane tracking, a single flag prevents repeated
// retry storms across all panes. Resets after WEBGL_RETRY_DELAY_MS to
// allow recovery from transient GPU resource exhaustion.
let webglUnavailable = false;
let webglUnavailableSince: number | null = null;

// 30 seconds: allows GPU driver recovery from transient context loss
// while preventing immediate retry storms. Only applies to newly opened
// panes; existing panes keep their DOM renderer fallback.
const WEBGL_RETRY_DELAY_MS = 30_000;

function shouldAttemptWebgl(): boolean {
    if (!webglUnavailable) return true;
    if (
        webglUnavailableSince !== null &&
        Date.now() - webglUnavailableSince >= WEBGL_RETRY_DELAY_MS
    ) {
        // Reset: allow next pane to attempt WebGL again.
        webglUnavailable = false;
        webglUnavailableSince = null;
        return true;
    }
    return false;
}

interface UseTerminalSetupOptions {
    paneId: string;
    containerRef: MutableRefObject<HTMLDivElement | null>;
    terminalRef: MutableRefObject<Terminal | null>;
    searchAddonRef: MutableRefObject<SearchAddon | null>;
    fitAddonRef: MutableRefObject<FitAddon | null>;
}

/**
 * Terminal インスタンスの生成・addon 読み込み・WebGL 初期化・
 * リプレイ取得を担う。クリーンアップ時に全リソースを解放する。
 *
 * INVARIANT: This hook MUST be called before useTerminalEvents, useTerminalResize,
 * and useTerminalFontSize in the parent component. Those hooks read terminalRef.current
 * which this hook populates. See TerminalPane.tsx for the full hook ordering contract.
 *
 * WARNING: Hook call order matters — do not reorder the hook calls in TerminalPane.tsx
 * without reviewing cross-hook dependencies. React guarantees effects with the same
 * dependency array execute in declaration order, and this ordering is load-bearing.
 * There is no lint rule to enforce this; the contract is purely by convention.
 *
 * 依存配列: [paneId] — paneId が変わるたびに Terminal を再生成する。
 * fontSize はエフェクト実行時にストアから直接取得する。以後の変更は useTerminalFontSize が担う。
 */
export function useTerminalSetup({
                                     paneId,
                                     containerRef,
                                     terminalRef,
                                     searchAddonRef,
                                     fitAddonRef,
                                 }: UseTerminalSetupOptions): void {
    useEffect(() => {
        // fontSize はエフェクト実行時の最新値をストアから直接取得する。
        // 依存配列が [paneId] のみのため、パラメータの fontSize はクロージャに
        // キャプチャされた時点の値が陳腐化する可能性がある。
        const currentFontSize = useTmuxStore.getState().fontSize;

        const term = new Terminal({
            convertEol: true,
            cursorBlink: true,
            // 5,000 lines scrollback: balances scroll history usability against memory.
            // At 10 panes × ~200 B/cell × 80 cols × 5,000 rows ≈ 8 MB total.
            // Reduced from 10,000 to halve xterm.js internal CircularList overhead.
            scrollback: 5000,
            scrollSensitivity: 1,
            fastScrollSensitivity: 5,
            fontFamily: `"Consolas", "JetBrains Mono", monospace`,
            fontSize: currentFontSize,
            theme: {
                background: "#0f1b2b",
                foreground: "#dce8f4",
                cursor: "#f6d365",
                selectionBackground: "rgba(246,211,101,0.3)",
            },
            allowProposedApi: true,
        });

        const fitAddon = new FitAddon();
        const searchAddon = new SearchAddon();
        const webLinksAddon = new WebLinksAddon((_event, uri) => {
            BrowserOpenURL(uri);
        });

        term.loadAddon(fitAddon);
        term.loadAddon(searchAddon);
        term.loadAddon(webLinksAddon);

        terminalRef.current = term;
        searchAddonRef.current = searchAddon;
        fitAddonRef.current = fitAddon;

        let disposed = false;
        let rendererAddon: { dispose: () => void } | null = null;

        const setRendererMode = (next: "webgl" | "dom") => {
            if (import.meta.env.DEV) {
                console.debug(`[DEBUG-terminal-renderer] pane=${paneId} renderer=${next}`);
            }
        };

        if (containerRef.current) {
            term.open(containerRef.current);
            fitAddon.fit();
            term.focus();
            // I-27: Notify backend of the initial terminal size after first fit.
            // Without this call, the backend uses the default 120x40 size.
            void api.ResizePane(paneId, term.cols, term.rows).catch((err) => {
                if (import.meta.env.DEV) {
                    console.warn(`[DEBUG-terminal] initial ResizePane failed for pane=${paneId}`, err);
                }
            });
        }

        // WebGL addon を非同期ロード。
        // disposed チェックと loadAddon の間に微小な競合窓があるため
        // try-catch で disposed 後の操作エラーを安全に吸収する。
        if (shouldAttemptWebgl()) {
            void import("@xterm/addon-webgl")
                .then(({WebglAddon}) => {
                    // 二重チェック: import 完了前に disposed になったケースを弾く
                    if (disposed || webglUnavailable) {
                        return;
                    }
                    try {
                        const addon = new WebglAddon();
                        // disposed 直後に loadAddon が呼ばれると addon 内部でエラーになる
                        // ことがある。try-catch で吸収しフォールバックする。
                        term.loadAddon(addon);
                        rendererAddon = addon;
                        setRendererMode("webgl");
                        addon.onContextLoss(() => {
                            webglUnavailable = true;
                            webglUnavailableSince = Date.now();
                            rendererAddon = null;
                            addon.dispose();
                            setRendererMode("dom");
                            term.refresh(0, term.rows - 1);
                        });
                    } catch (err) {
                        if (import.meta.env.DEV) {
                            console.warn(`[DEBUG-terminal-renderer] WebGL loadAddon failed for pane=${paneId}`, err);
                        }
                        webglUnavailable = true;
                        webglUnavailableSince = Date.now();
                    }
                })
                .catch((err) => {
                    if (import.meta.env.DEV) {
                        console.warn(`[DEBUG-terminal-renderer] WebGL addon import failed for pane=${paneId}`, err);
                    }
                    webglUnavailable = true;
                    webglUnavailableSince = Date.now();
                });
        }

        void api.GetPaneReplay(paneId)
            .then((replay) => {
                if (disposed || !replay) return;
                // term.dispose() が .then() 到達前に呼ばれると term.write() が
                // 例外を投げる場合がある。disposed フラグは上でチェック済みだが、
                // 非同期のため微小な競合窓が残る。try-catch で安全に吸収する。
                try {
                    term.write(replay);
                } catch (err) {
                    if (import.meta.env.DEV) {
                        console.warn(`[DEBUG-terminal] replay write failed (terminal may be disposed) for pane=${paneId}`, err);
                    }
                }
            })
            .catch((err) => {
                if (import.meta.env.DEV) {
                    console.warn(`[DEBUG-terminal] replay load failed for pane=${paneId}`, err);
                }
            });

        return () => {
            disposed = true;
            rendererAddon?.dispose();
            term.dispose();
            terminalRef.current = null;
            searchAddonRef.current = null;
            fitAddonRef.current = null;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [paneId]);
}
