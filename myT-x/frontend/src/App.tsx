import {type CSSProperties, useCallback, useEffect, useMemo, useRef, useState} from "react";
import "@xterm/xterm/css/xterm.css";
import {api} from "./api";
import {ConfirmDialog} from "./components/ConfirmDialog";
import {MenuBar} from "./components/MenuBar";
import {QuickSearch} from "./components/QuickSearch";
import {isQuickSearchShortcut, type QuickSearchTriggerMode} from "./components/quickSearchShared";
import {SessionView} from "./components/SessionView";
import {SettingsModal} from "./components/SettingsModal";
import {Sidebar} from "./components/Sidebar";
import {ChatLayout} from "./components/ChatLayout";
import {StatusBar} from "./components/StatusBar";
import {ToastContainer} from "./components/ToastContainer";
import {ViewerSystem} from "./components/viewer";
import {
    buildDockedCssVariables,
    buildDockedLayout,
    type DockedCSSVariables,
    type DockedLayout,
    normalizeDockedViewportWidth,
} from "./components/viewer/viewerDocking";
import {useIsViewerDocked} from "./components/viewer/useIsViewerDocked";
import {useViewerStore} from "./components/viewer/viewerStore";
import {useAppImeRecovery} from "./hooks/useAppImeRecovery";
import {useBackendSync} from "./hooks/useBackendSync";
import {useFileDrop} from "./hooks/useFileDrop";
import {usePrefixKeyMode} from "./hooks/usePrefixKeyMode";
import {useI18n} from "./i18n";
import {useTmuxStore} from "./stores/tmuxStore";
import type {ValidationRules} from "./types/tmux";
import {isImeTransitionalEvent} from "./utils/ime";
import {notifyAndLog} from "./utils/notifyUtils";
import {resolveActivePane, resolveActivePaneID, resolveActiveWindow} from "./utils/session";

type AppBodyStyle = CSSProperties & Partial<DockedCSSVariables>;
function readWindowWidth(): number {
    return normalizeDockedViewportWidth(window.innerWidth);
}

