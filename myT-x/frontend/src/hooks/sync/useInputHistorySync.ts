import {useEffect, useRef} from "react";
import {api} from "../../api";
import {useInputHistoryStore} from "../../stores/inputHistoryStore";
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
 * ping triggers a full snapshot fetch with debounce, retry, and stale-response
 * protection via a monotonic fetch counter.
 */
export function useInputHistorySync(): void {
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;
        const cleanupFns: Array<() => void> = [];
        const onEvent = createEventSubscriber<InputHistoryEventMap>(cleanupFns);

        let debounceTimer: ReturnType<typeof setTimeout> | null = null;
        let retryTimer: ReturnType<typeof setTimeout> | null = null;
        let fetchSeq = 0;

        const fetchInputHistory = (attempt = 0) => {
            if (!isMountedRef.current) return;
            const seq = ++fetchSeq;
            void api.GetInputHistory()
                .then((result) => {
                    if (!isMountedRef.current || seq !== fetchSeq) return;
                    if (retryTimer != null) {
                        clearTimeout(retryTimer);
                        retryTimer = null;
                    }
                    useInputHistoryStore.getState().setEntries(result ?? []);
                })
                .catch((err: unknown) => {
                    if (!isMountedRef.current || seq !== fetchSeq) return;
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] GetInputHistory failed:", err, "attempt:", attempt + 1);
                    }
                    if (attempt >= 1) {
                        logFrontendEventSafe("warn", "GetInputHistory failed after retry", "frontend/sync");
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
    }, []);
}
