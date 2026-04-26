import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {notifyAndLog} from "../../../../utils/notifyUtils";
import {buildReviewMarkdown} from "./diffReviewMarkdown";
import {useDiffReviewSessionKey} from "./diffReviewSession";

export function useDiffReviewSend() {
    const activeSessionKey = useDiffReviewSessionKey();
    const allComments = useDiffReviewStore((s) => s.comments);
    const removeCommentsForSession = useDiffReviewStore((s) => s.removeCommentsForSession);
    const [sending, setSending] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const isMountedRef = useRef(true);
    const latestSessionKeyRef = useRef(activeSessionKey);

    const comments = useMemo(
        () => allComments.filter((comment) => comment.sessionKey === activeSessionKey),
        [allComments, activeSessionKey],
    );

    useEffect(() => {
        latestSessionKeyRef.current = activeSessionKey;
    }, [activeSessionKey]);

    useEffect(() => () => {
        isMountedRef.current = false;
    }, []);

    useEffect(() => {
        if (sendError == null) return;
        const id = setTimeout(() => setSendError(null), 5000);
        return () => clearTimeout(id);
    }, [sendError]);

    const handleSend = useCallback(
        async (targetPaneId: string) => {
            if (!targetPaneId || activeSessionKey === "" || comments.length === 0 || sending) return;
            const capturedSessionKey = activeSessionKey;
            const sendingComments = comments;
            setSending(true);
            setSendError(null);
            try {
                const markdown = buildReviewMarkdown(sendingComments);
                await api.SendDiffReview(targetPaneId, markdown);
                removeCommentsForSession(
                    sendingComments.map((comment) => comment.id),
                    capturedSessionKey,
                );
                useNotificationStore
                    .getState()
                    .addNotification(
                        `Sent ${sendingComments.length} review comment${sendingComments.length === 1 ? "" : "s"}.`,
                        "info",
                    );
            } catch (err: unknown) {
                console.warn("[diff-review] SendDiffReview failed", err);
                if (latestSessionKeyRef.current !== capturedSessionKey) {
                    return;
                }
                notifyAndLog("Send diff review", "error", err, "DiffReview");
                setSendError(toErrorMessage(err, "Failed to send review."));
            } finally {
                if (isMountedRef.current) {
                    setSending(false);
                }
            }
        },
        [activeSessionKey, comments, sending, removeCommentsForSession],
    );

    return {commentCount: comments.length, sending, sendError, handleSend};
}
