import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FixedSizeList, type ListChildComponentProps } from "react-window";
import { api } from "../api";
import { useNotificationStore } from "../stores/notificationStore";
import { useTmuxStore } from "../stores/tmuxStore";
import type { SessionSnapshot } from "../types/tmux";
import { KillSessionDialog } from "./KillSessionDialog";
import { NewSessionModal } from "./NewSessionModal";
import { PromoteBranchModal } from "./PromoteBranchModal";

interface SidebarProps {
  sessions: SessionSnapshot[];
  activeSession: string | null;
}

type SessionVisualState = "running" | "idle" | "selected";

function SessionBadges({ session }: { session: SessionSnapshot }) {
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
        <>
          <span className="worktree-base-branch-badge" title={`分岐元: ${worktree.base_branch}`}>
            {worktree.base_branch}
          </span>
        </>
      )}
      {worktree.base_branch && (worktree.branch_name || worktree.is_detached) && (
        <span className="worktree-branch-arrow">{"\u2192"}</span>
      )}
      {(worktree.branch_name || worktree.is_detached) && (
        <>
          <span className={`worktree-branch-badge${worktree.is_detached ? " detached" : ""}`}>
            {worktree.is_detached ? "detached" : worktree.branch_name}
          </span>
        </>
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
    default:
      return "Running";
  }
}

const sessionRowHeight = 80;

interface SessionRowData {
  sessions: SessionSnapshot[];
  renderSession: (session: SessionSnapshot) => JSX.Element;
  onReorder: (fromIndex: number, toIndex: number) => void;
}

function SessionRow({ index, style, data }: ListChildComponentProps<SessionRowData>) {
  const session = data.sessions[index];
  if (!session) {
    return null;
  }
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
    >
      <div className="session-row">{data.renderSession(session)}</div>
    </div>
  );
}

export function Sidebar(props: SidebarProps) {
  const setActiveSession = useTmuxStore((s) => s.setActiveSession);
  const reorderSession = useTmuxStore((s) => s.reorderSession);
  const addNotification = useNotificationStore((s) => s.addNotification);
  const [editingSession, setEditingSession] = useState<string | null>(null);
  const editValueRef = useRef("");
  const inputRef = useRef<HTMLInputElement>(null);
  const listHostRef = useRef<HTMLDivElement | null>(null);
  const [listHeight, setListHeight] = useState(0);
  const [showNewSession, setShowNewSession] = useState(false);
  const [killTarget, setKillTarget] = useState<string | null>(null);
  const [promoteTarget, setPromoteTarget] = useState<string | null>(null);

  useEffect(() => {
    const host = listHostRef.current;
    if (!host) {
      return;
    }
    const updateHeight = () => {
      setListHeight(Math.max(host.clientHeight, sessionRowHeight));
    };
    updateHeight();
    const observer = new ResizeObserver(updateHeight);
    observer.observe(host);
    return () => observer.disconnect();
  }, []);

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
        console.error("[sidebar] SetActiveSession failed", { sessionName, error });
        addNotification(`Failed to activate session "${sessionName}".`, "warn");
      }
    },
    [addNotification, setActiveSession],
  );

  const commitRename = useCallback(
    async (oldName: string) => {
      const newName = editValueRef.current.trim();
      setEditingSession(null);
      if (!newName || newName === oldName) {
        return;
      }
      try {
        await api.RenameSession(oldName, newName);
        if (props.activeSession === oldName) {
          setActiveSession(newName);
        }
      } catch (error) {
        console.error("[sidebar] RenameSession failed", { oldName, newName, error });
        addNotification(`Failed to rename session "${oldName}".`, "warn");
      }
    },
    [addNotification, props.activeSession, setActiveSession],
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
      if (remaining.length > 0) {
        void activateSession(remaining[0].name);
      } else {
        setActiveSession(null);
      }
    }
  }, [activateSession, killTarget, props.activeSession, props.sessions, setActiveSession]);

  const renderSession = useCallback(
    (session: SessionSnapshot) => {
      const sessionState = resolveSessionState(props.activeSession, session);
      return (
        <div
          role="button"
          tabIndex={0}
          className={`session-item ${sessionState}`}
          onClick={() => {
            if (editingSession === session.name) {
              return;
            }
            void activateSession(session.name);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              if (editingSession === session.name) {
                return;
              }
              void activateSession(session.name);
            }
          }}
          onDoubleClick={() => startRename(session.name)}
        >
          <div className="session-item-row">
            <span className={`session-type-mark ${session.is_agent_team ? "agent" : "session"}`}>
              {session.is_agent_team ? "A" : "S"}
            </span>
            {editingSession === session.name ? (
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
              <SessionBadges session={session} />
              {session.worktree?.is_detached && Boolean(session.worktree?.path?.trim()) && (
                <button
                  type="button"
                  className="modal-btn"
                  style={{ marginLeft: 4, padding: "1px 6px", fontSize: "0.62rem", borderRadius: 6 }}
                  onClick={(e) => { e.stopPropagation(); setPromoteTarget(session.name); }}
                  title="ブランチに昇格"
                >
                  Promote
                </button>
              )}
            </span>
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
    [activateSession, commitRename, editingSession, handleKillClick, props.activeSession, startRename],
  );

  const rowData = useMemo<SessionRowData>(
    () => ({
      sessions: props.sessions,
      renderSession,
      onReorder: reorderSession,
    }),
    [props.sessions, renderSession, reorderSession],
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
        {listHeight > 0 && (
          <FixedSizeList
            height={listHeight}
            width="100%"
            itemCount={props.sessions.length}
            itemSize={sessionRowHeight}
            itemData={rowData}
            overscanCount={6}
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
