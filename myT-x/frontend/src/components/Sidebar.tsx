import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {FixedSizeList} from "react-window";
import {api} from "../api";
import {makeScrollStableOuter} from "./viewer/views/shared/TreeOuter";
import {useContainerHeight} from "../hooks/useContainerHeight";
import {useNotificationStore} from "../stores/notificationStore";
import {useTmuxStore} from "../stores/tmuxStore";
import type {SessionSnapshot} from "../types/tmux";
import {useI18n} from "../i18n";
import {logFrontendEventSafe} from "../utils/logFrontendEventSafe";

import {KillSessionDialog} from "./KillSessionDialog";
import {NewSessionModal} from "./NewSessionModal";
import {PromoteBranchModal} from "./PromoteBranchModal";
import {SidebarHeader} from "./SidebarHeader";
import {SessionRow, sessionRowHeight, type SessionRowData, type SessionVisualState} from "./SidebarSessionItem";

interface SidebarProps {
    sessions: SessionSnapshot[];
    activeSession: string | null;
}

export function Sidebar(props: SidebarProps) {
    const {language, t} = useI18n();
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const reorderSession = useTmuxStore((s) => s.reorderSession);
    const addNotification = useNotificationStore((s) => s.addNotification);
    const [editingSession, setEditingSession] = useState<string | null>(null);
    const listHostRef = useRef<HTMLDivElement | null>(null);
    // Reserve at least one row height as floor so the list never collapses to zero
    // before ResizeObserver reports. noiseThresholdPx: 1 suppresses ±1px RO churn
    // that would otherwise cause scroll jitter in the session list.
    const listHeight = useContainerHeight(listHostRef, sessionRowHeight, {noiseThresholdPx: 1});
    const [showNewSession, setShowNewSession] = useState(false);
    const [killTarget, setKillTarget] = useState<string | null>(null);
    const [promoteTarget, setPromoteTarget] = useState<string | null>(null);
    const activeSessionRef = useRef(props.activeSession);
    const renameInFlightRef = useRef<Set<string>>(new Set());

    const sessionListOuter = useMemo(
        () =>
            makeScrollStableOuter({
                role: "list",
                ariaLabel: language === "en" ? "Sessions" : t("sidebar.aria.sessionsList", "Sessions"),
            }),
        [language, t],
    );

    const labelForSessionState = useCallback(
        (state: SessionVisualState): string => {
            switch (state) {
                case "selected":
                    return language === "en" ? "Selected" : t("sidebar.sessionState.selected", "Selected");
                case "idle":
                    return language === "en" ? "Stopped" : t("sidebar.sessionState.stopped", "Stopped");
                case "running":
                    return language === "en" ? "Running" : t("sidebar.sessionState.running", "Running");
            }
            const _exhaustive: never = state;
            return _exhaustive;
        },
        [language, t],
    );

    useEffect(() => {
        activeSessionRef.current = props.activeSession;
    }, [props.activeSession]);

    const activateSession = useCallback(
        async (sessionName: string) => {
            try {
                await api.SetActiveSession(sessionName);
                setActiveSession(sessionName);
            } catch (error) {
                console.error("[sidebar] SetActiveSession failed", {sessionName, error});
                addNotification(
                    language === "en"
                        ? `Failed to activate session "${sessionName}".`
                        : t("sidebar.error.activateFailed", "Failed to activate session \"{sessionName}\".", {sessionName}),
                    "warn",
                );
                logFrontendEventSafe("warn", `SetActiveSession failed: ${String(error)}`, "Sidebar");
            }
        },
        [addNotification, language, setActiveSession, t],
    );

    const startRename = useCallback((sessionName: string) => {
        setEditingSession(sessionName);
    }, []);

    const commitRename = useCallback(
        async (oldName: string, newName: string) => {
            if (renameInFlightRef.current.has(oldName)) {
                return;
            }
            setEditingSession(null);
            if (!newName || newName === oldName) {
                return;
            }
            renameInFlightRef.current.add(oldName);
            try {
                await api.RenameSession(oldName, newName);
                if (activeSessionRef.current === oldName) {
                    setActiveSession(newName);
                }
            } catch (error) {
                console.error("[sidebar] RenameSession failed", {oldName, newName, error});
                addNotification(
                    language === "en"
                        ? `Failed to rename session "${oldName}".`
                        : t("sidebar.error.renameFailed", "Failed to rename session \"{oldName}\".", {oldName}),
                    "warn",
                );
                logFrontendEventSafe("warn", `RenameSession failed: ${String(error)}`, "Sidebar");
            } finally {
                renameInFlightRef.current.delete(oldName);
            }
        },
        [addNotification, language, setActiveSession, t],
    );

    const handleKillClick = useCallback(
        (e: React.MouseEvent, sessionName: string) => {
            e.stopPropagation();
            setKillTarget(sessionName);
        },
        [],
    );

    const handleNewSession = useCallback(() => {
        setShowNewSession(true);
    }, []);

    const handleOpenDirectory = useCallback(
        (sessionName: string) => {
            void api.OpenDirectoryInExplorer(sessionName).catch((err) => {
                console.warn("[sidebar] OpenDirectoryInExplorer failed", err);
                addNotification(
                    language === "en"
                        ? `Could not open directory: ${sessionName}`
                        : t("sidebar.error.openDirectoryFailed", "ディレクトリを開けませんでした: {sessionName}", {sessionName}),
                    "warn",
                );
                logFrontendEventSafe("warn", `OpenDirectoryInExplorer failed (${sessionName}): ${String(err)}`, "Sidebar");
            });
        },
        [addNotification, language, t],
    );

    const handlePromote = useCallback((sessionName: string) => {
        setPromoteTarget(sessionName);
    }, []);

    const handleKillDone = useCallback(() => {
        const killed = killTarget;
        setKillTarget(null);
        if (killed && props.activeSession === killed) {
            const remaining = props.sessions.filter((s) => s.name !== killed);
            const next = remaining[0];
            if (next) {
                void activateSession(next.name);
            } else {
                setActiveSession(null);
            }
        }
    }, [activateSession, killTarget, props.activeSession, props.sessions, setActiveSession]);

    const rowData = useMemo<SessionRowData>(
        () => ({
            sessions: props.sessions,
            activeSession: props.activeSession,
            editingSession,
            onActivate: activateSession,
            onStartRename: startRename,
            onCommitRename: commitRename,
            onKill: handleKillClick,
            onPromote: handlePromote,
            onOpenDirectory: handleOpenDirectory,
            labelForSessionState,
            onReorder: reorderSession,
        }),
        [props.sessions, props.activeSession, editingSession, activateSession, startRename,
            commitRename, handleKillClick, handlePromote, handleOpenDirectory, labelForSessionState, reorderSession],
    );

    return (
        <aside className="sidebar">
            <SidebarHeader onNewSession={handleNewSession}/>

            <div className="session-list" ref={listHostRef}>
                {/* NOTE: height starts at 0 until ResizeObserver reports; guard prevents empty FixedSizeList render. */}
                {listHeight > 0 && (
                    <FixedSizeList
                        height={listHeight}
                        width="100%"
                        itemCount={props.sessions.length}
                        itemSize={sessionRowHeight}
                        itemData={rowData}
                        overscanCount={6}
                        outerElementType={sessionListOuter}
                    >
                        {SessionRow}
                    </FixedSizeList>
                )}
            </div>

            <NewSessionModal
                open={showNewSession}
                onClose={() => setShowNewSession(false)}
                onCreated={(name) => {
                    void activateSession(name);
                }}
            />

            <KillSessionDialog
                open={killTarget !== null}
                sessionName={killTarget || ""}
                onClose={() => setKillTarget(null)}
                onKilled={handleKillDone}
            />

            <PromoteBranchModal
                open={promoteTarget !== null}
                sessionName={promoteTarget || ""}
                onClose={() => setPromoteTarget(null)}
                onPromoted={() => {
                    /* snapshot will update via event */
                }}
            />
        </aside>
    );
}
