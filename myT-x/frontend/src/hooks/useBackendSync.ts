import {useEffect, useRef} from "react";
import {EventsOn} from "../../wailsjs/runtime/runtime";
import {api} from "../api";
import {connect as connectPaneStream, disconnect as disconnectPaneStream} from "../services/paneDataStream";
import {useErrorLogStore} from "../stores/errorLogStore";
import {useInputHistoryStore} from "../stores/inputHistoryStore";
import {useNotificationStore} from "../stores/notificationStore";
import {useTmuxStore} from "../stores/tmuxStore";
import type {AppConfig, ParsedConfigUpdatedEvent, SessionSnapshot, SessionSnapshotDelta} from "../types/tmux";
import {asArray, asObject} from "../utils/typeGuards";
import {logFrontendEventSafe} from "../utils/logFrontendEventSafe";

// SUG-20: Module-level constant — shared across renders, avoids re-creation inside useEffect.
const ERROR_LOG_DEBOUNCE_MS = 80;
const ERROR_LOG_FETCH_RETRY_DELAY_MS = 250;
const INPUT_HISTORY_DEBOUNCE_MS = 80;
const INPUT_HISTORY_FETCH_RETRY_DELAY_MS = 250;

// ---------------------------------------------------------------------------
// Backend event payload type map (I-37: eliminates any-typed handler)
// Keys are Wails event names; values are the expected payload shapes.
//
// S-19: Payload types are narrowed from `unknown` to concrete shapes where
// the backend contract is known. Handlers still validate at runtime via
// asObject/asArray guards, so these types serve as documentation and IDE
// assistance rather than runtime guarantees.
//
// IMPORTANT: These types are compile-time documentation only.
// EventsOn delivers `unknown` at runtime — every handler MUST still
// validate with asObject/asArray before accessing properties.
// ---------------------------------------------------------------------------
interface BackendEventMap {
    "config:load-failed": { message: string };
    "tmux:snapshot": SessionSnapshot[];
    "tmux:snapshot-delta": Partial<SessionSnapshotDelta>;
    "tmux:active-session": { name?: string };
    "tmux:session-detached": { name?: string };
    "tmux:shim-installed": { installed_path?: string };
    "config:updated": ParsedConfigUpdatedEvent;
    "worktree:setup-complete": { sessionName?: string; success?: boolean; error?: string };
    "worktree:cleanup-failed": { sessionName?: string; path?: string; error?: string };
    "worktree:copy-files-failed": { sessionName?: string; files?: string[] };
    "worktree:copy-dirs-failed": { sessionName?: string; dirs?: string[] };
    "tmux:worker-panic": { worker?: string };
    "tmux:worker-fatal": { worker?: string; maxRetries?: number };
    // NOTE(A-0): Ping-only event — no payload. Frontend fetches full snapshot
    // via GetSessionErrorLog() on receipt. This eliminates data loss from throttling.
    "app:session-log-updated": null;
    // Ping-only event for input history — same pattern as session-log-updated.
    "app:input-history-updated": null;
}

function isAppConfigPayload(payload: unknown): payload is AppConfig {
    const cfg = asObject<Record<string, unknown>>(payload);
    if (!cfg) {
        return false;
    }
    if (typeof cfg.shell !== "string" || cfg.shell.trim() === "") {
        return false;
    }
    if (typeof cfg.prefix !== "string" || cfg.prefix.trim() === "") {
        return false;
    }
    if (typeof cfg.quake_mode !== "boolean") {
        return false;
    }
    if (typeof cfg.global_hotkey !== "string") {
        return false;
    }

    const keys = asObject<Record<string, unknown>>(cfg.keys);
    if (!keys) {
        return false;
    }
    for (const value of Object.values(keys)) {
        if (typeof value !== "string") {
            return false;
        }
    }

    const worktree = asObject<Record<string, unknown>>(cfg.worktree);
    if (!worktree) {
        return false;
    }
    if (typeof worktree.enabled !== "boolean" || typeof worktree.force_cleanup !== "boolean") {
        return false;
    }
    return true;
}

