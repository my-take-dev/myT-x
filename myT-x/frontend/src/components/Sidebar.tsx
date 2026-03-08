import {memo, useCallback, useEffect, useMemo, useRef, useState, type ReactElement} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {api} from "../api";
import {makeScrollStableOuter} from "./viewer/views/shared/TreeOuter";
import {useContainerHeight} from "../hooks/useContainerHeight";
import {useNotificationStore} from "../stores/notificationStore";
import {useTmuxStore} from "../stores/tmuxStore";
import type {SessionSnapshot} from "../types/tmux";
import {KillSessionDialog} from "./KillSessionDialog";
import {NewSessionModal} from "./NewSessionModal";
import {PromoteBranchModal} from "./PromoteBranchModal";

interface SidebarProps {
    sessions: SessionSnapshot[];
    activeSession: string | null;
}

type SessionVisualState = "running" | "idle" | "selected";

function SessionBadges({session}: { session: SessionSnapshot }) {
    const worktree = session.worktree;
    if (!worktree) return null;
    const repoPath = worktree.repo_path?.trim() ?? "";
    const repoName = repoPath.split(/[\\/]/).filter(Boolean).pop();
    const hasBranchInfo = Boolean(worktree.base_branch || worktree.branch_name || worktree.is_detached);
    if (!repoPath && !hasBranchInfo) {
        return null;
    }
    return (
        <>
            {repoPath && (
                <span className="worktree-repo-badge" title={repoPath}>
                    {repoName || repoPath}
                </span>
            )}
            {worktree.base_branch && (
                <span className="worktree-base-branch-badge" title={`分岐元: ${worktree.base_branch}`}>
                    {worktree.base_branch}
                </span>
            )}
            {worktree.base_branch && (worktree.branch_name || worktree.is_detached) && (
                <span className="worktree-branch-arrow">{"\u2192"}</span>
            )}
            {(worktree.branch_name || worktree.is_detached) && (
                <span className={`worktree-branch-badge${worktree.is_detached ? " detached" : ""}`}>
                    {worktree.is_detached ? "detached" : worktree.branch_name}
                </span>
            )}
        </>
    );
}

function resolveSessionState(activeSession: string | null, session: SessionSnapshot): SessionVisualState {
    if (activeSession === session.name) {
        return "selected";
    }
    if (session.is_idle) {
        return "idle";
    }
    return "running";
}

function labelForSessionState(state: SessionVisualState): string {
    switch (state) {
        case "selected":
            return "Selected";
        case "idle":
            return "Stopped";
        case "running":
            return "Running";
    }
    const _exhaustive: never = state;
    return _exhaustive;
}

const sessionRowHeight = 80;

/** Module-level factory call — must not be inside a render function (see makeScrollStableOuter). */
const SessionListOuter = makeScrollStableOuter({role: "list", ariaLabel: "Sessions"});

interface SessionRowData {
    sessions: SessionSnapshot[];
    activeSession: string | null;
    editingSession: string | null;
    renderSession: (session: SessionSnapshot, isEditing: boolean, sessionState: SessionVisualState) => ReactElement;
    onReorder: (fromIndex: number, toIndex: number) => void;
}

