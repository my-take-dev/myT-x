import {useEffect, useRef} from "react";
import {api} from "../../api";
import {useTmuxStore} from "../../stores/tmuxStore";
import type {ParsedConfigUpdatedEvent} from "../../types/tmux";
import {logFrontendEventSafe} from "../../utils/logFrontendEventSafe";
import {asObject} from "../../utils/typeGuards";
import {parseConfigUpdatedPayload} from "./configUpdatedEvent";
import {cleanupEventListeners, createEventSubscriber, notifyWarn, tr} from "./eventHelpers";

// Payload types are compile-time documentation only.
interface ConfigEventMap {
    "config:load-failed": {message: string};
    "config:updated": ParsedConfigUpdatedEvent;
}

/**
 * Subscribes to config load/update events and fetches the initial config.
 *
 * Uses a monotonic version counter to ensure event ordering:
 * - Backend version > 0: use as-is for dedup
 * - Backend version missing/null: synthetic monotonic counter
 */
export function useConfigSync(): void {
    const setConfig = useTmuxStore((s) => s.setConfig);
    const configEventVersionRef = useRef(0);
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;
        const cleanupFns: Array<() => void> = [];
        const onEvent = createEventSubscriber<ConfigEventMap>(cleanupFns);

        // I-06: BackendEventMap declares message as required `string`, so the
        // handler receives `{ message: string }`. Runtime validation via asObject
        // still applies; after the guard, `event.message` is guaranteed to be a
        // string — no optional chaining needed.
        onEvent("config:load-failed", (payload) => {
            const event = asObject<{message: string}>(payload);
            if (!event || typeof event.message !== "string" || event.message.trim() === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] config:load-failed: invalid payload", payload);
                }
                return;
            }
            if (import.meta.env.DEV) {
                console.warn("[SYNC] config:load-failed:", event.message);
            }
            notifyWarn(event.message);
        });

        // Initial config fetch.
        // I-26: Race condition edge case — if a "config:updated" event arrives
        // before this API response AND that event's parseConfigUpdatedPayload fails
        // (returning null), configEventVersionRef stays at 0 and the API response
        // is accepted normally. However, if the event parse succeeds but contains
        // an invalid config that increments the version, the API response will be
        // discarded (version > 0 guard below) and config stays as whatever the
        // event handler set. Recovery: the next valid "config:updated" event will
        // update config. This is acceptable because parse failure means the backend
        // sent malformed data, which will be corrected on the next config save/reload.
        void api.GetConfigAndFlushWarnings()
            .then((config) => {
                if (!isMountedRef.current) return;
                // Guard: event-driven config update may have arrived first.
                if (configEventVersionRef.current === 0) {
                    setConfig(config);
                }
            })
            .catch((err: unknown) => {
                if (!isMountedRef.current) return;
                // I-24: config stays null here intentionally. setConfig is NOT called
                // because there is no safe default config (shell path, prefix key, etc.
                // are environment-dependent). Recovery path:
                //   1. The user is notified via notifyWarn to restart the app.
                //   2. If the backend recovers, a "config:updated" event will arrive
                //      and setConfig will be called from that handler.
                if (import.meta.env.DEV) {
                    console.error("[SYNC] GetConfig failed:", err);
                }
                notifyWarn(
                    tr(
                        "sync.notifications.configLoadFailed",
                        "設定の読み込みに失敗しました。アプリを再起動してください。",
                        "Failed to load settings. Please restart the app.",
                    ),
                );
                logFrontendEventSafe("error", "GetConfigAndFlushWarnings failed on startup", "frontend/api");
            });

        onEvent("config:updated", (payload) => {
            const event = parseConfigUpdatedPayload(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] config:updated: invalid payload", payload);
                }
                return;
            }
            if (event.version === null) {
                if (import.meta.env.DEV) {
                    console.warn(
                        "[SYNC] config:updated: version is null, applying payload with synthetic monotonic version",
                        event.updated_at_unix_milli,
                        payload,
                    );
                }
                // Preserve event ordering even when backend payload omits version.
                configEventVersionRef.current += 1;
                setConfig(event.config);
                return;
            }

            if (event.version <= configEventVersionRef.current) {
                if (import.meta.env.DEV) {
                    console.warn(
                        "[SYNC] config:updated: stale payload ignored",
                        event.version,
                        configEventVersionRef.current,
                        event.updated_at_unix_milli,
                    );
                }
                return;
            }

            configEventVersionRef.current = event.version;
            setConfig(event.config);
        });

        return () => {
            isMountedRef.current = false;
            // Reset for StrictMode re-mount — ensures the initial API fetch
            // guard (configEventVersionRef.current === 0) works correctly.
            configEventVersionRef.current = 0;
            cleanupEventListeners(cleanupFns);
        };
        // Zustand store actions are stable references.
    }, [setConfig]);
}
