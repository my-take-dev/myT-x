import {useNotificationStore} from "../stores/notificationStore";

/** Notify the user of a clipboard write failure. Safe to call outside React components. */
export function notifyClipboardFailure(): void {
    useNotificationStore.getState().addNotification("Failed to copy to clipboard.", "warn");
}

/** Notify the user of a link open failure. Safe to call outside React components. */
export function notifyLinkOpenFailure(): void {
    useNotificationStore.getState().addNotification("Failed to open link", "warn");
}

/**
 * Module-level cooldown to prevent notification spam when multiple highlight calls fail
 * in succession. Uses module scope (not React state) because notify functions are called
 * from async contexts outside React components. A single global cooldown is sufficient
 * because highlight failures are typically correlated (e.g., a broken Shiki instance
 * affects all files simultaneously).
 */
const HIGHLIGHT_FAILURE_COOLDOWN_MS = 10_000;
// NOTE: Module-level state resets to 0 on Vite HMR reload, so the cooldown
// resets during development. This is acceptable — HMR-induced notification spam
// only affects dev and self-corrects on page reload. (checklist #96 HMR safety)
let lastHighlightFailureAt = 0;

/** Notify the user that syntax highlighting fell back to plain text. Safe to call outside React components. */
export function notifyHighlightFailure(): void {
    const now = Date.now();
    if (now - lastHighlightFailureAt < HIGHLIGHT_FAILURE_COOLDOWN_MS) return;
    lastHighlightFailureAt = now;
    useNotificationStore.getState().addNotification("Syntax highlighting failed. Showing plain text.", "warn");
}
