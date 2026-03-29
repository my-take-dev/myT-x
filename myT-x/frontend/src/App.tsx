import {type CSSProperties, useEffect, useMemo, useRef, useState} from "react";
import "@xterm/xterm/css/xterm.css";
import {api} from "./api";
import {ConfirmDialog} from "./components/ConfirmDialog";
import {MenuBar} from "./components/MenuBar";
import {QuickSearch} from "./components/QuickSearch";
import {SessionView} from "./components/SessionView";
import {SettingsModal} from "./components/SettingsModal";
import {Sidebar} from "./components/Sidebar";
import {ChatInputBar} from "./components/ChatInputBar";
import {StatusBar} from "./components/StatusBar";
import {ToastContainer} from "./components/ToastContainer";
import {ViewerSystem} from "./components/viewer";
import {useIsViewerDocked} from "./components/viewer/useIsViewerDocked";
import {useViewerStore} from "./components/viewer/viewerStore";
import {useAppImeRecovery} from "./hooks/useAppImeRecovery";
import {useBackendSync} from "./hooks/useBackendSync";
import {useFileDrop} from "./hooks/useFileDrop";
import {usePrefixKeyMode} from "./hooks/usePrefixKeyMode";
import {useI18n} from "./i18n";
import {useTmuxStore} from "./stores/tmuxStore";
import {isImeTransitionalEvent} from "./utils/ime";
import {notifyAndLog} from "./utils/notifyUtils";
import {resolveActivePane, resolveActivePaneID, resolveActiveWindow} from "./utils/session";

type DockedAppBodyStyle = CSSProperties & {
    "--dock-main-width": string;
    "--dock-viewer-width": string;
};

function App() {
    useBackendSync();
    const {t} = useI18n();
    const [quickSearchOpen, setQuickSearchOpen] = useState(false);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const lastSyncedSessionRef = useRef<string | null>(null);

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
    const appBodyClassName = isViewerDocked
        ? "app-body app-body--viewer-docked"
        : "app-body";
    const appBodyStyle: DockedAppBodyStyle | undefined = isViewerDocked
        ? {
            "--dock-main-width": `${dockRatio * 100}%`,
            "--dock-viewer-width": `${(1 - dockRatio) * 100}%`,
        }
        : undefined;
    usePrefixKeyMode({activePaneId});
    useFileDrop(activePaneId);
    const imeRecoverySurfaceRef = useAppImeRecovery({activePaneId});

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
            if (e.ctrlKey && (e.key === "p" || e.key === "P")) {
                e.preventDefault();
                setQuickSearchOpen((prev) => !prev);
            }
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, []);

    return (
        <div className="app-root">
            <MenuBar onOpenSettings={() => setSettingsOpen(true)}/>
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
                <Sidebar sessions={sessions} activeSession={current?.name ?? null}/>
                <main className="main-content">
                    <SessionView session={current}/>
                    <ChatInputBar
                        activePaneId={activePaneId}
                        activePaneTitle={activePane?.title ?? ""}
                        panes={activeWindow?.panes ?? []}
                        chatOverlayPercentage={config?.chat_overlay_percentage ?? 80}
                    />
                    <StatusBar/>
                </main>
                <ViewerSystem/>
            </div>
            <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)}/>
            <ToastContainer/>
            <QuickSearch open={quickSearchOpen} onClose={() => setQuickSearchOpen(false)}/>
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
