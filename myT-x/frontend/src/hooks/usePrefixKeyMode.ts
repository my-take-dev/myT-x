import {useEffect, useRef} from "react";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";
import {isImeTransitionalEvent} from "../utils/ime";
import {resolveActiveWindow} from "../utils/session";

interface UsePrefixKeyModeOptions {
    activePaneId: string | null;
}

export function usePrefixKeyMode(options: UsePrefixKeyModeOptions) {
    // I-35: ハンドラ内で useTmuxStore.getState() を直接参照することで
    // 個別 useRef+useEffect による Ref 同期パターンを廃止する。
    // stale closure を避けつつ React の再レンダリングコストも削減できる。
    const setPrefixMode = useTmuxStore((s) => s.setPrefixMode);
    const setZoomPaneId = useTmuxStore((s) => s.setZoomPaneId);
    const setPendingPrefixKillPaneId = useTmuxStore((s) => s.setPendingPrefixKillPaneId);
    const toggleSyncInputMode = useTmuxStore((s) => s.toggleSyncInputMode);
    // prefixMode は タイマーコールバック内でも最新値が必要なため Ref で保持する。
    // getState() 呼び出し時点の値が常に最新になるが、timer callback は
    // getState() を呼ぶため Ref は prefixMode のみに限定する。
    const prefixModeRef = useRef(false);
    const timerRef = useRef<number | null>(null);

    useEffect(() => {
        const clearPrefixTimer = () => {
            if (timerRef.current !== null) {
                window.clearTimeout(timerRef.current);
                timerRef.current = null;
            }
        };

        const armPrefixTimer = () => {
            clearPrefixTimer();
            timerRef.current = window.setTimeout(() => {
                prefixModeRef.current = false;
                setPrefixMode(false);
            }, 1200);
        };

        const handle = (event: KeyboardEvent) => {
            if (isImeTransitionalEvent(event)) {
                return;
            }
            if (event.ctrlKey && (event.key === "b" || event.key === "B")) {
                event.preventDefault();
                prefixModeRef.current = true;
                setPrefixMode(true);
                armPrefixTimer();
                return;
            }

            if (!prefixModeRef.current) {
                return;
            }

            event.preventDefault();
            clearPrefixTimer();
            prefixModeRef.current = false;
            setPrefixMode(false);

            const paneId = options.activePaneId;
            if (!paneId) {
                return;
            }

            // I-35: ストア最新値をハンドラ内で直接取得することで Ref 同期不要にする。
            const {sessions, activeSession, zoomPaneId} = useTmuxStore.getState();

            const currentSession = sessions.find(
                (session) => session.name === activeSession,
            );
            const windows = currentSession?.windows ?? [];
            // I-8: resolveActiveWindow を1回だけ呼び出して変数にキャッシュする。
            const activeWindow = resolveActiveWindow(currentSession);
            const activeWindowIndex = activeWindow
                ? windows.findIndex((window) => window.id === activeWindow.id)
                : -1;
            const panes = activeWindow?.panes ?? [];
            // S-28: currentIndex === -1 means the active pane was not found in the
            // current window's pane list. Arrow key handlers gracefully fall back to
            // index 0 (first pane) via Math.max/Math.min clamping.
            const currentIndex = panes.findIndex((pane) => pane.id === paneId);
            const focusWindowAt = (windowIndex: number) => {
                if (windowIndex < 0 || windowIndex >= windows.length) {
                    return;
                }
                const targetWindow = windows[windowIndex];
                if (!targetWindow) {
                    return;
                }
                const targetPane = targetWindow.panes.find((pane) => pane.active) ?? targetWindow.panes[0];
                if (!targetPane) {
                    return;
                }
                void api.FocusPane(targetPane.id).catch((err) => {
                    console.warn("[prefix] focus window failed", err);
                });
            };

            const key = event.key;
            const lowerKey = key.toLowerCase();
            if (key === "%") {
                void api.SplitPane(paneId, true).catch((err) => {
                    console.warn("[prefix] split vertical failed", err);
                });
                return;
            }
            if (key === '"') {
                void api.SplitPane(paneId, false).catch((err) => {
                    console.warn("[prefix] split horizontal failed", err);
                });
                return;
            }
            if (lowerKey === "z") {
                setZoomPaneId(zoomPaneId === paneId ? null : paneId);
                return;
            }
            if (lowerKey === "s") {
                toggleSyncInputMode();
                return;
            }
            // NOTE: Prefix+c (new-window) は タブUI削除に伴い無効化。
            // myT-x では new-window = 子セッション（child session）作成であり、
            // tmux標準の new-window（同一セッション内にウィンドウ追加）とは異なる。
            // 新セッション作成は NewSessionModal から行う。
            if (lowerKey === "c") {
                return;
            }
            if (lowerKey === "n") {
                if (windows.length === 0 || activeWindowIndex < 0) {
                    return;
                }
                focusWindowAt((activeWindowIndex + 1) % windows.length);
                return;
            }
            if (lowerKey === "p") {
                if (windows.length === 0 || activeWindowIndex < 0) {
                    return;
                }
                focusWindowAt((activeWindowIndex - 1 + windows.length) % windows.length);
                return;
            }
            if (lowerKey === "x") {
                setPendingPrefixKillPaneId(paneId);
                return;
            }
            if (lowerKey === "d") {
                if (activeSession) {
                    void api.DetachSession(activeSession).catch((err) => {
                        console.warn("[prefix] detach session failed", err);
                    });
                }
                return;
            }
            if ((key === "ArrowLeft" || key === "ArrowUp") && panes.length > 0) {
                const nextIndex = Math.max(0, currentIndex - 1);
                const target = panes[nextIndex];
                if (target) {
                    void api.FocusPane(target.id).catch((err) => {
                        console.warn("[prefix] focus pane failed", err);
                    });
                }
                return;
            }
            if ((key === "ArrowRight" || key === "ArrowDown") && panes.length > 0) {
                const nextIndex = Math.min(panes.length - 1, currentIndex + 1);
                const target = panes[nextIndex];
                if (target) {
                    void api.FocusPane(target.id).catch((err) => {
                        console.warn("[prefix] focus pane failed", err);
                    });
                }
                return;
            }
        };

        window.addEventListener("keydown", handle);
        return () => {
            window.removeEventListener("keydown", handle);
            clearPrefixTimer();
        };
    }, [options.activePaneId, setPendingPrefixKillPaneId, setPrefixMode, setZoomPaneId, toggleSyncInputMode]);
}
