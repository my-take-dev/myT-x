import { useCallback, useEffect, useState } from "react";
import { api } from "../api";
import { useEscapeClose } from "../hooks/useEscapeClose";

/**
 * Invariant: when has_worktree is false, the other fields are meaningless
 * (defaults from Go zero values). Consumers should check has_worktree first.
 */
interface WorktreeStatusResult {
  has_worktree: boolean;
  has_uncommitted: boolean;
  has_unpushed: boolean;
  branch_name: string;
  is_detached: boolean;
}

interface KillSessionDialogProps {
  open: boolean;
  sessionName: string;
  onClose: () => void;
  onKilled: () => void;
}

type DialogPhase = "loading" | "ready" | "processing";

export function KillSessionDialog({ open, sessionName, onClose, onKilled }: KillSessionDialogProps) {
  const [phase, setPhase] = useState<DialogPhase>("loading");
  const [status, setStatus] = useState<WorktreeStatusResult | null>(null);
  const [commitMessage, setCommitMessage] = useState("");
  const [deleteWorktree, setDeleteWorktree] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) {
      setPhase("loading");
      setStatus(null);
      setCommitMessage("");
      setDeleteWorktree(true);
      setError("");
      return;
    }
    setPhase("loading");
    api.CheckWorktreeStatus(sessionName)
      .then((s) => {
        setStatus(s);
        setPhase("ready");
      })
      .catch((err) => {
        // Safe fallback: assume worktree with uncommitted changes to prevent data loss.
        setStatus({ has_worktree: true, has_uncommitted: true, has_unpushed: true, branch_name: "", is_detached: false });
        setPhase("ready");
        setError(`ワークツリー状態の取得に失敗しました（安全のため未保存変更ありとして扱います）: ${String(err)}`);
        console.warn("[worktree] CheckWorktreeStatus failed:", err);
      });
  }, [open, sessionName]);

  useEscapeClose(open && phase !== "processing", onClose);

  const shouldDeleteWt = deleteWorktree && (status?.has_worktree ?? false);

  const handleKillOnly = useCallback(async () => {
    setPhase("processing");
    setError("");
    try {
      await api.KillSession(sessionName, shouldDeleteWt);
      onKilled();
      onClose();
    } catch (err) {
      setError(String(err));
      setPhase("ready");
    }
  }, [sessionName, shouldDeleteWt, onKilled, onClose]);

  const handleCommitAndKill = useCallback(async (push: boolean) => {
    setPhase("processing");
    setError("");
    try {
      const msg = commitMessage.trim();
      if (msg) {
        await api.CommitAndPushWorktree(sessionName, msg, push);
      } else if (push) {
        await api.CommitAndPushWorktree(sessionName, "", push);
      }
      await api.KillSession(sessionName, shouldDeleteWt);
      onKilled();
      onClose();
    } catch (err) {
      setError(String(err));
      setPhase("ready");
    }
  }, [sessionName, commitMessage, shouldDeleteWt, onKilled, onClose]);

  if (!open) return null;

  const isProcessing = phase === "processing";
  const needsAction = status?.has_uncommitted || status?.has_unpushed;
  const canPush = status ? !status.is_detached && !!status.branch_name : false;

  return (
    <div className="modal-overlay" onClick={() => { if (!isProcessing) onClose(); }}>
      <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>セッションを閉じる</h2>
        </div>
        <div className="modal-body">
          {phase === "loading" && (
            <div className="modal-loading">ワークツリー状態を確認中...</div>
          )}

          {phase !== "loading" && !needsAction && (
            <p style={{ margin: 0, fontSize: "0.88rem" }}>
              セッション &quot;{sessionName}&quot; を閉じますか？
            </p>
          )}

          {phase !== "loading" && needsAction && (
            <>
              <p style={{ margin: 0, fontSize: "0.84rem", color: "var(--fg-dim)" }}>
                セッション &quot;{sessionName}&quot; のワークツリーに未保存の変更があります。
              </p>

              {status?.has_uncommitted && (
                <>
                  <p style={{ margin: 0, fontSize: "0.78rem", color: "var(--danger)" }}>
                    未コミットの変更があります
                  </p>
                  <div className="form-group">
                    <span className="form-label">コミットメッセージ</span>
                    <input
                      className="form-input"
                      value={commitMessage}
                      onChange={(e) => setCommitMessage(e.target.value)}
                      placeholder="変更内容を入力..."
                      disabled={isProcessing}
                      autoFocus
                    />
                  </div>
                </>
              )}

              {!status?.has_uncommitted && status?.has_unpushed && (
                <p style={{ margin: 0, fontSize: "0.78rem", color: "var(--danger)" }}>
                  未プッシュのコミットがあります
                </p>
              )}
            </>
          )}

          {/* Worktree deletion checkbox */}
          {phase !== "loading" && status?.has_worktree && (
            <div className="form-checkbox-row" style={{ marginTop: 8 }}>
              <input
                type="checkbox"
                id="delete-worktree"
                checked={deleteWorktree}
                onChange={(e) => setDeleteWorktree(e.target.checked)}
                disabled={isProcessing}
              />
              <label htmlFor="delete-worktree">ワークツリーを削除する</label>
              <p className="form-hint">チェックを外すとworktreeは保持されます</p>
            </div>
          )}

          {error && <p className="form-error">{error}</p>}
        </div>
        <div className="modal-footer">
          <button
            type="button"
            className="modal-btn"
            onClick={onClose}
            disabled={isProcessing}
          >
            キャンセル
          </button>

          {phase !== "loading" && needsAction && (
            <button
              type="button"
              className="modal-btn danger"
              onClick={handleKillOnly}
              disabled={isProcessing}
            >
              そのまま閉じる
            </button>
          )}

          {phase !== "loading" && (() => {
            if (!needsAction) {
              return (
                <button type="button" className="modal-btn danger"
                  onClick={handleKillOnly} disabled={isProcessing}>
                  {isProcessing ? "処理中..." : "閉じる"}
                </button>
              );
            }
            if (status?.has_uncommitted) {
              return (
                <button type="button" className="modal-btn primary"
                  onClick={() => void handleCommitAndKill(canPush)}
                  disabled={isProcessing || !commitMessage.trim()}>
                  {isProcessing ? "処理中..." : canPush ? "Commit & Push して閉じる" : "Commit して閉じる"}
                </button>
              );
            }
            if (status?.has_unpushed && canPush) {
              return (
                <button type="button" className="modal-btn primary"
                  onClick={() => void handleCommitAndKill(true)}
                  disabled={isProcessing}>
                  {isProcessing ? "処理中..." : "Push して閉じる"}
                </button>
              );
            }
            return null;
          })()}
        </div>
      </div>
    </div>
  );
}
