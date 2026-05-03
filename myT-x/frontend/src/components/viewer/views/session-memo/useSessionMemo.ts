import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useI18n} from "../../../../i18n";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {buildSessionMemoDraftKey, useSessionMemoStore} from "../../../../stores/sessionMemoStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {shouldIgnoreSessionMutation, shouldIgnoreSessionRequest} from "../../../../utils/sessionGuard";

const SESSION_MEMO_MAX_BYTES = 1 << 20;
const utf8Encoder = new TextEncoder();

function getUtf8ByteLength(value: string): number {
    return utf8Encoder.encode(value).byteLength;
}

export function useSessionMemo() {
    const {t} = useI18n();
    const activeSession = useTmuxStore((state) => state.activeSession);
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((session) => session.name === activeSession) ?? null : null),
        [activeSession, sessions],
    );
    const activeSessionKey = useMemo(() => {
        if (!activeSession) {
            return "";
        }
        return activeSessionSnapshot ? buildSessionMemoDraftKey(activeSessionSnapshot.name, activeSessionSnapshot.id) : "";
    }, [activeSession, activeSessionSnapshot]);
    const draft = useSessionMemoStore((state) => activeSessionKey ? state.drafts[activeSessionKey] : undefined);
    const initializeMemo = useSessionMemoStore((state) => state.initializeMemo);
    const setMemoContent = useSessionMemoStore((state) => state.setMemoContent);
    const markSaved = useSessionMemoStore((state) => state.markSaved);
    const moveDraft = useSessionMemoStore((state) => state.moveDraft);
    const addNotification = useNotificationStore((state) => state.addNotification);

    const isMountedRef = useRef(true);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const savingRef = useRef(false);
    const loadRequestTokenRef = useRef(0);
    const previousActiveSessionRef = useRef<{ readonly id: number; readonly key: string } | null>(null);
    const [loading, setLoading] = useState(false);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);

    latestSessionKeyRef.current = activeSessionKey;
    savingRef.current = saving;

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    useEffect(() => {
        if (!activeSession) {
            previousActiveSessionRef.current = null;
            return;
        }
        if (!activeSessionSnapshot || !activeSessionKey) {
            return;
        }
        const previous = previousActiveSessionRef.current;
        if (previous && previous.id === activeSessionSnapshot.id && previous.key !== activeSessionKey) {
            moveDraft(previous.key, activeSessionKey);
        }
        previousActiveSessionRef.current = {
            id: activeSessionSnapshot.id,
            key: activeSessionKey,
        };
    }, [activeSessionKey, activeSessionSnapshot, moveDraft]);

    const refresh = useCallback(async (
        force = false,
        capturedSessionKey: string = latestSessionKeyRef.current,
    ) => {
        if (!activeSession || !capturedSessionKey || savingRef.current) {
            return;
        }
        const existingDraft = useSessionMemoStore.getState().drafts[capturedSessionKey];
        if (!force && existingDraft?.loaded) {
            return;
        }

        const capturedSession = activeSession;
        const requestToken = ++loadRequestTokenRef.current;
        setLoading(true);
        setError(null);

        try {
            const content = await api.LoadSessionMemo(capturedSession);
            if (
                shouldIgnoreSessionRequest(
                    capturedSessionKey,
                    requestToken,
                    isMountedRef,
                    latestSessionKeyRef,
                    loadRequestTokenRef,
                )
            ) {
                return;
            }
            initializeMemo(capturedSessionKey, content, force);
        } catch (err: unknown) {
            console.warn("[session-memo] load failed", err);
            if (
                shouldIgnoreSessionRequest(
                    capturedSessionKey,
                    requestToken,
                    isMountedRef,
                    latestSessionKeyRef,
                    loadRequestTokenRef,
                )
            ) {
                return;
            }
            setError(toErrorMessage(err, t("viewer.sessionMemo.error.load", "セッションメモの読み込みに失敗しました。")));
            if (force) {
                addNotification(
                    t("viewer.sessionMemo.notification.refreshFailed", "Failed to reload the session memo."),
                    "warn",
                );
            }
            throw err;
        } finally {
            if (isMountedRef.current) {
                setLoading(false);
            }
        }
    }, [activeSession, initializeMemo, addNotification, t]);

    useEffect(() => {
        void refresh(false, activeSessionKey).catch(() => {
            // The hook already updated the user-visible error state.
        });
    }, [activeSessionKey, refresh]);

    const content = draft?.content ?? "";
    const savedContent = draft?.savedContent ?? "";
    const isDirty = content !== savedContent;

    const updateContent = useCallback((nextContent: string) => {
        if (!activeSessionKey) {
            return;
        }
        if (getUtf8ByteLength(nextContent) > SESSION_MEMO_MAX_BYTES) {
            setError(t("viewer.sessionMemo.error.size", "Session memo must be 1 MiB or smaller."));
            return;
        }
        setError(null);
        setMemoContent(activeSessionKey, nextContent);
    }, [activeSessionKey, setMemoContent, t]);

    const save = useCallback(async () => {
        if (!activeSession || !activeSessionKey || saving) {
            return;
        }

        const capturedSession = activeSession;
        const capturedSessionKey = activeSessionKey;
        const memo = useSessionMemoStore.getState().drafts[capturedSessionKey]?.content ?? "";
        if (getUtf8ByteLength(memo) > SESSION_MEMO_MAX_BYTES) {
            const message = t("viewer.sessionMemo.error.size", "Session memo must be 1 MiB or smaller.");
            setError(message);
            addNotification(message, "warn");
            return;
        }
        loadRequestTokenRef.current += 1;
        setSaving(true);
        setError(null);

        try {
            await api.SaveSessionMemo(capturedSession, memo);
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            markSaved(capturedSessionKey, memo);
            addNotification(t("viewer.sessionMemo.notification.saved", "セッションメモを保存しました。"), "info");
        } catch (err: unknown) {
            console.warn("[session-memo] save failed", err);
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            const message = toErrorMessage(err, t("viewer.sessionMemo.error.save", "セッションメモの保存に失敗しました。"));
            setError(message);
            addNotification(message, "warn");
        } finally {
            if (isMountedRef.current) {
                setSaving(false);
            }
        }
    }, [activeSession, activeSessionKey, saving, markSaved, addNotification, t]);

    return {
        activeSession,
        content,
        loading,
        saving,
        error,
        isDirty,
        updateContent,
        refresh,
        save,
        clearError: () => setError(null),
    };
}
