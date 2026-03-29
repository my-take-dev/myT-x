import {memo, useEffect, useRef, type ReactElement} from "react";
import type {ListChildComponentProps} from "react-window";
import {useI18n} from "../i18n";
import type {SessionSnapshot} from "../types/tmux";

export type SessionVisualState = "running" | "idle" | "selected";

export const sessionRowHeight = 80;

export function resolveSessionState(activeSession: string | null, session: SessionSnapshot): SessionVisualState {
    if (activeSession === session.name) {
        return "selected";
    }
    if (session.is_idle) {
        return "idle";
    }
    return "running";
}

// --- SessionRowData: passed through react-window's FixedSizeList itemData ---

export interface SessionRowData {
    readonly sessions: SessionSnapshot[];
    readonly activeSession: string | null;
    readonly editingSession: string | null;
    readonly onActivate: (name: string) => void;
    readonly onStartRename: (name: string) => void;
    readonly onCommitRename: (oldName: string, newName: string) => void;
    readonly onKill: (e: React.MouseEvent, name: string) => void;
    readonly onPromote: (name: string) => void;
    readonly onOpenDirectory: (name: string) => void;
    readonly labelForSessionState: (state: SessionVisualState) => string;
    readonly onReorder: (fromIndex: number, toIndex: number) => void;
}

// --- SessionBadges: worktree repo/branch badge row ---

function SessionBadges({session}: { readonly session: SessionSnapshot }) {
    const {language, t} = useI18n();
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
                <span
                    className="worktree-base-branch-badge"
                    title={`${
                        language === "en"
                            ? "Base branch"
                            : t("sidebar.worktree.baseBranchFrom", "分岐元")
                    }: ${worktree.base_branch}`}
                >
                    {worktree.base_branch}
                </span>
            )}
            {worktree.base_branch && (worktree.branch_name || worktree.is_detached) && (
                <span className="worktree-branch-arrow">{"\u2192"}</span>
            )}
            {(worktree.branch_name || worktree.is_detached) && (
                <span className={`worktree-branch-badge${worktree.is_detached ? " detached" : ""}`}>
                    {worktree.is_detached
                        ? (language === "en" ? "detached" : t("sidebar.worktree.detached", "detached"))
                        : worktree.branch_name}
                </span>
            )}
        </>
    );
}

// --- SidebarSessionItem: single session item rendering ---

interface SidebarSessionItemProps {
    readonly session: SessionSnapshot;
    readonly isEditing: boolean;
    readonly sessionState: SessionVisualState;
    readonly sessionStateLabel: string;
    readonly onActivate: (name: string) => void;
    readonly onStartRename: (name: string) => void;
    readonly onCommitRename: (oldName: string, newName: string) => void;
    readonly onKill: (e: React.MouseEvent, name: string) => void;
    readonly onPromote: (name: string) => void;
    readonly onOpenDirectory: (name: string) => void;
}

export function SidebarSessionItem({
    session,
    isEditing,
    sessionState,
    sessionStateLabel,
    onActivate,
    onStartRename,
    onCommitRename,
    onKill,
    onPromote,
    onOpenDirectory,
}: SidebarSessionItemProps): ReactElement {
    const {language, t} = useI18n();
    // Uncontrolled input: editValueRef tracks keystroke values without re-render.
    // Reset when editing starts so the ref always reflects the current session name.
    const editValueRef = useRef(session.name);
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        if (isEditing) {
            editValueRef.current = session.name;
            requestAnimationFrame(() => inputRef.current?.select());
        }
    }, [isEditing, session.name]);

    return (
        <div
            role="button"
            tabIndex={0}
            className={`session-item ${sessionState}`}
            onClick={() => {
                if (isEditing) return;
                onActivate(session.name);
            }}
            onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    if (isEditing) return;
                    onActivate(session.name);
                    return;
                }
                if (e.key === "F2") {
                    e.preventDefault();
                    if (!isEditing) {
                        onStartRename(session.name);
                    }
                }
            }}
            onDoubleClick={() => onStartRename(session.name)}
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
                                e.preventDefault();
                                onCommitRename(session.name, editValueRef.current.trim());
                            } else if (e.key === "Escape") {
                                e.preventDefault();
                                // Reset ref before commit so the blur handler sees no change.
                                editValueRef.current = session.name;
                                onCommitRename(session.name, session.name);
                            }
                        }}
                        onBlur={() => onCommitRename(session.name, editValueRef.current.trim())}
                        onClick={(e) => e.stopPropagation()}
                    />
                ) : (
                    <span className="session-name">{session.name}</span>
                )}
                <span className={`session-state ${sessionState}`}>
                    {sessionStateLabel}
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
                                onPromote(session.name);
                            }}
                            title={
                                language === "en"
                                    ? "Promote to branch"
                                    : t("sidebar.action.promoteBranch.title", "ブランチに昇格")
                            }
                        >
                            {language === "en"
                                ? "Promote"
                                : t("sidebar.action.promoteBranch.button", "Promote")}
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
                        onOpenDirectory(session.name);
                    }}
                    title={
                        language === "en"
                            ? "Open in Explorer"
                            : t("sidebar.action.openInExplorer.title", "エクスプローラーで開く")
                    }
                    aria-label={
                        language === "en"
                            ? `Open directory for ${session.name}`
                            : t("sidebar.action.openInExplorer.aria", "Open directory for {sessionName}", {
                                sessionName: session.name,
                            })
                    }
                >
                    {"\u21D7"}
                </button>
            )}
            <button
                type="button"
                className="session-close"
                onClick={(e) => onKill(e, session.name)}
                title={language === "en" ? "Close session" : t("sidebar.action.closeSession.title", "セッションを閉じる")}
                aria-label={
                    language === "en"
                        ? `Close session ${session.name}`
                        : t("sidebar.action.closeSession.aria", "Close session {sessionName}", {
                            sessionName: session.name,
                        })
                }
            >
                ×
            </button>
        </div>
    );
}

// --- SessionRow: react-window virtualized row wrapper with drag-drop ---

// memo with custom areEqual: the `rowData` object reference changes whenever
// `editingSession` changes (since useMemo regenerates it), so default shallow
// comparison of `data` would cause ALL rows to re-render. The custom comparator
// checks individual `data` properties, so rows where no property actually changed
// (e.g. `isEditing` stays false) truly skip re-rendering.
export const SessionRow = memo(function SessionRow({index, style, data}: ListChildComponentProps<SessionRowData>) {
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
            <div className="session-row">
                <SidebarSessionItem
                    session={session}
                    isEditing={isEditing}
                    sessionState={sessionState}
                    sessionStateLabel={data.labelForSessionState(sessionState)}
                    onActivate={data.onActivate}
                    onStartRename={data.onStartRename}
                    onCommitRename={data.onCommitRename}
                    onKill={data.onKill}
                    onPromote={data.onPromote}
                    onOpenDirectory={data.onOpenDirectory}
                />
            </div>
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
        // Callbacks compared here (onActivate, onCommitRename, onReorder) are
        // useCallback-wrapped in Sidebar, so reference equality holds unless their
        // dependencies change (in which case all rows should re-render).
        //
        // Intentionally omitted from comparison (all useCallback-wrapped with
        // stable deps in Sidebar, so reference never changes across renders):
        //   onStartRename, onKill, onPromote, onOpenDirectory, labelForSessionState
        // If any of these callbacks' useCallback wrapping is removed in Sidebar,
        // add them to this comparison to prevent stale-callback rendering.
        pd.onActivate === nd.onActivate &&
        pd.onCommitRename === nd.onCommitRename &&
        pd.onReorder === nd.onReorder
    );
});
