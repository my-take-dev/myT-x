import {api} from "../api";

type FrontendLogAPI = {
    LogFrontendEvent?: (level: string, msg: string, source: string) => Promise<void>;
};

/**
 * Fire-and-forget wrapper for LogFrontendEvent.
 * Defensively handles missing bindings, sync throws, and async rejections.
 */
export function logFrontendEventSafe(level: string, msg: string, source: string): void {
    const maybeAPI = api as FrontendLogAPI;
    if (typeof maybeAPI.LogFrontendEvent !== "function") {
        return;
    }
    try {
        void maybeAPI.LogFrontendEvent(level, msg, source).catch(() => {
            // Silently discard: logging must not throw during error recovery.
        });
    } catch {
        // Silently discard: logging must never crash the caller path.
    }
}
