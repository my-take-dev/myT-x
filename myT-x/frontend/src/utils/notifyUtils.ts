import {useNotificationStore, type Notification} from "../stores/notificationStore";
import {toErrorMessage} from "./errorUtils";
import {logFrontendEventSafe} from "./logFrontendEventSafe";

// ---------------------------------------------------------------------------
// Consecutive failure counter for background sync/polling operations
// ---------------------------------------------------------------------------

/**
 * A counter that tracks consecutive failures and fires a callback only after
 * reaching a threshold, with cooldown to prevent notification spam.
 *
 * Use for background sync/polling operations where transient failures should
 * be silently recovered (next poll succeeds) but persistent failures need user
 * attention.
 */
export interface ConsecutiveFailureCounter {
    /** Record a failure. If count reaches threshold and cooldown allows, calls onThreshold. */
    recordFailure: (onThreshold: () => void) => void;
    /** Record a success. Resets the consecutive failure count. */
    recordSuccess: () => void;
}

/**
 * Creates a counter that fires a callback after {@link threshold} consecutive
 * failures. When the threshold is reached AND the cooldown has elapsed, the
 * callback fires and the count resets to 0. When the threshold is reached but
 * cooldown is still active, the count is NOT reset — failures continue to
 * accumulate so that the callback fires immediately once the cooldown expires.
 *
 * NOTE: Module-level state resets on Vite HMR reload. This is acceptable —
 * the counter self-corrects on the next successful cycle.
 *
 * @param threshold - Number of consecutive failures before notification.
 * @param cooldownMs - Minimum interval between threshold notifications. Defaults to 30 s.
 */
export function createConsecutiveFailureCounter(
    threshold: number,
    cooldownMs = 30_000,
): ConsecutiveFailureCounter {
    let count = 0;
    let lastNotifiedAt = 0;
    return {
        recordFailure: (onThreshold) => {
            count++;
            if (count >= threshold) {
                const now = Date.now();
                if (now - lastNotifiedAt >= cooldownMs) {
                    onThreshold();
                    lastNotifiedAt = now;
                    count = 0;
                }
                // When cooldown prevents firing, count stays >= threshold.
                // The next recordFailure call after cooldown expires will
                // re-check and fire immediately.
            }
        },
        recordSuccess: () => {
            count = 0;
        },
    };
}

/** Notify the user of a clipboard write failure. Safe to call outside React components. */
export function notifyClipboardFailure(): void {
    useNotificationStore.getState().addNotification("Failed to copy to clipboard.", "warn");
    logFrontendEventSafe("warn", "Failed to copy to clipboard", "Clipboard");
}

/** Notify the user of a clipboard read (paste) failure. Safe to call outside React components. */
export function notifyPasteFailure(): void {
    useNotificationStore.getState().addNotification("Failed to paste from clipboard.", "warn");
    logFrontendEventSafe("warn", "Failed to paste from clipboard", "Clipboard");
}

/** Notify the user of a link open failure. Safe to call outside React components. */
export function notifyLinkOpenFailure(): void {
    useNotificationStore.getState().addNotification("Failed to open link", "warn");
    logFrontendEventSafe("warn", "Failed to open link", "LinkOpen");
}

/**
 * Module-level cooldown to prevent notification spam when multiple highlight calls fail
 * in succession. Uses module scope (not React state) because notify functions are called
 * from async contexts outside React components. A single global cooldown is sufficient
 * because highlight failures are typically correlated (e.g., a broken Shiki instance
 * affects all files simultaneously).
 */
const HIGHLIGHT_FAILURE_COOLDOWN_MS = 10_000;
let lastHighlightFailureAt = 0;

/** Notify the user that syntax highlighting fell back to plain text. Safe to call outside React components. */
export function notifyHighlightFailure(): void {
    const now = Date.now();
    if (now - lastHighlightFailureAt < HIGHLIGHT_FAILURE_COOLDOWN_MS) return;
    lastHighlightFailureAt = now;
    useNotificationStore.getState().addNotification("Syntax highlighting failed. Showing plain text.", "warn");
    logFrontendEventSafe("warn", "Syntax highlighting failed", "Highlight");
}

/**
 * Notify the user that an operation failed.
 * Displays a Toast with the operation name and optional error details.
 * Safe to call outside React components.
 *
 * @param operation - Human-readable operation name (e.g., "Split pane").
 * @param level - Notification severity. Defaults to "warn".
 * @param err - The caught error value. When provided, its message is appended.
 */
export function notifyOperationFailure(
    operation: string,
    level: Notification["level"] = "warn",
    err?: unknown,
): void {
    const msg = err != null
        ? toErrorMessage(err, `${operation} failed`)
        : `${operation} failed`;
    useNotificationStore.getState().addNotification(msg, level);
}

/**
 * Notify the user via Toast AND record the error to the right-sidebar error log panel.
 * Use this for important user-initiated operations whose failures should be
 * discoverable even after the Toast auto-dismisses.
 * Safe to call outside React components.
 *
 * @param operation - Human-readable operation name.
 * @param level - Notification severity. Defaults to "warn".
 * @param err - The caught error value.
 * @param source - Source identifier for logFrontendEventSafe (e.g., "SessionView").
 */
export function notifyAndLog(
    operation: string,
    level: Notification["level"] = "warn",
    err: unknown,
    source: string,
): void {
    notifyOperationFailure(operation, level, err);
    logFrontendEventSafe(
        level,
        toErrorMessage(err, `${operation} failed`),
        source,
    );
}
