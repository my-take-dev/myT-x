import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";
import type {PaneSnapshot, SessionSnapshot} from "../types/tmux";
import {resolveActivePane, resolveActiveWindow} from "../utils/session";
import {notifyAndLog} from "../utils/notifyUtils";
import {useI18n} from "../i18n";
import {LayoutPresetSelector} from "./LayoutPresetSelector";
import {LayoutRenderer} from "./LayoutRenderer";
import {CanvasModeToggle} from "./canvas/CanvasModeToggle";
import {ReactFlowProvider} from "@xyflow/react";
import {CanvasView} from "./canvas/CanvasView";
import {useCanvasStore} from "../stores/canvasStore";

interface SessionViewProps {
    session: SessionSnapshot | null;
}

export function SessionView(props: SessionViewProps) {
    const {language, t} = useI18n();
    const zoomPaneId = useTmuxStore((s) => s.zoomPaneId);
    const setZoomPaneId = useTmuxStore((s) => s.setZoomPaneId);
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const syncInputMode = useTmuxStore((s) => s.syncInputMode);
    const toggleSyncInputMode = useTmuxStore((s) => s.toggleSyncInputMode);
    const canvasMode = useCanvasStore((s) => s.mode);

    const activeWindow = useMemo(() => resolveActiveWindow(props.session), [props.session]);

    const paneList = useMemo(
        () => (activeWindow ? activeWindow.panes : ([] as PaneSnapshot[])),
        [activeWindow],
    );

    const activePaneId = useMemo(
        () => resolveActivePane(activeWindow)?.id ?? null,
        [activeWindow],
    );

    useEffect(() => {
        setZoomPaneId(null);
    }, [activeWindow?.id, setZoomPaneId]);

    const onFocusPane = useCallback((paneId: string) => {
        void api.FocusPane(paneId).catch((err) => {
            console.warn("[session-view] FocusPane failed", err);
            notifyAndLog("Focus pane", "warn", err, "SessionView");
        });
    }, []);

    const onSplitVertical = useCallback((paneId: string) => {
        void api.SplitPane(paneId, true).catch((err) => {
            console.warn("[session-view] SplitPane(vertical) failed", err);
            notifyAndLog("Split pane", "warn", err, "SessionView");
        });
    }, []);

    const onSplitHorizontal = useCallback((paneId: string) => {
        void api.SplitPane(paneId, false).catch((err) => {
            console.warn("[session-view] SplitPane(horizontal) failed", err);
            notifyAndLog("Split pane", "warn", err, "SessionView");
        });
    }, []);

    const onToggleZoom = useCallback(
        (paneId: string) => {
            const current = useTmuxStore.getState().zoomPaneId;
            setZoomPaneId(current === paneId ? null : paneId);
        },
        [setZoomPaneId],
    );

    const onKillPane = useCallback((paneId: string) => {
        void api.KillPane(paneId).catch((err) => {
            console.warn("[session-view] KillPane failed", err);
            notifyAndLog("Close pane", "warn", err, "SessionView");
        });
    }, []);

    const onRenamePane = useCallback((paneId: string, title: string) => {
        void api.RenamePane(paneId, title).catch((err) => {
            console.warn("[session-view] RenamePane failed", err);
            notifyAndLog("Rename pane", "warn", err, "SessionView");
        });
    }, []);

    const onSwapPane = useCallback((sourcePaneId: string, targetPaneId: string) => {
        void api.SwapPanes(sourcePaneId, targetPaneId).catch((err) => {
            console.warn("[session-view] SwapPanes failed", err);
            notifyAndLog("Swap panes", "warn", err, "SessionView");
        });
    }, []);

    const [quickStartLoading, setQuickStartLoading] = useState(false);
    const [quickStartError, setQuickStartError] = useState("");
    const [createPaneLoading, setCreatePaneLoading] = useState(false);
    const [createPaneError, setCreatePaneError] = useState("");

    const mountedRef = useRef(true);
    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

    const handleQuickStart = useCallback(async () => {
        setQuickStartLoading(true);
        setQuickStartError("");
        try {
            const snapshot = await api.QuickStartSession();
            if (!mountedRef.current) return;
            setActiveSession(snapshot.name);
        } catch (err) {
            if (!mountedRef.current) return;
            const message = err instanceof Error
                ? err.message
                : String(err ?? t("sessionView.error.unknown", "Unknown error"));
            setQuickStartError(message);
        } finally {
            if (mountedRef.current) {
                setQuickStartLoading(false);
            }
        }
    }, [language, setActiveSession, t]);

    const handleCreatePane = useCallback(async () => {
        const sessionName = props.session?.name?.trim() ?? "";
        if (sessionName === "") {
            return;
        }
        setCreatePaneLoading(true);
        setCreatePaneError("");
        try {
            await api.CreatePaneInSession(sessionName);
        } catch (err) {
            if (!mountedRef.current) return;
            const message = err instanceof Error
                ? err.message
                : String(err ?? t("sessionView.error.unknown", "Unknown error"));
            setCreatePaneError(message);
        } finally {
            if (mountedRef.current) {
                setCreatePaneLoading(false);
            }
        }
    }, [language, props.session?.name, t]);

    const onDetachSession = useCallback(() => {
        const sessionName = props.session?.name;
        if (!sessionName) {
            return;
        }
        void api.DetachSession(sessionName).catch((err) => {
            console.warn("[session-view] DetachSession failed", err);
            notifyAndLog("Detach session", "error", err, "SessionView");
        });
    }, [props.session?.name]);

    const renderSessionContent = () => {
        if (!props.session) {
            return (
                <div className="session-empty">
                    <div className="session-empty-content">
                        <p className="session-empty-message">
                            {language === "en"
                                ? "Create a session to get started."
                                : t("sessionView.empty.createSession", "セッションを作成してください。")}
                        </p>
                        <button
                            type="button"
                            className="session-quick-start-btn"
                            onClick={handleQuickStart}
                            disabled={quickStartLoading}
                        >
                            {quickStartLoading
                                ? (language === "en"
                                    ? "Starting..."
                                    : t("sessionView.quickStart.loading", "開始中..."))
                                : (language === "en"
                                    ? "▶ Quick Start"
                                    : t("sessionView.quickStart.button", "▶ クイックスタート"))}
                        </button>
                        {quickStartError && (
                            <p className="session-quick-start-error">{quickStartError}</p>
                        )}
                    </div>
                </div>
            );
        }
        if (props.session.windows.length === 0) {
            return (
                <div className="session-empty">
                    <div className="session-empty-content">
                        <p className="session-empty-message">
                            {language === "en"
                                ? "All panes were closed."
                                : t("sessionView.empty.allPanesClosed", "全てのペインが閉じられました。")}
                        </p>
                        <button
                            type="button"
                            className="session-quick-start-btn"
                            onClick={handleCreatePane}
                            disabled={createPaneLoading}
                        >
                            {createPaneLoading
                                ? (language === "en"
                                    ? "Creating..."
                                    : t("sessionView.createPane.loading", "作成中..."))
                                : (language === "en"
                                    ? "+ New Pane"
                                    : t("sessionView.createPane.button", "+ 新しいペイン"))}
                        </button>
                        {createPaneError && (
                            <p className="session-quick-start-error">{createPaneError}</p>
                        )}
                    </div>
                </div>
            );
        }
        if (!activeWindow) {
            return (
                <div className="session-empty">
                    {language === "en"
                        ? "No active window."
                        : t("sessionView.empty.noActiveWindow", "アクティブウィンドウがありません。")}
                </div>
            );
        }

        return (
            <>
                <div className="session-view-header">
                    <LayoutPresetSelector
                        sessionName={props.session.name}
                        paneCount={paneList.length}
                    />
                    <CanvasModeToggle/>
                    {paneList.length >= 2 && (
                        <button
                            type="button"
                            className={`terminal-toolbar-btn sync-toggle-btn ${syncInputMode ? "sync-active" : ""}`}
                            title={
                                language === "en"
                                    ? "Sync input mode (Prefix: s)"
                                    : t("sessionView.syncMode.title", "同期入力モード (Prefix: s)")
                            }
                            aria-label={
                                language === "en"
                                    ? "Toggle sync input mode"
                                    : t("sessionView.syncMode.aria", "Toggle sync input mode")
                            }
                            onClick={toggleSyncInputMode}
                        >
                            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor"
                                 strokeWidth="1.4">
                                <path d="M2 5h4l-2-3M12 9H8l2 3"/>
                                <path d="M2 5c0 3.3 2.7 6 6 6M12 9c0-3.3-2.7-6-6-6"/>
                            </svg>
                            <span className="sync-toggle-label">
                                {language === "en"
                                    ? "Sync"
                                    : t("sessionView.syncMode.label", "Sync")}
                            </span>
                        </button>
                    )}
                </div>
                <div className="session-view-body">
                    {canvasMode === "canvas" ? (
                        <ReactFlowProvider>
                            <CanvasView
                                panes={activeWindow.panes}
                                activePaneId={activePaneId}
                                sessionName={props.session.name}
                                onFocusPane={onFocusPane}
                                onSplitVertical={onSplitVertical}
                                onSplitHorizontal={onSplitHorizontal}
                                onToggleZoom={onToggleZoom}
                                onKillPane={onKillPane}
                                onRenamePane={onRenamePane}
                                onSwapPane={onSwapPane}
                                onDetachSession={onDetachSession}
                            />
                        </ReactFlowProvider>
                    ) : (
                        <LayoutRenderer
                            layout={activeWindow.layout ?? null}
                            panes={activeWindow.panes}
                            activePaneId={activePaneId}
                            zoomPaneId={zoomPaneId}
                            onFocusPane={onFocusPane}
                            onSplitVertical={onSplitVertical}
                            onSplitHorizontal={onSplitHorizontal}
                            onToggleZoom={onToggleZoom}
                            onKillPane={onKillPane}
                            onRenamePane={onRenamePane}
                            onSwapPane={onSwapPane}
                            onDetachSession={onDetachSession}
                        />
                    )}
                </div>
            </>
        );
    };

    return (
        <div className="session-view">
            {renderSessionContent()}
        </div>
    );
}
