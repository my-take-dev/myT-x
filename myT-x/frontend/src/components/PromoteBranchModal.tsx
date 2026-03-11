import {useCallback, useEffect, useState} from "react";
import {api} from "../api";
import {useEscapeClose} from "../hooks/useEscapeClose";
import {useI18n} from "../i18n";

interface PromoteBranchModalProps {
    open: boolean;
    sessionName: string;
    onClose: () => void;
    onPromoted: () => void;
}

export function PromoteBranchModal({open, sessionName, onClose, onPromoted}: PromoteBranchModalProps) {
    const {language, t} = useI18n();
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
                    <h2>
                        {language === "en"
                            ? "Promote to Branch"
                            : t("promoteBranch.title", "ブランチに昇格")}
                    </h2>
                </div>
                <div className="modal-body">
                    <p style={{margin: 0, fontSize: "0.84rem", color: "var(--fg-dim)"}}>
                        {language === "en"
                            ? `Convert detached HEAD of session "${sessionName}" to a named branch.`
                            : t("promoteBranch.description", "セッション \"{sessionName}\" の detached HEAD を名前付きブランチに変換します。", {sessionName})}
                    </p>
                    <div className="form-group">
                        <span className="form-label">
                            {language === "en"
                                ? "Branch Name"
                                : t("promoteBranch.branchName.label", "ブランチ名")}
                        </span>
                        <input
                            className="form-input"
                            value={branchName}
                            onChange={(e) => setBranchName(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === "Enter" && branchName.trim()) void handleSubmit();
                            }}
                            placeholder={
                                language === "en"
                                    ? "feature/my-branch"
                                    : t("promoteBranch.branchName.placeholder", "feature/my-branch")
                            }
                            autoFocus
                        />
                    </div>
                    {error && <p className="form-error">{error}</p>}
                </div>
                <div className="modal-footer">
                    <button type="button" className="modal-btn" onClick={onClose} disabled={loading}>
                        {language === "en"
                            ? "Cancel"
                            : t("common.cancel", "キャンセル")}
                    </button>
                    <button
                        type="button"
                        className="modal-btn primary"
                        onClick={handleSubmit}
                        disabled={!branchName.trim() || loading}
                    >
                        {loading
                            ? (language === "en"
                                ? "Promoting..."
                                : t("promoteBranch.action.promoting", "昇格中..."))
                            : (language === "en"
                                ? "Promote"
                                : t("promoteBranch.action.promote", "昇格"))}
                    </button>
                </div>
            </div>
        </div>
    );
}
