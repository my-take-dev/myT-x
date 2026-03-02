import {useCallback, useEffect, useRef, useState} from "react";

const DEFAULT_COPY_FEEDBACK_MS = 1500;

interface UseCopyFeedbackResult {
    allCopied: boolean;
    copiedEntrySeq: number | null;
    markAllCopied: () => void;
    markEntryCopied: (seq: number) => void;
}

/**
 * Shared copy-feedback state ("Copied!" badges) for list views.
 */
export function useCopyFeedback(durationMs = DEFAULT_COPY_FEEDBACK_MS): UseCopyFeedbackResult {
    const [allCopied, setAllCopied] = useState(false);
    const [copiedEntrySeq, setCopiedEntrySeq] = useState<number | null>(null);
    const allCopyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const entryCopyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    const clearAllCopyTimer = useCallback(() => {
        if (allCopyTimerRef.current !== null) {
            clearTimeout(allCopyTimerRef.current);
            allCopyTimerRef.current = null;
        }
    }, []);

    const clearEntryCopyTimer = useCallback(() => {
        if (entryCopyTimerRef.current !== null) {
            clearTimeout(entryCopyTimerRef.current);
            entryCopyTimerRef.current = null;
        }
    }, []);

    const markAllCopied = useCallback(() => {
        clearAllCopyTimer();
        setAllCopied(true);
        allCopyTimerRef.current = setTimeout(() => {
            allCopyTimerRef.current = null;
            setAllCopied(false);
        }, durationMs);
    }, [clearAllCopyTimer, durationMs]);

    const markEntryCopied = useCallback((seq: number) => {
        clearEntryCopyTimer();
        setCopiedEntrySeq(seq);
        entryCopyTimerRef.current = setTimeout(() => {
            entryCopyTimerRef.current = null;
            setCopiedEntrySeq((prev) => (prev === seq ? null : prev));
        }, durationMs);
    }, [clearEntryCopyTimer, durationMs]);

    useEffect(() => {
        return () => {
            clearAllCopyTimer();
            clearEntryCopyTimer();
        };
    }, [clearAllCopyTimer, clearEntryCopyTimer]);

    return {
        allCopied,
        copiedEntrySeq,
        markAllCopied,
        markEntryCopied,
    };
}
