import {useEffect, useRef} from "react";
import {api} from "../../api";
import {useErrorLogStore} from "../../stores/errorLogStore";
import {logFrontendEventSafe} from "../../utils/logFrontendEventSafe";
import {cleanupEventListeners, createEventSubscriber} from "./eventHelpers";

// SUG-20: Module-level constants — shared across renders, avoids re-creation inside useEffect.
const DEBOUNCE_MS = 80;
const FETCH_RETRY_DELAY_MS = 250;

// Payload types are compile-time documentation only.
interface SessionLogEventMap {
    // NOTE(A-0): Ping-only event — no payload. Frontend fetches full snapshot
    // via GetSessionErrorLog() on receipt. This eliminates data loss from throttling.
    "app:session-log-updated": null;
}

/**
 * Subscribes to session log update pings and fetches the full error log snapshot.
 *
 * Uses the "ping + fetch" pattern: the backend sends a lightweight ping (no payload)
 * when new log entries are available. The frontend fetches the full snapshot via
 * GetSessionErrorLog(). Debounce prevents flooding when multiple entries arrive
 * in quick succession.
 *
 * IMP-13: Monotonic fetch counter prevents stale (older) responses from
 * overwriting newer data when multiple requests are in flight concurrently.
 */
export function useSessionLogSync(): void {
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;
        const cleanupFns: Array<() => void> = [];
        const onEvent = createEventSubscriber<SessionLogEventMap>(cleanupFns);

        let debounceTimer: ReturnType<typeof setTimeout> | null = null;
        let retryTimer: ReturnType<typeof setTimeout> | null = null;
        // IMP-13: Monotonic fetch counter — prevents stale responses from
        // overwriting newer data when multiple requests are in flight.
        let fetchSeq = 0;

        const fetchErrorLog = (attempt = 0) => {
            // IMP-13: Skip fetch if component is already unmounted (#96 timer guard).
            if (!isMountedRef.current) return;
            const seq = ++fetchSeq;
            void api.GetSessionErrorLog()
                .then((result) => {
                    // IMP-13: Discard result if a newer request was issued or component unmounted.
                    if (!isMountedRef.current || seq !== fetchSeq) return;
                    if (retryTimer != null) {
                        clearTimeout(retryTimer);
                        retryTimer = null;
                    }
                    useErrorLogStore.getState().setEntries(result ?? []);
                })
                .catch((err: unknown) => {
                    if (!isMountedRef.current || seq !== fetchSeq) return;
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] GetSessionErrorLog failed:", err, "attempt:", attempt + 1);
                    }
                    if (attempt >= 1) {
                        logFrontendEventSafe("warn", "GetSessionErrorLog failed after retry", "frontend/sync");
                        return;
                    }
                    if (retryTimer != null) {
                        clearTimeout(retryTimer);
                    }
                    retryTimer = setTimeout(() => {
                        retryTimer = null;
                        fetchErrorLog(attempt + 1);
                    }, FETCH_RETRY_DELAY_MS);
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] scheduled GetSessionErrorLog retry", "delayMs:", FETCH_RETRY_DELAY_MS);
                    }
                });
        };

        onEvent("app:session-log-updated", () => {
            // IMP-13: Guard against debounce timer firing after unmount.
            if (!isMountedRef.current) return;
            if (debounceTimer != null) {
                clearTimeout(debounceTimer);
            }
            debounceTimer = setTimeout(fetchErrorLog, DEBOUNCE_MS);
        });

        // IMP-02 + IMP-15: Load initial error log entries after subscribing to the
        // ping event. Reuses fetchErrorLog to eliminate duplicated Promise chain.
        // By subscribing first, then loading the snapshot, we ensure no entries
        // are missed: pings arriving during the fetch trigger setEntries which
        // always replaces with the latest full snapshot. The fetch counter (IMP-13)
        // ensures only the latest response is applied.
        fetchErrorLog();

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
