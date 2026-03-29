import {EventsOn} from "../../../wailsjs/runtime/runtime";
import {getLanguage, translate} from "../../i18n";
import {useNotificationStore} from "../../stores/notificationStore";

/**
 * Creates a typed event subscriber that registers EventsOn handlers
 * and collects their unsubscribe functions for cleanup.
 *
 * IMPORTANT: EventMap types are compile-time documentation only.
 * EventsOn delivers `unknown` at runtime — every handler MUST still
 * validate with asObject/asArray before accessing properties.
 */
export function createEventSubscriber<EventMap>(
    cleanupFns: Array<() => void>,
) {
    return <K extends keyof EventMap & string>(
        eventName: K,
        handler: (payload: EventMap[K]) => void,
    ) => {
        cleanupFns.push(EventsOn(eventName, handler));
    };
}

/**
 * Unsubscribes all registered event listeners in reverse order.
 * Catches and logs errors during cleanup to prevent one failed
 * unsubscribe from blocking subsequent cleanups.
 */
export function cleanupEventListeners(cleanupFns: Array<() => void>): void {
    for (let i = cleanupFns.length - 1; i >= 0; i -= 1) {
        try {
            cleanupFns[i]?.();
        } catch (err) {
            if (import.meta.env.DEV) {
                console.warn("[SYNC] failed to cleanup event listener", err);
            }
        }
    }
}

/** Shorthand for adding a "warn" notification via the notification store. */
export function notifyWarn(message: string): void {
    useNotificationStore.getState().addNotification(message, "warn");
}

/** Translation helper using current locale. */
export function tr(
    key: string,
    jaText: string,
    enText: string,
    params?: Record<string, string | number>,
): string {
    return translate(key, getLanguage() === "ja" ? jaText : enText, params);
}
