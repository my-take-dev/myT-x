import { useCallback, useEffect, useState } from "react";
import { api } from "../api";
import { useEscapeClose } from "../hooks/useEscapeClose";

interface PromoteBranchModalProps {
  open: boolean;
  sessionName: string;
  onClose: () => void;
  onPromoted: () => void;
}

export function PromoteBranchModal({ open, sessionName, onClose, onPromoted }: PromoteBranchModalProps) {
  const [branchName, setBranchName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) {
      setBranchName("");
      setLoading(false);
      setError("");
    }
  }, [open]);

  useEscapeClose(open, onClose);

  const handleSubmit = useCallback(async () => {
    const name = branchName.trim();
    if (!name) return;
    setLoading(true);
    setError("");
    try {
      await api.PromoteWorktreeToBranch(sessionName, name);
      onPromoted();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }, [branchName, sessionName, onPromoted, onClose]);

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>ブランチに昇格</h2>
        </div>
        <div className="modal-body">
          <p style={{ margin: 0, fontSize: "0.84rem", color: "var(--fg-dim)" }}>
            セッション &quot;{sessionName}&quot; の detached HEAD を名前付きブランチに変換します。
          </p>
          <div className="form-group">
            <span className="form-label">ブランチ名</span>
            <input
              className="form-input"
              value={branchName}
              onChange={(e) => setBranchName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && branchName.trim()) void handleSubmit(); }}
              placeholder="feature/my-branch"
              autoFocus
            />
          </div>
          {error && <p className="form-error">{error}</p>}
        </div>
        <div className="modal-footer">
          <button type="button" className="modal-btn" onClick={onClose} disabled={loading}>
            キャンセル
          </button>
          <button
            type="button"
            className="modal-btn primary"
            onClick={handleSubmit}
            disabled={!branchName.trim() || loading}
          >
            {loading ? "昇格中..." : "昇格"}
          </button>
        </div>
      </div>
    </div>
  );
}
