import {useEffect, useRef} from "react";
import {api} from "../../api";
import {connect as connectPaneStream, disconnect as disconnectPaneStream} from "../../services/paneDataStream";
import {useMCPStore} from "../../stores/mcpStore";
import {useNotificationStore} from "../../stores/notificationStore";
import {useCanvasStore} from "../../stores/canvasStore";
import {useTmuxStore} from "../../stores/tmuxStore";
import type {SessionSnapshot, SessionSnapshotDelta} from "../../types/tmux";
import {logFrontendEventSafe} from "../../utils/logFrontendEventSafe";
import {asArray, asObject} from "../../utils/typeGuards";
import {cleanupEventListeners, createEventSubscriber, notifyWarn, tr} from "./eventHelpers";

// Payload types are compile-time documentation only.
// EventsOn delivers `unknown` at runtime — every handler MUST still
// validate with asObject/asArray before accessing properties.
interface SnapshotEventMap {
    "tmux:snapshot": SessionSnapshot[];
    "tmux:snapshot-delta": Partial<SessionSnapshotDelta>;
    "tmux:active-session": {name?: string};
    "tmux:session-detached": {name?: string};
    "tmux:session-destroyed": {name?: string};
    "tmux:session-emptied": {name?: string};
    "tmux:session-renamed": {oldName?: string; newName?: string};
    "tmux:shim-installed": {installed_path?: string};
    "worktree:setup-complete": {sessionName?: string; success?: boolean; error?: string};
    "worktree:cleanup-failed": {sessionName?: string; path?: string; error?: string};
    "worktree:copy-files-failed": {sessionName?: string; files?: string[]};
    "worktree:copy-dirs-failed": {sessionName?: string; dirs?: string[]};
    "worktree:pull-failed": {sessionName?: string; message?: string; error?: string};
    "tmux:worker-panic": {worker?: string};
    "tmux:worker-fatal": {worker?: string; maxRetries?: number};
}

/**
 * Subscribes to session/snapshot lifecycle events, initializes the pane data
 * stream (WebSocket), and handles worktree and worker notification events.
 *
 * Initial data: ListSessions + GetActiveSession (Promise.allSettled).
 */
