import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {type DiffReviewComment, useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useViewerStore} from "../../viewerStore";
import {translate} from "../../../../i18n";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {logFrontendEventSafe} from "../../../../utils/logFrontendEventSafe";
import {buildReviewMarkdown, buildReviewSendMarkdown} from "./diffReviewMarkdown";
import {useDiffReviewApiSessionKey, useDiffReviewSessionKey} from "./diffReviewSession";
import {formatDiffReviewRangeLabel} from "./diffReviewRange";

export interface DiffReviewSendPayload {
    readonly message?: string;
    readonly comments?: readonly DiffReviewComment[];
}

function buildReviewTaskTitle(comment: DiffReviewComment): string {
    const rangeLabel = formatDiffReviewRangeLabel(
        {lineNum: comment.startLineNum, lineType: comment.startLineType},
        {lineNum: comment.endLineNum, lineType: comment.endLineType},
    );
    return `Review: ${comment.filePath} ${rangeLabel}`;
}

function translateReviewCount(key: string, defaultText: string, count: number): string {
    return translate(key, defaultText, {
        count,
        plural: count === 1 ? "" : "s",
    });
}

function notifyDiffReviewFailure(err: unknown, fallbackKey: string, defaultText: string): string {
    const message = toErrorMessage(err, translate(fallbackKey, defaultText));
    useNotificationStore.getState().addNotification(message, "error");
    logFrontendEventSafe("error", message, "DiffReview");
    return message;
}

export function useDiffReviewSend() {
    const activeSessionKey = useDiffReviewSessionKey();
    const activeApiSessionKey = useDiffReviewApiSessionKey();
    const allComments = useDiffReviewStore((s) => s.comments);
    const removeCommentsForSession = useDiffReviewStore((s) => s.removeCommentsForSession);
    const openView = useViewerStore((s) => s.openView);
    const [sending, setSending] = useState(false);
    const [registering, setRegistering] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const isMountedRef = useRef(true);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const latestApiSessionKeyRef = useRef(activeApiSessionKey);
    const isBusy = sending || registering;

    const comments = useMemo(
        () => allComments.filter((comment) => comment.sessionKey === activeSessionKey),
        [allComments, activeSessionKey],
    );

    useEffect(() => {
        latestSessionKeyRef.current = activeSessionKey;
    }, [activeSessionKey]);

    useEffect(() => {
        latestApiSessionKeyRef.current = activeApiSessionKey;
    }, [activeApiSessionKey]);

    useEffect(() => () => {
        isMountedRef.current = false;
    }, []);

    useEffect(() => {
        if (sendError == null) return;
        const id = setTimeout(() => setSendError(null), 5000);
        return () => clearTimeout(id);
    }, [sendError]);

    const handleSend = useCallback(
        async (targetPaneId: string, payload?: DiffReviewSendPayload): Promise<boolean> => {
            if (isBusy || !targetPaneId || activeSessionKey === "") return false;
            const payloadComments = payload?.comments ?? comments;
            const payloadMessage = payload?.message ?? "";
            const markdown = buildReviewSendMarkdown({message: payloadMessage, comments: payloadComments});
            if (markdown === "") return false;
            const capturedSessionKey = activeSessionKey;
            const sendingComments = payloadComments;
            setSending(true);
            setSendError(null);
            try {
                await api.SendDiffReview(targetPaneId, markdown);
                removeCommentsForSession(
                    sendingComments.map((comment) => comment.id),
                    capturedSessionKey,
                );
                useNotificationStore
                    .getState()
                    .addNotification(
                        sendingComments.length > 0
                            ? translateReviewCount(
                                "viewer.diffReview.notification.sentComments",
                                "{count}件のレビューコメントを送信しました。",
                                sendingComments.length,
                            )
                            : translate("viewer.diffReview.notification.sentReview", "レビューを送信しました。"),
                        "info",
                    );
                return true;
            } catch (err: unknown) {
                console.warn("[diff-review] SendDiffReview failed", err);
                const message = notifyDiffReviewFailure(
                    err,
                    "viewer.diffReview.error.sendFailed",
                    "レビュー送信に失敗しました。",
                );
                if (latestSessionKeyRef.current !== capturedSessionKey) {
                    return false;
                }
                setSendError(message);
                return false;
            } finally {
                if (isMountedRef.current) {
                    setSending(false);
                }
            }
        },
        [activeSessionKey, comments, isBusy, removeCommentsForSession],
    );

    const handleAddToSingleTaskRunner = useCallback(
        async (targetPaneId: string) => {
            if (!targetPaneId || activeSessionKey === "" || activeApiSessionKey === "" || comments.length === 0 || isBusy) return;
            const capturedSessionKey = activeSessionKey;
            const capturedApiSessionKey = activeApiSessionKey;
            const sendingComments = comments;
            const registeredCommentIDs: string[] = [];
            setRegistering(true);
            setSendError(null);

            try {
                // Registration is fail-fast: comments after the first failed enqueue stay pending.
                for (const comment of sendingComments) {
                    await api.AddSingleTaskRunnerItem(
                        capturedApiSessionKey,
                        buildReviewTaskTitle(comment),
                        buildReviewMarkdown([comment]),
                        targetPaneId,
                        false,
                        "",
                    );
                    registeredCommentIDs.push(comment.id);
                }

                if (latestSessionKeyRef.current !== capturedSessionKey
                    || latestApiSessionKeyRef.current !== capturedApiSessionKey) {
                    return;
                }
                if (registeredCommentIDs.length > 0) {
                    removeCommentsForSession(registeredCommentIDs, capturedSessionKey);
                }
                useNotificationStore
                    .getState()
                    .addNotification(
                        translateReviewCount(
                            "viewer.diffReview.notification.registeredTasks",
                            "{count}件のレビュータスクを登録しました。",
                            registeredCommentIDs.length,
                        ),
                        "info",
                    );
                openView("single-task-runner");
            } catch (err: unknown) {
                console.warn("[diff-review] AddSingleTaskRunnerItem failed", err);
                if (latestSessionKeyRef.current !== capturedSessionKey
                    || latestApiSessionKeyRef.current !== capturedApiSessionKey) {
                    notifyDiffReviewFailure(
                        err,
                        "viewer.diffReview.error.registerFailed",
                        "レビュータスクの登録に失敗しました。",
                    );
                    return;
                }
                if (registeredCommentIDs.length > 0) {
                    removeCommentsForSession(registeredCommentIDs, capturedSessionKey);
                }
                if (registeredCommentIDs.length > 0) {
                    useNotificationStore
                        .getState()
                        .addNotification(
                            translateReviewCount(
                                "viewer.diffReview.notification.partialRegisteredTasks",
                                "{count}件のレビュータスクを登録した後、登録に失敗しました。残りのコメントはdiff reviewに残りました。",
                                registeredCommentIDs.length,
                            ),
                            "warn",
                        );
                }
                const message = notifyDiffReviewFailure(
                    err,
                    "viewer.diffReview.error.registerFailed",
                    "レビュータスクの登録に失敗しました。",
                );
                setSendError(message);
            } finally {
                if (isMountedRef.current) {
                    setRegistering(false);
                }
            }
        },
        [activeApiSessionKey, activeSessionKey, comments, isBusy, removeCommentsForSession, openView],
    );

    return {
        commentCount: comments.length,
        comments,
        sending,
        registering,
        sendError,
        handleSend,
        handleAddToSingleTaskRunner,
    };
}