function App() {
    useBackendSync();
    const {t} = useI18n();
    const [quickSearchOpen, setQuickSearchOpen] = useState(false);
    const [quickSearchTriggerMode, setQuickSearchTriggerMode] = useState<QuickSearchTriggerMode>("palette");
    const [newSessionSignal, setNewSessionSignal] = useState(0);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [validationRules, setValidationRules] = useState<ValidationRules | null>(null);
    const [windowWidth, setWindowWidth] = useState(readWindowWidth);
    const lastSyncedSessionRef = useRef<string | null>(null);
    const quickSearchTriggerRef = useRef<HTMLButtonElement>(null);

    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);
    const pendingPrefixKillPaneId = useTmuxStore((s) => s.pendingPrefixKillPaneId);
    const setPendingPrefixKillPaneId = useTmuxStore((s) => s.setPendingPrefixKillPaneId);

    const current = useMemo(
        () => sessions.find((session) => session.name === activeSession) ?? sessions[0] ?? null,
        [activeSession, sessions],
    );

    const activePaneId = useMemo(() => resolveActivePaneID(current), [current]);
    const activeWindow = useMemo(() => resolveActiveWindow(current), [current]);
    const activePane = useMemo(() => resolveActivePane(activeWindow), [activeWindow]);
    const config = useTmuxStore((s) => s.config);
    const dockRatio = useViewerStore((s) => s.dockRatio);
    const isViewerDocked = useIsViewerDocked();
    const dockedLayout = useMemo<DockedLayout | null>(() => {
        if (!isViewerDocked) {
            return null;
        }
        return buildDockedLayout(windowWidth, dockRatio);
    }, [dockRatio, isViewerDocked, windowWidth]);
    const appBodyClassName = [
        "app-body",
        isViewerDocked && "app-body--viewer-docked",
        dockedLayout?.isScaled && "app-body--viewer-scaled",
    ].filter(Boolean).join(" ");

    const appBodyStyle = useMemo<AppBodyStyle>(() => {
        if (dockedLayout === null) {
            return {};
        }
        // App only injects dock-specific variables here. Shared static widths
        // stay in :root so the portaled ActivityStrip and overlay mode keep one
        // source of truth.
        return buildDockedCssVariables(dockedLayout);
    }, [dockedLayout]);
    usePrefixKeyMode({activePaneId});
    useFileDrop(activePaneId);
    const imeRecoverySurfaceRef = useAppImeRecovery({activePaneId});
    const handleOpenSettings = useCallback(() => {
        setSettingsOpen(true);
    }, []);
    const handleCloseSettings = useCallback(() => {
        setSettingsOpen(false);
    }, []);
    const handleCloseQuickSearch = useCallback(() => {
        setQuickSearchOpen(false);
    }, []);
    const handleOpenQuickSearchFromMenuBar = useCallback(() => {
        setQuickSearchTriggerMode("dropdown");
        setQuickSearchOpen(true);
    }, []);
    const handleOpenNewSession = useCallback(() => {
        setNewSessionSignal((currentSignal) => currentSignal + 1);
    }, []);

    useEffect(() => {
        let cancelled = false;
        void api.GetValidationRules()
            .then((rules) => {
                if (!cancelled) {
                    setValidationRules(rules);
                }
            })
            .catch((err: unknown) => {
                if (!cancelled) {
                    console.warn("[app] GetValidationRules failed (non-fatal)", err);
                }
            });
        return () => {
            cancelled = true;
        };
    }, []);

    useEffect(() => {
        let resizeFrameID: number | null = null;
        const handleResize = () => {
            if (resizeFrameID !== null) {
                return;
            }
            resizeFrameID = window.requestAnimationFrame(() => {
                resizeFrameID = null;
                setWindowWidth(readWindowWidth());
            });
        };
        window.addEventListener("resize", handleResize);
        return () => {
            if (resizeFrameID !== null) {
                window.cancelAnimationFrame(resizeFrameID);
            }
            window.removeEventListener("resize", handleResize);
        };
    }, []);

    useEffect(() => {
        const currentSessionName = current?.name ?? null;
        if (currentSessionName === null) {
            lastSyncedSessionRef.current = null;
            return;
        }
        if (lastSyncedSessionRef.current === currentSessionName) {
            return;
        }
        lastSyncedSessionRef.current = currentSessionName;
        void api.SetActiveSession(currentSessionName).catch((err) => {
            lastSyncedSessionRef.current = null;
            console.warn("[app] SetActiveSession failed", err);
            notifyAndLog("Set active session", "warn", err, "App");
        });
    }, [current?.name]);

    // Ctrl+P: クイック検索パレット
    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if (isImeTransitionalEvent(e)) {
                return;
            }
            if (e.defaultPrevented) {
                return;
            }
            if (isQuickSearchShortcut(e)) {
                e.preventDefault();
                if (quickSearchOpen && quickSearchTriggerMode === "palette") {
                    setQuickSearchOpen(false);
                    return;
                }
                setQuickSearchTriggerMode("palette");
                setQuickSearchOpen(true);
            }
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [quickSearchOpen, quickSearchTriggerMode]);

    return (
        <div className="app-root">
            <MenuBar
                onOpenSettings={handleOpenSettings}
                onOpenQuickSearch={handleOpenQuickSearchFromMenuBar}
                isQuickSearchOpen={quickSearchOpen}
                quickSearchTriggerRef={quickSearchTriggerRef}
            />
            <textarea
                ref={imeRecoverySurfaceRef}
                className="ime-recovery-surface"
                data-ime-recovery-surface="true"
                tabIndex={-1}
                readOnly
                aria-hidden="true"
                spellCheck={false}
            />
            <div className={appBodyClassName} style={appBodyStyle}>
                <div className="app-body__inner">
                    <Sidebar
                        sessions={sessions}
                        activeSession={current?.name ?? null}
                        newSessionSignal={newSessionSignal}
                    />
                    <main className="main-content">
                        <ChatLayout
                            activePaneId={activePaneId}
                            activePaneTitle={activePane?.title ?? ""}
                            panes={activeWindow?.panes ?? []}
                            chatOverlayPercentage={config?.chat_overlay_percentage ?? 40}
                            validationRules={validationRules}
                        >
                            <SessionView session={current}/>
                        </ChatLayout>
                        <StatusBar/>
                    </main>
                    <ViewerSystem/>
                </div>
            </div>
            <SettingsModal open={settingsOpen} onClose={handleCloseSettings}/>
            <ToastContainer/>
            <QuickSearch
                open={quickSearchOpen}
                onClose={handleCloseQuickSearch}
                onOpenNewSession={handleOpenNewSession}
                onOpenSettings={handleOpenSettings}
                triggerMode={quickSearchTriggerMode}
                dropdownAnchorRef={quickSearchTriggerRef}
            />
            <ConfirmDialog
                open={pendingPrefixKillPaneId !== null}
                title={t("app.closePane.title", "Close pane")}
                message={
                    pendingPrefixKillPaneId
                        ? t("app.closePane.message", "Close pane \"{paneId}\"?", {paneId: pendingPrefixKillPaneId})
                        : ""
                }
                actions={[{label: t("app.closePane.action", "Close"), value: "close", variant: "danger"}]}
                onAction={(value) => {
                    if (value !== "close") {
                        setPendingPrefixKillPaneId(null);
                        return;
                    }
                    const paneID = pendingPrefixKillPaneId;
                    setPendingPrefixKillPaneId(null);
                    if (!paneID) {
                        return;
                    }
                    void api.KillPane(paneID).catch((err) => {
                        console.warn("[prefix] kill pane failed", err);
                        notifyAndLog("Close pane", "warn", err, "App");
                    });
                }}
                onClose={() => setPendingPrefixKillPaneId(null)}
            />
        </div>
    );
}

export default App;