// memo with custom areEqual: the `rowData` object reference changes whenever
// `editingSession` changes (since useMemo regenerates it), so default shallow
// comparison of `data` would cause ALL rows to re-render. The custom comparator
// checks individual `data` properties, so rows where no property actually changed
// (e.g. `isEditing` stays false) truly skip re-rendering.
const SessionRow = memo(function SessionRow({index, style, data}: ListChildComponentProps<SessionRowData>) {
    const session = data.sessions[index];
    if (!session) {
        return null;
    }
    const isEditing = data.editingSession === session.name;
    const sessionState = resolveSessionState(data.activeSession, session);
    return (
        <div
            style={style}
            draggable
            onDragStart={(e) => {
                e.dataTransfer.setData("text/session-index", String(index));
                e.dataTransfer.effectAllowed = "move";
            }}
            onDragOver={(e) => {
                e.preventDefault();
                e.dataTransfer.dropEffect = "move";
                const rect = e.currentTarget.getBoundingClientRect();
                const midY = rect.top + rect.height / 2;
                e.currentTarget.classList.toggle("drop-above", e.clientY < midY);
                e.currentTarget.classList.toggle("drop-below", e.clientY >= midY);
            }}
            onDragLeave={(e) => {
                if (e.relatedTarget instanceof Node && e.currentTarget.contains(e.relatedTarget)) {
                    return;
                }
                e.currentTarget.classList.remove("drop-above", "drop-below");
            }}
            onDrop={(e) => {
                e.preventDefault();
                e.currentTarget.classList.remove("drop-above", "drop-below");
                const fromIndex = parseInt(e.dataTransfer.getData("text/session-index"), 10);
                if (!isNaN(fromIndex) && fromIndex !== index) {
                    data.onReorder(fromIndex, index);
                }
            }}
            onDragEnd={(e) => {
                e.currentTarget.classList.remove("drop-above", "drop-below");
            }}
        >
            <div className="session-row">{data.renderSession(session, isEditing, sessionState)}</div>
        </div>
    );
}, (prev, next) => {
    if (prev.index !== next.index) return false;
    // NOTE: react-window's FixedSizeList provides referentially stable style objects
    // for each visible index (reused across renders). If migrating to VariableSizeList,
    // verify style stability or switch to top/height value comparison.
    if (prev.style !== next.style) return false;
    const pd = prev.data;
    const nd = next.data;
    return (
        // NOTE: sessions array reference is stable from Zustand store (only replaced on actual change).
        pd.sessions === nd.sessions &&
        // Re-render only rows whose active-status relation changed.
        (pd.activeSession === pd.sessions[prev.index]?.name) === (nd.activeSession === nd.sessions[next.index]?.name) &&
        // Compare whether editingSession affects THIS specific row, not the global value.
        // This ensures only the previously-editing row and the newly-editing row re-render.
        (pd.editingSession === pd.sessions[prev.index]?.name) === (nd.editingSession === nd.sessions[next.index]?.name) &&
        pd.renderSession === nd.renderSession &&
        pd.onReorder === nd.onReorder
    );
});

