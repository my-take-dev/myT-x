import {memo, useCallback, useEffect, useMemo, useState} from "react";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {PaneSnapshot} from "../../../../types/tmux";
import {useDiffReviewSend} from "./useDiffReviewSend";
import {useDiffReviewSessionKey} from "./diffReviewSession";

export const DiffReviewActionBar = memo(function DiffReviewActionBar() {
    const {commentCount, sending, sendError, handleSend} = useDiffReviewSend();
    const activeSessionKey = useDiffReviewSessionKey();
    const clearCommentsForSession = useDiffReviewStore((s) => s.clearCommentsForSession);
    const [selectedPaneId, setSelectedPaneId] = useState("");

    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);
    const activeWindowId = useTmuxStore((s) => s.activeWindowId);

    const panes: PaneSnapshot[] = useMemo(() => {
        const session = sessions.find((s) => s.name === activeSession);
        const window = session?.windows?.find((w) => String(w.id) === activeWindowId);
        return window?.panes ?? [];
    }, [sessions, activeSession, activeWindowId]);

    useEffect(() => {
        if (selectedPaneId !== "" && !panes.some((pane) => pane.id === selectedPaneId)) {
            setSelectedPaneId("");
        }
    }, [panes, selectedPaneId]);

    const selectedPaneIsAvailable = selectedPaneId !== "" && panes.some((pane) => pane.id === selectedPaneId);
    const clearLabel = `Clear ${commentCount} review comment${commentCount === 1 ? "" : "s"} in this session`;

    const handleSendClick = useCallback(() => {
        if (selectedPaneIsAvailable) {
            void handleSend(selectedPaneId);
        }
    }, [selectedPaneId, selectedPaneIsAvailable, handleSend]);

    const handleClearClick = useCallback(() => {
        if (activeSessionKey === "") return;
        if (!window.confirm(`${clearLabel}\n\nThis cannot be undone.`)) {
            return;
        }
        clearCommentsForSession(activeSessionKey);
    }, [activeSessionKey, clearCommentsForSession, clearLabel]);

    if (commentCount === 0) return null;

    return (
        <>
            <span className="diff-review-badge" title={`${commentCount} review comment${commentCount !== 1 ? "s" : ""}`}>
                {commentCount}
            </span>
            <select
                className="diff-review-pane-select"
                value={selectedPaneId}
                onChange={(e) => setSelectedPaneId(e.target.value)}
                title="Select target pane"
            >
                <option value="">Pane...</option>
                {panes.map((p) => (
                    <option key={p.id} value={p.id}>
                        {p.id} ({p.title ? `pane ${p.index}: ${p.title}` : `pane ${p.index}`})
                    </option>
                ))}
            </select>
            <button
                type="button"
                className="viewer-header-btn diff-review-send-btn"
                onClick={handleSendClick}
                disabled={!selectedPaneIsAvailable || sending}
                title="Send review comments to pane"
            >
                {sending ? "..." : "Send"}
            </button>
            <button
                type="button"
                className="viewer-header-btn diff-review-clear-btn"
                onClick={handleClearClick}
                title={clearLabel}
                aria-label={clearLabel}
            >
                Clear
            </button>
            {sendError && <span className="diff-review-error">{sendError}</span>}
        </>
    );
});
