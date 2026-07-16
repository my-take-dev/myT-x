import {useEffect, useRef} from "react";
import {api} from "../../api";
import {useInputHistoryStore} from "../../stores/inputHistoryStore";
import {useTmuxStore} from "../../stores/tmuxStore";
import {logFrontendEventSafe} from "../../utils/logFrontendEventSafe";
import {cleanupEventListeners, createEventSubscriber} from "./eventHelpers";

// SUG-20: Module-level constants — shared across renders, avoids re-creation inside useEffect.
const DEBOUNCE_MS = 80;
const FETCH_RETRY_DELAY_MS = 250;

// Payload types are compile-time documentation only.
interface InputHistoryEventMap {
    // Ping-only event for input history — same pattern as session-log-updated.
    "app:input-history-updated": null;
}

/**
 * Subscribes to input history update pings and fetches the full history snapshot.
 *
 * Uses the same "ping + fetch" pattern as useSessionLogSync: lightweight backend
 * ping triggers a scoped snapshot fetch with debounce, retry, and stale-response
 * protection via a monotonic fetch counter.
 */
export function useInputHistorySync(): void {
    const activeSession = useTmuxStore((state) => state.activeSession);
    const isMountedRef = useRef(true);
    const fetchGenerationRef = useRef(0);

    useEffect(() => {
        isMountedRef.current = true;
        const effectGeneration = ++fetchGenerationRef.current;
        const cleanupFns: Array<() => void> = [];
        const onEvent = createEventSubscriber<InputHistoryEventMap>(cleanupFns);

        let debounceTimer: ReturnType<typeof setTimeout> | null = null;
        let retryTimer: ReturnType<typeof setTimeout> | null = null;
        let fetchSeq = 0;

        const fetchInputHistory = (attempt = 0) => {
            if (!isMountedRef.current) return;
            if (activeSession == null || activeSession.trim() === "") {
                useInputHistoryStore.getState().setSnapshot({scope_key: "", entries: []});
                return;
            }
            const seq = ++fetchSeq;
            const capturedActiveSession = activeSession;
            void api.GetInputHistoryForSession(capturedActiveSession)
                .then((result) => {
                    if (
                        !isMountedRef.current ||
                        effectGeneration !== fetchGenerationRef.current ||
                        seq !== fetchSeq ||
                        useTmuxStore.getState().activeSession !== capturedActiveSession
                    ) return;
                    if (retryTimer != null) {
                        clearTimeout(retryTimer);
                        retryTimer = null;
                    }
                    useInputHistoryStore.getState().setSnapshot(result ?? {scope_key: "", entries: []});
                })
                .catch((err: unknown) => {
                    if (
                        !isMountedRef.current ||
                        effectGeneration !== fetchGenerationRef.current ||
                        seq !== fetchSeq ||
                        useTmuxStore.getState().activeSession !== capturedActiveSession
                    ) return;
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] GetInputHistoryForSession failed:", err, "attempt:", attempt + 1);
                    }
                    if (attempt >= 1) {
                        logFrontendEventSafe("warn", "GetInputHistoryForSession failed after retry", "frontend/sync");
                        return;
                    }
                    if (retryTimer != null) {
                        clearTimeout(retryTimer);
                    }
                    retryTimer = setTimeout(() => {
                        retryTimer = null;
                        fetchInputHistory(attempt + 1);
                    }, FETCH_RETRY_DELAY_MS);
                });
        };

        onEvent("app:input-history-updated", () => {
            if (!isMountedRef.current) return;
            if (debounceTimer != null) {
                clearTimeout(debounceTimer);
            }
            debounceTimer = setTimeout(fetchInputHistory, DEBOUNCE_MS);
        });

        // Subscribe first, then load initial snapshot to avoid missing entries.
        fetchInputHistory();

        return () => {
            fetchGenerationRef.current++;
            isMountedRef.current = false;
            if (debounceTimer != null) {
                clearTimeout(debounceTimer);
                debounceTimer = null;
            }
            if (retryTimer != null) {
                clearTimeout(retryTimer);
                retryTimer = null;
            }
            cleanupEventListeners(cleanupFns);
        };
    }, [activeSession]);
}
