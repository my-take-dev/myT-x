import {memo, type KeyboardEvent, useCallback, useEffect, useId, useMemo, useRef, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {formatDiffReviewRangeLabel} from "./diffReviewRange";
import type {DiffReviewSendPayload} from "./useDiffReviewSend";

interface DraftComment {
    readonly comment: DiffReviewComment;
    readonly text: string;
    readonly editing: boolean;
}

interface TrimmedDraftComment extends DraftComment {
    readonly trimmedText: string;
}

interface DiffReviewSendDialogProps {
    readonly open: boolean;
    readonly comments: readonly DiffReviewComment[];
    readonly targetPaneId: string;
    readonly sending: boolean;
    readonly sendError: string | null;
    readonly onClose: () => void;
    readonly onDirtyChange?: (dirty: boolean) => void;
    readonly onSend: (targetPaneId: string, payload: DiffReviewSendPayload) => Promise<boolean>;
    readonly targetPaneAvailable?: boolean;
}

function createDraftComments(comments: readonly DiffReviewComment[]): DraftComment[] {
    return comments.map((comment) => ({
        comment,
        text: comment.commentText,
        editing: false,
    }));
}

export const DiffReviewSendDialog = memo(function DiffReviewSendDialog({
    open,
    comments,
    targetPaneId,
    sending,
    sendError,
    onClose,
    onDirtyChange,
    onSend,
    targetPaneAvailable = true,
}: DiffReviewSendDialogProps) {
    const {t} = useI18n();
    const titleId = useId();
    const subtitleId = useId();
    const descriptionId = useId();
    const panelRef = useRef<HTMLDivElement | null>(null);
    const messageRef = useRef<HTMLTextAreaElement | null>(null);
    const wasOpenRef = useRef(false);
    const [message, setMessage] = useState("");
    const [draftComments, setDraftComments] = useState<DraftComment[]>([]);
    const [openingDraftCount, setOpeningDraftCount] = useState(0);

    useEffect(() => {
        if (open && !wasOpenRef.current) {
            // Opening creates a sealed send draft. Later store updates stay pending outside this dialog.
            setMessage("");
            setDraftComments(createDraftComments(comments));
            setOpeningDraftCount(comments.length);
        }
        else if (!open && wasOpenRef.current) {
            setMessage("");
            setDraftComments([]);
            setOpeningDraftCount(0);
        }
        wasOpenRef.current = open;
    }, [open]);

    useEffect(() => {
        if (!open) return;
        messageRef.current?.focus();
    }, [open]);

    const updateDraftText = useCallback((commentId: string, text: string) => {
        setDraftComments((current) => current.map((draft) => (
            draft.comment.id === commentId ? {...draft, text} : draft
        )));
    }, []);

    const toggleEdit = useCallback((commentId: string) => {
        setDraftComments((current) => current.map((draft) => (
            draft.comment.id === commentId ? {...draft, editing: !draft.editing} : draft
        )));
    }, []);

    const deleteDraft = useCallback((commentId: string) => {
        setDraftComments((current) => current.filter((draft) => draft.comment.id !== commentId));
    }, []);

    const trimmedDraftComments = useMemo<TrimmedDraftComment[]>(
        () => draftComments.map((draft) => ({...draft, trimmedText: draft.text.trim()})),
        [draftComments],
    );
    const hasUnsavedDraft = useMemo(
        () => message !== ""
            || draftComments.length !== openingDraftCount
            || draftComments.some((draft) => draft.text !== draft.comment.commentText),
        [draftComments, message, openingDraftCount],
    );
    const hasInvalidComment = trimmedDraftComments.some((draft) => draft.trimmedText === "");
    const hasContent = message.trim() !== "" || draftComments.length > 0;
    const canSend = targetPaneId !== "" && targetPaneAvailable && hasContent && !hasInvalidComment && !sending;

    const sendPayload = useMemo<DiffReviewSendPayload>(() => ({
        message,
        comments: trimmedDraftComments.map((draft) => ({
            ...draft.comment,
            commentText: draft.trimmedText,
        })),
    }), [message, trimmedDraftComments]);

    useEffect(() => {
        onDirtyChange?.(open && hasUnsavedDraft);
    }, [hasUnsavedDraft, onDirtyChange, open]);

    const requestClose = useCallback(() => {
        if (sending) return;
        if (hasUnsavedDraft) {
            const confirmMessage = t(
                "viewer.diffReview.sendDialog.discardConfirm",
                "未送信のdiff review下書きを破棄しますか？\n\n元に戻せません。",
            );
            if (!window.confirm(confirmMessage)) {
                return;
            }
        }
        onClose();
    }, [hasUnsavedDraft, onClose, sending, t]);

    const handleSend = useCallback(async () => {
        if (!canSend) return;
        const sent = await onSend(targetPaneId, sendPayload);
        if (sent) {
            onClose();
        }
    }, [canSend, onClose, onSend, sendPayload, targetPaneId]);

    const handleKeyDown = useCallback(
        (event: KeyboardEvent<HTMLDivElement>) => {
            if (event.key === "Escape") {
                event.stopPropagation();
                if (!sending) {
                    requestClose();
                }
                return;
            }
            if (event.key !== "Tab" || panelRef.current == null) {
                return;
            }
            event.stopPropagation();
            const focusable = panelRef.current.querySelectorAll<HTMLElement>(
                'button:not(:disabled), [href], input, select, textarea, [contenteditable="true"], [tabindex]:not([tabindex="-1"])',
            );
            if (focusable.length === 0) return;
            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            if (event.shiftKey) {
                if (document.activeElement === first) {
                    event.preventDefault();
                    last.focus();
                }
                return;
            }
            if (document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        },
        [requestClose, sending],
    );

    if (!open || targetPaneId === "") return null;

    return (
        <div className="modal-overlay diff-review-send-overlay" onClick={sending ? undefined : requestClose}>
            <div
                ref={panelRef}
                className="modal-panel diff-review-send-dialog"
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
                aria-describedby={descriptionId}
                onKeyDown={handleKeyDown}
                onClick={(event) => event.stopPropagation()}
            >
                <div className="modal-header diff-review-send-header">
                    <div>
                        <h2 id={titleId}>{t("viewer.diffReview.sendDialog.title", "レビュー送信")}</h2>
                        <p id={subtitleId} className="diff-review-send-subtitle">
                            {t("viewer.diffReview.sendDialog.targetPane", "送信先: {paneId}", {paneId: targetPaneId})}
                        </p>
                        <p id={descriptionId} className="diff-review-send-description">
                            {t(
                                "viewer.diffReview.sendDialog.description",
                                "送信するdiff review内容を確認・編集します。",
                            )}
                        </p>
                    </div>
                    <button
                        type="button"
                        className="modal-btn primary diff-review-send-primary"
                        onClick={handleSend}
                        disabled={!canSend}
                    >
                        {sending
                            ? t("viewer.diffReview.sending", "送信中...")
                            : t("viewer.diffReview.send", "送信")}
                    </button>
                </div>
                <div className="modal-body diff-review-send-body">
                    <label className="diff-review-send-message-field">
                        <span className="form-label">
                            {t("viewer.diffReview.sendDialog.messageLabel", "任意メッセージ")}
                        </span>
                        <textarea
                            ref={messageRef}
                            className="form-input diff-review-send-message"
                            value={message}
                            onChange={(event) => setMessage(event.target.value)}
                            placeholder={t(
                                "viewer.diffReview.sendDialog.messagePlaceholder",
                                "ペインへ送信するメッセージ...",
                            )}
                        />
                    </label>
                    <div className="diff-review-send-comment-list" aria-label={t(
                        "viewer.diffReview.sendDialog.commentsLabel",
                        "レビューコメント",
                    )}>
                        {draftComments.length === 0 ? (
                            <div className="diff-review-send-empty">
                                {t("viewer.diffReview.sendDialog.noComments", "この送信に含めるコメントはありません。")}
                            </div>
                        ) : trimmedDraftComments.map((draft) => {
                            const rangeLabel = formatDiffReviewRangeLabel(
                                {lineNum: draft.comment.startLineNum, lineType: draft.comment.startLineType},
                                {lineNum: draft.comment.endLineNum, lineType: draft.comment.endLineType},
                            );
                            const isInvalid = draft.trimmedText === "";
                            return (
                                <article key={draft.comment.id} className="diff-review-send-comment">
                                    <div className="diff-review-send-comment-header">
                                        <div className="diff-review-send-comment-location">
                                            <span className="diff-review-send-file">{draft.comment.filePath}</span>
                                            <span className="diff-review-send-range">{rangeLabel}</span>
                                        </div>
                                        <div className="diff-review-send-comment-actions">
                                            <button
                                                type="button"
                                                className="diff-comment-btn"
                                                onClick={() => toggleEdit(draft.comment.id)}
                                            >
                                                {draft.editing
                                                    ? t("viewer.diffReview.sendDialog.done", "完了")
                                                    : t("viewer.diffReview.sendDialog.edit", "編集")}
                                            </button>
                                            <button
                                                type="button"
                                                className="diff-comment-btn diff-review-send-delete"
                                                onClick={() => deleteDraft(draft.comment.id)}
                                            >
                                                {t("viewer.diffReview.sendDialog.delete", "削除")}
                                            </button>
                                        </div>
                                    </div>
                                    <pre className="diff-review-send-code">{draft.comment.lineContent}</pre>
                                    {draft.editing ? (
                                        <>
                                            <textarea
                                                className={`diff-comment-textarea diff-review-send-comment-editor${isInvalid ? " is-invalid" : ""}`}
                                                value={draft.text}
                                                onChange={(event) => updateDraftText(draft.comment.id, event.target.value)}
                                                aria-invalid={isInvalid}
                                            />
                                            {isInvalid && (
                                                <div className="form-error">
                                                    {t(
                                                        "viewer.diffReview.sendDialog.emptyCommentError",
                                                        "コメント本文を入力するか、この行を削除してください。",
                                                    )}
                                                </div>
                                            )}
                                        </>
                                    ) : (
                                        <div className="diff-review-send-comment-text">{draft.text}</div>
                                    )}
                                </article>
                            );
                        })}
                    </div>
                    {sendError != null && <div className="form-error diff-review-send-error">{sendError}</div>}
                </div>
                <div className="modal-footer diff-review-send-footer">
                    <button type="button" className="modal-btn" onClick={requestClose} disabled={sending}>
                        {t("common.cancel", "キャンセル")}
                    </button>
                </div>
            </div>
        </div>
    );
});