/**
 * S-08: Parse config:updated event payload with synthetic version compatibility.
 *
 * The backend may omit the `version` field (e.g., older backend versions or
 * manual config file edits that trigger a filesystem watcher event). When
 * version is missing/invalid, this function returns `version: null` and the
 * caller in the event handler uses a synthetic monotonic counter
 * (`configEventVersionRef.current += 1`) to preserve event ordering.
 *
 * This ensures forward compatibility: new frontends can handle old backends
 * that do not emit version numbers.
 */
function parseConfigUpdatedPayload(payload: unknown): ParsedConfigUpdatedEvent | null {
    const event = asObject<Record<string, unknown>>(payload);
    if (!event) {
        return null;
    }

    const nestedConfig = asObject<Record<string, unknown>>(event.config);
    const rawVersion = event.version;
    let version: number | null = null;
    if (typeof rawVersion === "number" && Number.isSafeInteger(rawVersion) && rawVersion > 0) {
        version = rawVersion;
    }
    const rawUpdatedAt = event.updated_at_unix_milli;
    let updatedAtUnixMilli: number | null = null;
    if (typeof rawUpdatedAt === "number" && Number.isSafeInteger(rawUpdatedAt) && rawUpdatedAt > 0) {
        updatedAtUnixMilli = rawUpdatedAt;
    }

    if (nestedConfig) {
        if (!isAppConfigPayload(nestedConfig)) {
            return null;
        }
        return {config: nestedConfig, version, updated_at_unix_milli: updatedAtUnixMilli};
    }

    if (!isAppConfigPayload(event)) {
        return null;
    }
    return {config: event, version, updated_at_unix_milli: updatedAtUnixMilli};
}

