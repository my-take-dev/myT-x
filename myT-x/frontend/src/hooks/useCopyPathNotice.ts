import {useCallback, useEffect, useRef, useState} from "react";
import {type CopyNoticeState, writeClipboardText} from "../utils/clipboardUtils";
import {notifyClipboardFailure} from "../utils/notifyUtils";

const DEFAULT_COPY_NOTICE_RESET_MS = 1500;

interface UseCopyPathNoticeOptions {
    resetDelayMs?: number;
    logPrefix: string;
}

interface UseCopyPathNoticeResult {
    copyState: CopyNoticeState;
    copyPath: (path?: string | null) => void;
}

export function useCopyPathNotice(
    resetKey: string | null | undefined,
    options: UseCopyPathNoticeOptions,
): UseCopyPathNoticeResult {
    const {logPrefix, resetDelayMs = DEFAULT_COPY_NOTICE_RESET_MS} = options;
    const [copyState, setCopyState] = useState<CopyNoticeState>("idle");
    const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const requestIdRef = useRef(0);
    // defaultPathRef is synced via useEffect (commit-phase), so there is a brief window
    // between resetKey changing and the ref updating. In practice this does not cause issues
    // because users cannot click copyPath during the synchronous render phase.
    const defaultPathRef = useRef<string | null | undefined>(resetKey);

    const clearResetTimer = useCallback(() => {
        if (resetTimerRef.current !== null) {
            clearTimeout(resetTimerRef.current);
            resetTimerRef.current = null;
        }
    }, []);

    const scheduleReset = useCallback((requestId: number) => {
        clearResetTimer();
        resetTimerRef.current = setTimeout(() => {
            resetTimerRef.current = null;
            if (requestIdRef.current !== requestId) return;
            setCopyState("idle");
        }, resetDelayMs);
    }, [clearResetTimer, resetDelayMs]);

    // Sync defaultPathRef when resetKey changes. The ref is initialized with resetKey
    // in useRef(), so this effect is only needed for subsequent updates. In StrictMode,
    // the double-effect execution is harmless: the first cleanup is a no-op, and the
    // second effect correctly updates the ref to the current resetKey.
    useEffect(() => {
        defaultPathRef.current = resetKey;
        requestIdRef.current += 1;
        setCopyState("idle");
        clearResetTimer();
    }, [resetKey, clearResetTimer]);

    useEffect(() => {
        return () => {
            clearResetTimer();
        };
    }, [clearResetTimer]);

    const copyPath = useCallback((path?: string | null) => {
        const targetPath = path ?? defaultPathRef.current;
        if (!targetPath) return;
        const requestId = requestIdRef.current + 1;
        requestIdRef.current = requestId;
        void writeClipboardText(targetPath)
            .then(() => {
                if (requestIdRef.current !== requestId) return;
                setCopyState("copied");
                scheduleReset(requestId);
            })
            .catch((err: unknown) => {
                if (requestIdRef.current !== requestId) return;
                setCopyState("failed");
                scheduleReset(requestId);
                notifyClipboardFailure();
                console.warn(`${logPrefix} clipboard write failed`, err);
            });
    }, [logPrefix, scheduleReset]);

    return {copyState, copyPath};
}