export function Sidebar(props: SidebarProps) {
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const reorderSession = useTmuxStore((s) => s.reorderSession);
    const addNotification = useNotificationStore((s) => s.addNotification);
    const [editingSession, setEditingSession] = useState<string | null>(null);
    const editValueRef = useRef("");
    const inputRef = useRef<HTMLInputElement>(null);
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

    useEffect(() => {
        activeSessionRef.current = props.activeSession;
    }, [props.activeSession]);

    const startRename = useCallback((sessionName: string) => {
        editValueRef.current = sessionName;
        setEditingSession(sessionName);
        requestAnimationFrame(() => inputRef.current?.select());
    }, []);

    const activateSession = useCallback(
        async (sessionName: string) => {
            try {
                await api.SetActiveSession(sessionName);
                setActiveSession(sessionName);
            } catch (error) {
                console.error("[sidebar] SetActiveSession failed", {sessionName, error});
                addNotification(`Failed to activate session "${sessionName}".`, "warn");
            }
        },
        [addNotification, setActiveSession],
    );

    const commitRename = useCallback(
        async (oldName: string) => {
            if (renameInFlightRef.current.has(oldName)) {
                return;
            }
            const newName = editValueRef.current.trim();
            // Prevent Enter + blur double-submit from sending two rename requests.
            editValueRef.current = oldName;
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
                addNotification(`Failed to rename session "${oldName}".`, "warn");
            } finally {
                renameInFlightRef.current.delete(oldName);
            }
        },
        [addNotification, setActiveSession],
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

    const renderSession = useCallback(
        (session: SessionSnapshot, isEditing: boolean, sessionState: SessionVisualState) => {
            return (
                <div
                    role="button"
                    tabIndex={0}
                    className={`session-item ${sessionState}`}
                    onClick={() => {
                        if (isEditing) {
                            return;
                        }
                        void activateSession(session.name);
                    }}
                    onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            if (isEditing) {
                                return;
                            }
                            void activateSession(session.name);
                            return;
                        }
                        if (e.key === "F2") {
                            e.preventDefault();
                            if (!isEditing) {
                                startRename(session.name);
                            }
                        }
                    }}
                    onDoubleClick={() => startRename(session.name)}
                >
                    <div className="session-item-row">
                        <span className={`session-type-mark ${session.is_agent_team ? "agent" : "session"}`}>
                            {session.is_agent_team ? "A" : "S"}
                        </span>
                        {isEditing ? (
                            <input
                                key={session.name}
                                ref={inputRef}
                                className="session-name-input"
                                defaultValue={session.name}
                                onChange={(e) => {
                                    editValueRef.current = e.target.value;
                                }}
                                onKeyDown={(e) => {
                                    if (e.key === "Enter") {
                                        void commitRename(session.name);
                                    } else if (e.key === "Escape") {
                                        editValueRef.current = session.name;
                                        setEditingSession(null);
                                    }
                                }}
                                onBlur={() => void commitRename(session.name)}
                                onClick={(e) => e.stopPropagation()}
                            />
                        ) : (
                            <span className="session-name">{session.name}</span>
                        )}
                        <span className={`session-state ${sessionState}`}>
                            {labelForSessionState(sessionState)}
                        </span>
                    </div>
                    {(session.worktree?.repo_path || session.worktree?.is_detached) && (
                        <span className="session-meta">
                            <SessionBadges session={session}/>
                            {session.worktree?.is_detached && Boolean(session.worktree?.path?.trim()) && (
                                <button
                                    type="button"
                                    className="modal-btn session-promote-btn"
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        setPromoteTarget(session.name);
                                    }}
                                    title="ブランチに昇格"
                                >
                                    Promote
                                </button>
                            )}
                        </span>
                    )}
                    {(session.root_path?.trim() || session.worktree?.path?.trim()) && (
                        <button
                            type="button"
                            className="session-explorer"
                            onClick={(e) => {
                                e.stopPropagation();
                                void api.OpenDirectoryInExplorer(session.name).catch((err) => {
                                    console.warn("[sidebar] OpenDirectoryInExplorer failed", err);
                                    addNotification(`ディレクトリを開けませんでした: ${session.name}`, "warn");
                                });
                            }}
                            title="エクスプローラーで開く"
                            aria-label={`Open directory for ${session.name}`}
                        >
                            {"\u21D7"}
                        </button>
                    )}
                    <button
                        type="button"
                        className="session-close"
                        onClick={(e) => handleKillClick(e, session.name)}
                        title="セッションを閉じる"
                        aria-label={`Close session ${session.name}`}
                    >
                        ×
                    </button>
                </div>
            );
        },
        [activateSession, addNotification, commitRename, handleKillClick, startRename],
    );

    const rowData = useMemo<SessionRowData>(
        () => ({
            sessions: props.sessions,
            activeSession: props.activeSession,
            editingSession,
            renderSession,
            onReorder: reorderSession,
        }),
        [props.sessions, props.activeSession, editingSession, renderSession, reorderSession],
    );

    return (
        <aside className="sidebar">
            <div className="sidebar-header">
                <h1>myT-x</h1>
                <p>ターミナルマルチプレクサ</p>
            </div>

            <div className="sidebar-actions">
                <button
                    type="button"
                    className="primary"
                    onClick={handleNewSession}
                >
                    + 新規セッション
                </button>
            </div>

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
                        outerElementType={SessionListOuter}
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