export function useBackendSync() {
    const setConfig = useTmuxStore((s) => s.setConfig);
    const setSessions = useTmuxStore((s) => s.setSessions);
    const applySessionDelta = useTmuxStore((s) => s.applySessionDelta);
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const configEventVersionRef = useRef(0);
    // Guard against React state updates after the component unmounts.
    // Promise.allSettled callbacks are async and may resolve after cleanup.
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;

        // WebSocket connection for high-throughput pane data streaming.
        // Low-frequency events (snapshots, config) continue using Wails IPC.
        const initPaneDataStream = async () => {
            try {
                const url = await api.GetWebSocketURL();
                if (url && isMountedRef.current) {
                    connectPaneStream(url);
                }
            } catch (err) {
                // NOTE: If WebSocket URL retrieval fails, pane data will not be
                // streamed. The reconnection logic in paneDataStream handles
                // recovery once the backend becomes available.
                if (import.meta.env.DEV) {
                    console.warn("[WS] GetWebSocketURL failed", err);
                }
            }
        };
        void initPaneDataStream();

        const addNotification = useNotificationStore.getState().addNotification;
        const notifyWarn = (message: string) => addNotification(message, "warn");
        const cleanupFns: Array<() => void> = [];

        // I-37: typed event registration — handler receives a single unknown payload
        // matching the BackendEventMap entry for the given event name.
        const onEvent = <K extends keyof BackendEventMap>(
            eventName: K,
            handler: (payload: BackendEventMap[K]) => void,
        ) => {
            cleanupFns.push(EventsOn(eventName, handler));
        };

        // I-06: BackendEventMap declares message as required `string`, so the
        // handler receives `{ message: string }`. Runtime validation via asObject
        // still applies; after the guard, `event.message` is guaranteed to be a
        // string — no optional chaining needed.
        onEvent("config:load-failed", (payload) => {
            const event = asObject<{ message: string }>(payload);
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

        // L-04: Parallelize all three initial API calls to reduce startup latency.
        // GetConfigAndFlushWarnings is independent of session data.
        // I-23: fetch ListSessions and GetActiveSession concurrently and resolve
        // activeSession in a single, deterministic write.
        // Priority: GetActiveSession result > ListSessions[0] fallback > null.
        // Promise.allSettled is used so that one failure does not suppress the other.
        void Promise.allSettled([
            api.ListSessions(),
            api.GetActiveSession(),
            api.GetConfigAndFlushWarnings(),
        ]).then(([sessionsResult, activeResult, configResult]) => {
            // Bail out if the component unmounted while awaiting the API calls.
            if (!isMountedRef.current) return;

            // --- config ---
            // I-26: Race condition edge case — if a "config:updated" event arrives
            // before this API response AND that event's parseConfigUpdatedPayload fails
            // (returning null), configEventVersionRef stays at 0 and the API response
            // is accepted normally. However, if the event parse succeeds but contains
            // an invalid config that increments the version, the API response will be
            // discarded (version > 0 guard below) and config stays as whatever the
            // event handler set. Recovery: the next valid "config:updated" event will
            // update config. This is acceptable because parse failure means the backend
            // sent malformed data, which will be corrected on the next config save/reload.
            if (configResult.status === "fulfilled") {
                // Guard: event-driven config update may have arrived first.
                if (configEventVersionRef.current === 0) {
                    setConfig(configResult.value);
                }
            } else {
                // I-24: config stays null here intentionally. setConfig is NOT called
                // because there is no safe default config (shell path, prefix key, etc.
                // are environment-dependent). Recovery path:
                //   1. The user is notified via notifyWarn to restart the app.
                //   2. If the backend recovers, a "config:updated" event will arrive
                //      and setConfig will be called from that handler.
                if (import.meta.env.DEV) {
                    console.error("[SYNC] GetConfig failed:", configResult.reason);
                }
                notifyWarn("設定の読み込みに失敗しました。アプリを再起動してください。");
                logFrontendEventSafe("error", "GetConfigAndFlushWarnings failed on startup", "frontend/api");
            }

            // --- session list ---
            let normalizedSessions: SessionSnapshot[] = [];
            if (sessionsResult.status === "fulfilled") {
                normalizedSessions = sessionsResult.value ?? [];
                setSessions(normalizedSessions);
            } else {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] ListSessions failed:", sessionsResult.reason);
                }
                notifyWarn("Failed to load session list. Please restart the app and try again.");
                logFrontendEventSafe("error", "ListSessions failed on startup", "frontend/api");
            }

            // --- active session (GetActiveSession takes priority) ---
            if (activeResult.status === "fulfilled") {
                const raw = activeResult.value;
                const normalized = typeof raw === "string" ? raw.trim() : "";
                setActiveSession(normalized !== "" ? normalized : null);
            } else {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] GetActiveSession failed:", activeResult.reason);
                }
                notifyWarn("アクティブセッションの取得に失敗しました。");
                logFrontendEventSafe("warn", "GetActiveSession failed on startup", "frontend/api");
                // Fall back to the first available session from ListSessions.
                const fallback = normalizedSessions[0]?.name ?? null;
                setActiveSession(fallback);
            }
        });

        onEvent("tmux:snapshot", (payload) => {
            const snapshots = asArray<ReturnType<typeof useTmuxStore.getState>["sessions"][number]>(payload);
            if (!snapshots) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:snapshot: payload is not an array", payload);
                }
                return;
            }
            setSessions(snapshots);
        });

        onEvent("tmux:snapshot-delta", (payload) => {
            const delta = asObject<Partial<SessionSnapshotDelta>>(payload);
            if (!delta) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:snapshot-delta: payload is null/invalid", payload);
                }
                return;
            }
            // I-04: Default to empty arrays so that "upsert-only" or "remove-only"
            // deltas are not silently discarded. Only skip when both are empty.
            const upserts = asArray<SessionSnapshotDelta["upserts"][number]>(delta.upserts) ?? [];
            const removed = asArray<string>(delta.removed) ?? [];
            if (upserts.length === 0 && removed.length === 0) {
                return;
            }
            applySessionDelta(upserts, removed);
        });

        onEvent("tmux:active-session", (payload) => {
            const event = asObject<{ name?: unknown }>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:active-session: invalid payload", payload);
                }
                return;
            }
            const name = typeof event.name === "string" ? event.name.trim() : "";
            setActiveSession(name !== "" ? name : null);
        });

        onEvent("tmux:session-detached", (payload) => {
            const event = asObject<{ name?: unknown }>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-detached: invalid payload", payload);
                }
                return;
            }
            const name = typeof event.name === "string" ? event.name.trim() : "";
            if (name !== "") {
                setActiveSession(name);
            }
        });

        onEvent("tmux:shim-installed", (payload) => {
            const event = asObject<{ installed_path?: unknown }>(payload);
            const installedPath =
                event && typeof event.installed_path === "string" ? event.installed_path.trim() : "";
            if (installedPath !== "") {
                useNotificationStore.getState().addNotification(`tmux shim installed: ${installedPath}`, "info");
                return;
            }
            useNotificationStore.getState().addNotification("tmux shim was installed.", "info");
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

        onEvent("worktree:setup-complete", (payload) => {
            const event = asObject<{ sessionName?: unknown; success?: unknown; error?: unknown }>(payload);
            if (!event || typeof event.sessionName !== "string" || event.sessionName.trim() === "") {
                if (import.meta.env.DEV) {
                    console.warn("[worktree] setup-complete: invalid payload", payload);
                }
                return;
            }
            const success = typeof event.success === "boolean" ? event.success : undefined;
            const error = typeof event.error === "string" ? event.error : undefined;

            // [DEBUG:worktree] setup-complete event
            if (import.meta.env.DEV) {
                console.log("[worktree] setup-complete:", event.sessionName, success, error);
            }
            if (success === false) {
                notifyWarn(`Worktree setup failed (${event.sessionName}): ${error || "Unknown error"}`);
            }
        });

        onEvent("worktree:cleanup-failed", (payload) => {
            const event = asObject<{ sessionName?: unknown; path?: unknown; error?: unknown }>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[worktree] cleanup-failed: invalid payload", payload);
                }
                return;
            }
            const sessionName = typeof event.sessionName === "string" ? event.sessionName : "";
            const path = typeof event.path === "string" ? event.path : "";
            const error = typeof event.error === "string" ? event.error : "";

            if (import.meta.env.DEV) {
                console.warn("[worktree] cleanup-failed:", sessionName, path, error);
            }
            notifyWarn(
                `Worktreeのクリーンアップに失敗しました: ${sessionName ? ` (${sessionName})` : ""}: ${error || "不明なエラー"}`,
            );
        });

        onEvent("worktree:copy-files-failed", (payload) => {
            const event = asObject<{ sessionName?: unknown; files?: unknown }>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[worktree] copy-files-failed: invalid payload", payload);
                }
                return;
            }
            const sessionName = typeof event.sessionName === "string" ? event.sessionName : "";
            const files = asArray<string>(event.files);

            if (import.meta.env.DEV) {
                console.warn("[worktree] copy-files-failed:", sessionName, files);
            }
            notifyWarn(
                `ファイルコピーに失敗しました (${sessionName}): ${(files ?? []).join(", ")}`,
            );
        });

        onEvent("worktree:copy-dirs-failed", (payload) => {
            const event = asObject<{ sessionName?: unknown; dirs?: unknown }>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[worktree] copy-dirs-failed: invalid payload", payload);
                }
                return;
            }
            const sessionName = typeof event.sessionName === "string" ? event.sessionName : "";
            const dirs = asArray<string>(event.dirs);

            if (import.meta.env.DEV) {
                console.warn("[worktree] copy-dirs-failed:", sessionName, dirs);
            }
            notifyWarn(
                `ディレクトリコピーに失敗しました (${sessionName}): ${(dirs ?? []).join(", ")}`,
            );
        });

        onEvent("tmux:worker-panic", (payload) => {
            const event = asObject<{ worker?: unknown }>(payload);
            if (!event || typeof event.worker !== "string" || event.worker.trim() === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:worker-panic: invalid payload", payload);
                }
                notifyWarn("A background worker panic was recovered.");
                // NOTE: Also persist to session log. The backend emits this event after
                // recovering from a panic; the Go-side slog record may not always be
                // present (e.g. if the panic occurred before the log write).
                logFrontendEventSafe("warn", "A background worker panic was recovered", "frontend/worker");
                return;
            }
            const workerName = event.worker.trim();
            if (import.meta.env.DEV) {
                console.warn("[SYNC] tmux:worker-panic:", workerName);
            }
            notifyWarn(`A background worker panic was recovered (${workerName}).`);
            logFrontendEventSafe("warn", `A background worker panic was recovered (${workerName})`, "frontend/worker");
        });

        onEvent("tmux:worker-fatal", (payload) => {
            const event = asObject<{ worker?: unknown; maxRetries?: unknown }>(payload);
            const workerName = event && typeof event.worker === "string" ? event.worker.trim() : "";
            const maxRetries = event && typeof event.maxRetries === "number" ? event.maxRetries : undefined;
            if (workerName === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:worker-fatal: invalid payload", payload);
                }
                notifyWarn("A background worker has permanently stopped after exceeding max retries.");
                // NOTE: Also persist to session log — fatal worker stops are high-severity events.
                logFrontendEventSafe("error", "A background worker has permanently stopped after exceeding max retries", "frontend/worker");
                return;
            }
            if (import.meta.env.DEV) {
                console.warn("[SYNC] tmux:worker-fatal:", workerName, "maxRetries:", maxRetries);
            }
            const fatalMsg =
                `Background worker "${workerName}" has permanently stopped` +
                (maxRetries != null ? ` after ${maxRetries} retries` : "");
            notifyWarn(fatalMsg + ".");
            logFrontendEventSafe("error", fatalMsg, "frontend/worker");
        });

        // NOTE(A-0): "ping + fetch" model — the backend sends a lightweight ping
        // (no payload) when new log entries are available. The frontend fetches
        // the full snapshot via GetSessionErrorLog(). Debounce prevents flooding
        // when multiple log entries arrive in quick succession.
        let errorLogDebounceTimer: ReturnType<typeof setTimeout> | null = null;
        let errorLogRetryTimer: ReturnType<typeof setTimeout> | null = null;
        // IMP-13: Monotonic fetch counter — prevents stale (older) responses from
        // overwriting newer data when multiple requests are in flight concurrently.
        let errorLogFetchSeq = 0;

        const fetchErrorLog = (attempt = 0) => {
            // IMP-13: Skip fetch if component is already unmounted (#96 timer guard).
            if (!isMountedRef.current) return;
            const seq = ++errorLogFetchSeq;
            void api.GetSessionErrorLog()
                .then((result) => {
                    // IMP-13: Discard result if a newer request was issued or component unmounted.
                    if (!isMountedRef.current || seq !== errorLogFetchSeq) return;
                    if (errorLogRetryTimer != null) {
                        clearTimeout(errorLogRetryTimer);
                        errorLogRetryTimer = null;
                    }
                    useErrorLogStore.getState().setEntries(result ?? []);
                })
                .catch((err: unknown) => {
                    if (!isMountedRef.current || seq !== errorLogFetchSeq) return;
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] GetSessionErrorLog failed:", err, "attempt:", attempt + 1);
                    }
                    if (attempt >= 1) {
                        return;
                    }
                    if (errorLogRetryTimer != null) {
                        clearTimeout(errorLogRetryTimer);
                    }
                    errorLogRetryTimer = setTimeout(() => {
                        errorLogRetryTimer = null;
                        fetchErrorLog(attempt + 1);
                    }, ERROR_LOG_FETCH_RETRY_DELAY_MS);
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] scheduled GetSessionErrorLog retry", "delayMs:", ERROR_LOG_FETCH_RETRY_DELAY_MS);
                    }
                });
        };

        onEvent("app:session-log-updated", () => {
            // IMP-13: Guard against debounce timer firing after unmount.
            if (!isMountedRef.current) return;
            if (errorLogDebounceTimer != null) {
                clearTimeout(errorLogDebounceTimer);
            }
            errorLogDebounceTimer = setTimeout(fetchErrorLog, ERROR_LOG_DEBOUNCE_MS);
        });

        // IMP-02 + IMP-15: Load initial error log entries after subscribing to the
        // ping event. Reuses fetchErrorLog to eliminate duplicated Promise chain.
        // By subscribing first, then loading the snapshot, we ensure no entries
        // are missed: pings arriving during the fetch trigger setEntries which
        // always replaces with the latest full snapshot. The fetch counter (IMP-13)
        // ensures only the latest response is applied.
        fetchErrorLog();

        // --- Input History: same "ping + fetch" pattern as error log ---
        let inputHistoryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
        let inputHistoryRetryTimer: ReturnType<typeof setTimeout> | null = null;
        let inputHistoryFetchSeq = 0;

        const fetchInputHistory = (attempt = 0) => {
            if (!isMountedRef.current) return;
            const seq = ++inputHistoryFetchSeq;
            void api.GetInputHistory()
                .then((result) => {
                    if (!isMountedRef.current || seq !== inputHistoryFetchSeq) return;
                    if (inputHistoryRetryTimer != null) {
                        clearTimeout(inputHistoryRetryTimer);
                        inputHistoryRetryTimer = null;
                    }
                    useInputHistoryStore.getState().setEntries(result ?? []);
                })
                .catch((err: unknown) => {
                    if (!isMountedRef.current || seq !== inputHistoryFetchSeq) return;
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] GetInputHistory failed:", err, "attempt:", attempt + 1);
                    }
                    if (attempt >= 1) {
                        return;
                    }
                    if (inputHistoryRetryTimer != null) {
                        clearTimeout(inputHistoryRetryTimer);
                    }
                    inputHistoryRetryTimer = setTimeout(() => {
                        inputHistoryRetryTimer = null;
                        fetchInputHistory(attempt + 1);
                    }, INPUT_HISTORY_FETCH_RETRY_DELAY_MS);
                });
        };

        onEvent("app:input-history-updated", () => {
            if (!isMountedRef.current) return;
            if (inputHistoryDebounceTimer != null) {
                clearTimeout(inputHistoryDebounceTimer);
            }
            inputHistoryDebounceTimer = setTimeout(fetchInputHistory, INPUT_HISTORY_DEBOUNCE_MS);
        });

        fetchInputHistory();

        return () => {
            isMountedRef.current = false;
            disconnectPaneStream(); // WebSocket切断 + タイマークリア (#96)
            configEventVersionRef.current = 0;
            if (errorLogDebounceTimer != null) {
                clearTimeout(errorLogDebounceTimer);
                errorLogDebounceTimer = null;
            }
            if (errorLogRetryTimer != null) {
                clearTimeout(errorLogRetryTimer);
                errorLogRetryTimer = null;
            }
            if (inputHistoryDebounceTimer != null) {
                clearTimeout(inputHistoryDebounceTimer);
                inputHistoryDebounceTimer = null;
            }
            if (inputHistoryRetryTimer != null) {
                clearTimeout(inputHistoryRetryTimer);
                inputHistoryRetryTimer = null;
            }
            for (let i = cleanupFns.length - 1; i >= 0; i -= 1) {
                try {
                    cleanupFns[i]?.();
                } catch (err) {
                    if (import.meta.env.DEV) {
                        console.warn("[SYNC] failed to cleanup event listener", err);
                    }
                }
            }
        };
        // Zustand store actions are stable references.
    }, [applySessionDelta, setActiveSession, setConfig, setSessions]);
}