export function useSnapshotSync(): void {
    const setSessions = useTmuxStore((s) => s.setSessions);
    const applySessionDelta = useTmuxStore((s) => s.applySessionDelta);
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;
        const cleanupFns: Array<() => void> = [];
        const onEvent = createEventSubscriber<SnapshotEventMap>(cleanupFns);

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
                logFrontendEventSafe("warn", "GetWebSocketURL failed — pane data stream not started", "frontend/ws");
            }
        };
        void initPaneDataStream();

        // L-04: Parallelize initial session API calls to reduce startup latency.
        // I-23: fetch ListSessions and GetActiveSession concurrently and resolve
        // activeSession in a single, deterministic write.
        // Priority: GetActiveSession result > ListSessions[0] fallback > null.
        // Promise.allSettled is used so that one failure does not suppress the other.
        void Promise.allSettled([
            api.ListSessions(),
            api.GetActiveSession(),
        ]).then(([sessionsResult, activeResult]) => {
            if (!isMountedRef.current) return;

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
                notifyWarn(
                    tr(
                        "sync.notifications.activeSessionLoadFailed",
                        "アクティブセッションの取得に失敗しました。",
                        "Failed to fetch the active session.",
                    ),
                );
                logFrontendEventSafe("warn", "GetActiveSession failed on startup", "frontend/api");
                // Fall back to the first available session from ListSessions.
                const fallback = normalizedSessions[0]?.name ?? null;
                setActiveSession(fallback);
            }
        });

        // --- Snapshot events ---

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
            const removed = (asArray<unknown>(delta.removed) ?? []).filter((name): name is string => typeof name === "string");
            if (upserts.length === 0 && removed.length === 0) {
                return;
            }
            applySessionDelta(upserts, removed);
            // 削除されたセッションのキャンバスデータをクリーンアップ
            // NOTE: mcpStore/canvasStore は低頻度クリーンアップのため getState() で直接呼び出す。
            // tmuxStore アクションは高頻度かつ主データフローのため deps 経由で参照する。
            for (const name of removed) {
                if (name !== "") {
                    useCanvasStore.getState().clearSessionData(name);
                }
            }
        });

        onEvent("tmux:active-session", (payload) => {
            const event = asObject<{name?: unknown}>(payload);
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
            const event = asObject<{name?: unknown}>(payload);
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

        onEvent("tmux:session-destroyed", (payload) => {
            const event = asObject<{name?: unknown}>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-destroyed: invalid payload", payload);
                }
                return;
            }
            const name = typeof event.name === "string" ? event.name.trim() : "";
            if (name === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-destroyed: empty session name", payload);
                }
                return;
            }
            useMCPStore.getState().clearSession(name);
            useCanvasStore.getState().clearSessionData(name);
        });

        onEvent("tmux:session-emptied", (payload) => {
            const event = asObject<{name?: unknown}>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-emptied: invalid payload", payload);
                }
                return;
            }
            const name = typeof event.name === "string" ? event.name.trim() : "";
            if (name === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-emptied: empty session name", payload);
                }
                return;
            }
            // NOTE: Unlike session-destroyed, clearSession() is intentionally NOT called here.
            // The session still exists (empty but alive), so MCP state should be preserved.
            useCanvasStore.getState().clearSessionData(name);
        });

        onEvent("tmux:session-renamed", (payload) => {
            const event = asObject<{oldName?: unknown; newName?: unknown}>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-renamed: invalid payload", payload);
                }
                return;
            }
            const oldName = typeof event.oldName === "string" ? event.oldName.trim() : "";
            if (oldName === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] tmux:session-renamed: empty oldName", payload);
                }
                return;
            }
            useMCPStore.getState().clearSession(oldName);
            useCanvasStore.getState().clearSessionData(oldName);
        });

        onEvent("tmux:shim-installed", (payload) => {
            const event = asObject<{installed_path?: unknown}>(payload);
            const installedPath =
                event && typeof event.installed_path === "string" ? event.installed_path.trim() : "";
            if (installedPath !== "") {
                useNotificationStore.getState().addNotification(`tmux shim installed: ${installedPath}`, "info");
                return;
            }
            useNotificationStore.getState().addNotification("tmux shim was installed.", "info");
        });

        // --- Worktree notification events ---

        onEvent("worktree:setup-complete", (payload) => {
            const event = asObject<{sessionName?: unknown; success?: unknown; error?: unknown}>(payload);
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
            const event = asObject<{sessionName?: unknown; path?: unknown; error?: unknown}>(payload);
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
                tr(
                    "sync.notifications.worktreeCleanupFailed",
                    `Worktreeのクリーンアップに失敗しました: ${sessionName ? ` (${sessionName})` : ""}: ${error || "不明なエラー"}`,
                    `Failed to clean up worktree${sessionName ? ` (${sessionName})` : ""}: ${error || "Unknown error"}`,
                ),
            );
        });

        onEvent("worktree:copy-files-failed", (payload) => {
            const event = asObject<{sessionName?: unknown; files?: unknown}>(payload);
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
                tr(
                    "sync.notifications.worktreeCopyFilesFailed",
                    `ファイルコピーに失敗しました (${sessionName}): ${(files ?? []).join(", ")}`,
                    `Failed to copy files (${sessionName}): ${(files ?? []).join(", ")}`,
                ),
            );
        });

        onEvent("worktree:copy-dirs-failed", (payload) => {
            const event = asObject<{sessionName?: unknown; dirs?: unknown}>(payload);
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
                tr(
                    "sync.notifications.worktreeCopyDirsFailed",
                    `ディレクトリコピーに失敗しました (${sessionName}): ${(dirs ?? []).join(", ")}`,
                    `Failed to copy directories (${sessionName}): ${(dirs ?? []).join(", ")}`,
                ),
            );
        });

        onEvent("worktree:pull-failed", (payload) => {
            const event = asObject<{sessionName?: unknown; message?: unknown; error?: unknown}>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[worktree] pull-failed: invalid payload", payload);
                }
                return;
            }
            const sessionName = typeof event.sessionName === "string" ? event.sessionName.trim() : "";
            const message = typeof event.message === "string" ? event.message.trim() : "";
            const error = typeof event.error === "string" ? event.error.trim() : "";

            if (import.meta.env.DEV) {
                console.warn("[worktree] pull-failed:", sessionName, message, error);
            }

            const sessionLabel = sessionName !== "" ? ` (${sessionName})` : "";
            const detailSuffix = error !== "" ? ` Error: ${error}` : "";
            notifyWarn(
                `Git pull failed while creating the worktree${sessionLabel}. Continuing with the local checkout state.${detailSuffix}`,
            );
        });

        // --- Worker lifecycle events ---

        onEvent("tmux:worker-panic", (payload) => {
            const event = asObject<{worker?: unknown}>(payload);
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
            const event = asObject<{worker?: unknown; maxRetries?: unknown}>(payload);
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

        return () => {
            isMountedRef.current = false;
            disconnectPaneStream(); // WebSocket切断 + タイマークリア (#96)
            cleanupEventListeners(cleanupFns);
        };
        // Zustand store actions are stable references.
    }, [applySessionDelta, setActiveSession, setSessions]);
}
