import {memo, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useI18n} from "../../../../i18n";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useMCPStore} from "../../../../stores/mcpStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {PaneSnapshot} from "../../../../types/tmux";
import {isStrMcp} from "../mcp-manager/useMcpManager";
import {type DiffReviewSendPayload, useDiffReviewSend} from "./useDiffReviewSend";
import {useDiffReviewSessionKey} from "./diffReviewSession";
import {DiffReviewSendDialog} from "./DiffReviewSendDialog";

export const DiffReviewActionBar = memo(function DiffReviewActionBar() {
    const {t} = useI18n();
    const {commentCount, comments, sending, registering, sendError, handleSend, handleAddToSingleTaskRunner} =
        useDiffReviewSend();
    const activeSessionKey = useDiffReviewSessionKey();
    const clearCommentsForSession = useDiffReviewStore((s) => s.clearCommentsForSession);
    const [selectedPaneId, setSelectedPaneId] = useState("");
    const [sendDialogOpen, setSendDialogOpen] = useState(false);
    const [sendDialogDirty, setSendDialogDirty] = useState(false);
    const [sendInFlight, setSendInFlight] = useState(false);
    const sendSuccessClosePendingRef = useRef(false);

    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);
    const activeWindowId = useTmuxStore((s) => s.activeWindowId);
    const activeMcpSnapshots = useMCPStore((s) => (activeSession ? s.snapshots[activeSession] : undefined));
    const beginMcpSessionLoad = useMCPStore((s) => s.beginSessionLoad);
    const setMcpSessionLoading = useMCPStore((s) => s.setSessionLoading);
    const setMcpSessionError = useMCPStore((s) => s.setSessionError);
    const setMcpSnapshots = useMCPStore((s) => s.setSnapshots);

    const panes: PaneSnapshot[] = useMemo(() => {
        const session = sessions.find((s) => s.name === activeSession);
        const window = session?.windows?.find((w) => String(w.id) === activeWindowId);
        return window?.panes ?? [];
    }, [sessions, activeSession, activeWindowId]);
    const paneIds = useMemo(() => new Set(panes.map((pane) => pane.id)), [panes]);
    const selectedPaneIsAvailable = selectedPaneId !== "" && paneIds.has(selectedPaneId);
    const dialogSending = sending || sendInFlight;
    const isBusy = sending || registering || sendInFlight;

    const confirmDiscardSendDraft = useCallback(() => {
        if (!sendDialogDirty) return true;
        const confirmMessage = t(
            "viewer.diffReview.sendDialog.discardConfirm",
            "未送信のdiff review下書きを破棄しますか？\n\n元に戻せません。",
        );
        return window.confirm(confirmMessage);
    }, [sendDialogDirty, t]);

    const closeSendDialog = useCallback(() => {
        sendSuccessClosePendingRef.current = false;
        setSendDialogOpen(false);
        setSendDialogDirty(false);
    }, []);

    useEffect(() => {
        if (!sendDialogOpen) {
            sendSuccessClosePendingRef.current = false;
            setSendDialogDirty(false);
        }
    }, [sendDialogOpen]);

    useEffect(() => {
        if (selectedPaneId !== "" && !paneIds.has(selectedPaneId)) {
            if (sendInFlight) {
                return;
            }
            if (sendDialogOpen && !confirmDiscardSendDraft()) {
                return;
            }
            setSelectedPaneId("");
            closeSendDialog();
        }
    }, [closeSendDialog, confirmDiscardSendDraft, paneIds, selectedPaneId, sendDialogOpen, sendInFlight]);

    useEffect(() => {
        if (commentCount === 0 && sendDialogOpen) {
            if (sendInFlight || sendSuccessClosePendingRef.current) {
                return;
            }
            if (!confirmDiscardSendDraft()) {
                return;
            }
            closeSendDialog();
        }
    }, [closeSendDialog, commentCount, confirmDiscardSendDraft, sendDialogOpen, sendInFlight]);

    useEffect(() => {
        if (activeSession == null || activeMcpSnapshots !== undefined) {
            return;
        }
        if (useMCPStore.getState().sessionStates[activeSession]?.loading) {
            return;
        }

        let cancelled = false;
        beginMcpSessionLoad(activeSession);
        void api.ListMCPServers(activeSession)
            .then((result) => {
                if (cancelled) {
                    return;
                }
                setMcpSnapshots(activeSession, result ?? []);
                setMcpSessionError(activeSession, null);
            })
            .catch((err: unknown) => {
                if (cancelled) {
                    return;
                }
                const message = err instanceof Error ? err.message : String(err);
                console.warn("[diff-review] failed to load MCP snapshots", err);
                setMcpSessionError(activeSession, message);
            })
            .finally(() => {
                setMcpSessionLoading(activeSession, false);
            });

        return () => {
            cancelled = true;
        };
    }, [
        activeMcpSnapshots,
        activeSession,
        beginMcpSessionLoad,
        setMcpSessionError,
        setMcpSessionLoading,
        setMcpSnapshots,
    ]);

    const isSingleTaskRunnerEnabled = (activeMcpSnapshots ?? [])
        .some((snapshot) => isStrMcp(snapshot) && snapshot.enabled && snapshot.status === "running");
    const commentPlural = commentCount === 1 ? "" : "s";
    const commentCountLabel = t(
        "viewer.diffReview.commentCountLabel",
        "{count}件のレビューコメント",
        {count: commentCount, plural: commentPlural},
    );
    const clearLabel = t(
        "viewer.diffReview.clearComments",
        "{count}件のレビューコメントをクリア",
        {count: commentCount, plural: commentPlural},
    );
    const disabledActionTitle = selectedPaneIsAvailable
        ? undefined
        : t("viewer.diffReview.selectPaneFirst", "先にペインを選択してください");

    const handleSendClick = useCallback(() => {
        if (selectedPaneIsAvailable) {
            setSendDialogOpen(true);
        }
    }, [selectedPaneIsAvailable]);

    const handleAddToSingleTaskRunnerClick = useCallback(() => {
        if (selectedPaneIsAvailable) {
            void handleAddToSingleTaskRunner(selectedPaneId);
        }
    }, [selectedPaneId, selectedPaneIsAvailable, handleAddToSingleTaskRunner]);

    const handleDialogSend = useCallback(async (targetPaneId: string, payload: DiffReviewSendPayload) => {
        if (sendInFlight) return false;
        setSendInFlight(true);
        let sent = false;
        try {
            sent = await handleSend(targetPaneId, payload);
            if (sent) {
                sendSuccessClosePendingRef.current = true;
            }
            return sent;
        } finally {
            if (!sent) {
                sendSuccessClosePendingRef.current = false;
            }
            setSendInFlight(false);
        }
    }, [handleSend, sendInFlight]);

    const handleClearClick = useCallback(() => {
        if (activeSessionKey === "" || isBusy) return;
        const confirmMessage = t(
            "viewer.diffReview.clearConfirm",
            "{label}\n\n元に戻せません。",
            {label: clearLabel},
        );
        if (!window.confirm(confirmMessage)) {
            return;
        }
        clearCommentsForSession(activeSessionKey);
    }, [activeSessionKey, clearCommentsForSession, clearLabel, isBusy, t]);

    if (commentCount === 0 && !sendDialogOpen) return null;

    return (
        <>
            <span className="diff-review-badge" title={commentCountLabel}>
                {commentCount}
            </span>
            <select
                className="diff-review-pane-select"
                value={selectedPaneId}
                onChange={(e) => setSelectedPaneId(e.target.value)}
                title={t("viewer.diffReview.selectTargetPane", "送信先ペインを選択")}
            >
                <option value="">{t("viewer.diffReview.panePlaceholder", "ペイン...")}</option>
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
                disabled={!selectedPaneIsAvailable || isBusy}
                title={disabledActionTitle ?? t("viewer.diffReview.sendToPane", "diff review内容をペインへ送信")}
            >
                {sending ? t("viewer.diffReview.sending", "送信中...") : t("viewer.diffReview.send", "送信")}
            </button>
            {isSingleTaskRunnerEnabled && (
                <button
                    type="button"
                    className="viewer-header-btn diff-review-str-btn"
                    onClick={handleAddToSingleTaskRunnerClick}
                    disabled={!selectedPaneIsAvailable || isBusy}
                    title={disabledActionTitle ?? t(
                        "viewer.diffReview.registerToSingleTaskRunnerTitle",
                        "レビューコメントをSingle Task Runnerに登録",
                    )}
                >
                    {registering
                        ? t("viewer.diffReview.registeringToSingleTaskRunner", "登録中...")
                        : t("viewer.diffReview.registerToSingleTaskRunner", "Single Task Runnerに登録")}
                </button>
            )}
            <button
                type="button"
                className="viewer-header-btn diff-review-clear-btn"
                onClick={handleClearClick}
                disabled={isBusy}
                title={clearLabel}
                aria-label={clearLabel}
            >
                {t("viewer.diffReview.clear", "クリア")}
            </button>
            {sendError && <span className="diff-review-error">{sendError}</span>}
            <DiffReviewSendDialog
                open={sendDialogOpen}
                comments={comments}
                targetPaneId={selectedPaneId}
                targetPaneAvailable={selectedPaneIsAvailable}
                sending={dialogSending}
                sendError={sendError}
                onClose={closeSendDialog}
                onDirtyChange={setSendDialogDirty}
                onSend={handleDialogSend}
            />
        </>
    );
});
