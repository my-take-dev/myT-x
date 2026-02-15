import { useEffect, useRef } from "react";
import { EventsOff, EventsOn } from "../../wailsjs/runtime/runtime";
import { api } from "../api";
import { useNotificationStore } from "../stores/notificationStore";
import { useTmuxStore } from "../stores/tmuxStore";
import type { AppConfig, ParsedConfigUpdatedEvent, SessionSnapshotDelta } from "../types/tmux";
import { asArray, asObject } from "../utils/typeGuards";

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
    return { config: nestedConfig, version, updated_at_unix_milli: updatedAtUnixMilli };
  }

  if (!isAppConfigPayload(event)) {
    return null;
  }
  return { config: event, version, updated_at_unix_milli: updatedAtUnixMilli };
}

export function useBackendSync() {
  const setConfig = useTmuxStore((s) => s.setConfig);
  const setSessions = useTmuxStore((s) => s.setSessions);
  const applySessionDelta = useTmuxStore((s) => s.applySessionDelta);
  const setActiveSession = useTmuxStore((s) => s.setActiveSession);
  const configEventVersionRef = useRef(0);

  useEffect(() => {
    const addNotification = useNotificationStore.getState().addNotification;
    const notifyWarn = (message: string) => addNotification(message, "warn");

    EventsOn("config:load-failed", (payload: unknown) => {
      const event = asObject<{ message?: string }>(payload);
      if (!event?.message) {
        console.warn("[DEBUG-SYNC] config:load-failed: invalid payload", payload);
        return;
      }
      console.warn("[DEBUG-SYNC] config:load-failed:", event.message);
      notifyWarn(event.message);
    });

    api.GetConfigAndFlushWarnings()
      .then((cfg) => {
        if (configEventVersionRef.current > 0) {
          return;
        }
        setConfig(cfg);
      })
      .catch((err) => {
        console.error("[DEBUG-SYNC] GetConfig failed:", err);
        notifyWarn("設定の読み込みに失敗しました。アプリを再起動してください。");
      });
    api.ListSessions()
      .then((sessions) => {
        const normalizedSessions = sessions ?? [];
        setSessions(normalizedSessions);
        if (normalizedSessions[0]) {
          setActiveSession(normalizedSessions[0].name);
        }
      })
      .catch((err) => {
        console.warn("[DEBUG-SYNC] ListSessions failed:", err);
        notifyWarn("Failed to load session list. Please restart the app and try again.");
      });
    api.GetActiveSession()
      .then((activeSessionName) => {
        const normalized = typeof activeSessionName === "string" ? activeSessionName.trim() : "";
        setActiveSession(normalized !== "" ? normalized : null);
      })
      .catch((err) => {
        console.warn("[DEBUG-SYNC] GetActiveSession failed:", err);
      });

    EventsOn("tmux:snapshot", (payload: unknown) => {
      const snapshots = asArray<ReturnType<typeof useTmuxStore.getState>["sessions"][number]>(payload);
      if (!snapshots) {
        console.warn("[DEBUG-SYNC] tmux:snapshot: payload is not an array", payload);
        return;
      }
      setSessions(snapshots);
    });

    EventsOn("tmux:snapshot-delta", (payload: unknown) => {
      const delta = asObject<Partial<SessionSnapshotDelta>>(payload);
      if (!delta) {
        console.warn("[DEBUG-SYNC] tmux:snapshot-delta: payload is null/invalid", payload);
        return;
      }
      const upserts = asArray<SessionSnapshotDelta["upserts"][number]>(delta.upserts);
      const removed = asArray<string>(delta.removed);
      if (!upserts || !removed) {
        console.warn("[DEBUG-SYNC] tmux:snapshot-delta: invalid array fields", payload);
        return;
      }
      applySessionDelta(upserts, removed);
    });

    EventsOn("tmux:active-session", (payload: unknown) => {
      const event = asObject<{ name?: unknown }>(payload);
      if (!event) {
        console.warn("[DEBUG-SYNC] tmux:active-session: invalid payload", payload);
        return;
      }
      const name = typeof event.name === "string" ? event.name.trim() : "";
      setActiveSession(name !== "" ? name : null);
    });

    EventsOn("tmux:session-detached", (payload: unknown) => {
      const event = asObject<{ name?: unknown }>(payload);
      if (!event) {
        console.warn("[DEBUG-SYNC] tmux:session-detached: invalid payload", payload);
        return;
      }
      const name = typeof event.name === "string" ? event.name.trim() : "";
      if (name !== "") {
        setActiveSession(name);
      }
    });

    EventsOn("tmux:shim-installed", (payload: unknown) => {
      const event = asObject<{ installed_path?: unknown }>(payload);
      const installedPath =
        event && typeof event.installed_path === "string" ? event.installed_path.trim() : "";
      if (installedPath !== "") {
        useNotificationStore.getState().addNotification(`tmux shim installed: ${installedPath}`, "info");
        return;
      }
      useNotificationStore.getState().addNotification("tmux shim was installed.", "info");
    });

    EventsOn("config:updated", (payload: unknown) => {
      const event = parseConfigUpdatedPayload(payload);
      if (!event) {
        console.warn("[DEBUG-SYNC] config:updated: invalid payload", payload);
        return;
      }
      if (event.version === null) {
        console.warn(
          "[DEBUG-SYNC] config:updated: version is null, applying payload with synthetic monotonic version",
          event.updated_at_unix_milli,
          payload,
        );
        // Preserve event ordering even when backend payload omits version.
        configEventVersionRef.current += 1;
        setConfig(event.config);
        return;
      }

      if (event.version <= configEventVersionRef.current) {
        console.warn(
          "[DEBUG-SYNC] config:updated: stale payload ignored",
          event.version,
          configEventVersionRef.current,
          event.updated_at_unix_milli,
        );
        return;
      }

      configEventVersionRef.current = event.version;
      setConfig(event.config);
    });

    EventsOn("worktree:setup-complete", (payload: unknown) => {
      const event = asObject<{ sessionName?: unknown; success?: unknown; error?: unknown }>(payload);
      if (!event || typeof event.sessionName !== "string" || event.sessionName.trim() === "") {
        console.warn("[worktree] setup-complete: invalid payload", payload);
        return;
      }
      const success = typeof event.success === "boolean" ? event.success : undefined;
      const error = typeof event.error === "string" ? event.error : undefined;

      // [DEBUG:worktree] setup-complete event
      console.log("[worktree] setup-complete:", event.sessionName, success, error);
      if (success === false) {
        notifyWarn(`Worktree setup failed (${event.sessionName}): ${error || "Unknown error"}`);
      }
    });

    EventsOn("worktree:cleanup-failed", (payload: unknown) => {
      const event = asObject<{ sessionName?: unknown; path?: unknown; error?: unknown }>(payload);
      if (!event) {
        console.warn("[worktree] cleanup-failed: invalid payload", payload);
        return;
      }
      const sessionName = typeof event.sessionName === "string" ? event.sessionName : "";
      const path = typeof event.path === "string" ? event.path : "";
      const error = typeof event.error === "string" ? event.error : "";

      console.warn("[worktree] cleanup-failed:", sessionName, path, error);
      notifyWarn(
        `Worktreeのクリーンアップに失敗しました: ${sessionName ? ` (${sessionName})` : ""}: ${error || "不明なエラー"}`,
      );
    });

    EventsOn("worktree:copy-files-failed", (payload: unknown) => {
      const event = asObject<{ sessionName?: unknown; files?: unknown }>(payload);
      if (!event) {
        console.warn("[worktree] copy-files-failed: invalid payload", payload);
        return;
      }
      const sessionName = typeof event.sessionName === "string" ? event.sessionName : "";
      const files = asArray<string>(event.files);

      console.warn("[worktree] copy-files-failed:", sessionName, files);
      notifyWarn(
        `ファイルコピーに失敗しました (${sessionName}): ${(files ?? []).join(", ")}`,
      );
    });

    EventsOn("tmux:worker-panic", (payload: unknown) => {
      const event = asObject<{ worker?: unknown }>(payload);
      if (!event || typeof event.worker !== "string" || event.worker.trim() === "") {
        console.warn("[DEBUG-SYNC] tmux:worker-panic: invalid payload", payload);
        notifyWarn("A background worker panic was recovered.");
        return;
      }
      const workerName = event.worker.trim();
      console.warn("[DEBUG-SYNC] tmux:worker-panic:", workerName);
      notifyWarn(`A background worker panic was recovered (${workerName}).`);
    });
    return () => {
      configEventVersionRef.current = 0;
      EventsOff("tmux:snapshot");
      EventsOff("tmux:snapshot-delta");
      EventsOff("tmux:active-session");
      EventsOff("tmux:session-detached");
      EventsOff("tmux:shim-installed");
      EventsOff("worktree:setup-complete");
      EventsOff("config:updated");
      EventsOff("config:load-failed");
      EventsOff("worktree:cleanup-failed");
      EventsOff("worktree:copy-files-failed");
      EventsOff("tmux:worker-panic");
    };
  // Zustand store actions are stable references.
  }, [applySessionDelta, setActiveSession, setConfig, setSessions]);
}
