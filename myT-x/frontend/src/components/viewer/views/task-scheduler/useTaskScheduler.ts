import {useState, useEffect, useCallback, useMemo, useRef} from "react";
import {
    GetTaskSchedulerStatus,
    StartTaskScheduler,
    StopTaskScheduler,
    PauseTaskScheduler,
    ResumeTaskScheduler,
    AddTaskSchedulerItem,
    RemoveTaskSchedulerItem,
    ReorderTaskSchedulerItems,
    UpdateTaskSchedulerItem,
    CheckTaskSchedulerOrchestratorReady,
} from "../../../../../wailsjs/go/main/App";
import {main, taskscheduler} from "../../../../../wailsjs/go/models";
import {EventsOn} from "../../../../../wailsjs/runtime";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {PaneSnapshot} from "../../../../types/tmux";

export type QueueStatus = taskscheduler.QueueStatus;
export type QueueItem = taskscheduler.QueueItem;
export type QueueConfig = taskscheduler.QueueConfig;

export type OrchestratorReadiness = main.TaskSchedulerOrchestratorReadiness;

export const RUNNING_ITEM_STATUS = "running";
export const PENDING_ITEM_STATUS = "pending";
export const COMPLETED_ITEM_STATUS = "completed";
export const FAILED_ITEM_STATUS = "failed";
export const SKIPPED_ITEM_STATUS = "skipped";

const EDITABLE_ITEM_STATUSES: ReadonlySet<string> = new Set([
    PENDING_ITEM_STATUS,
    COMPLETED_ITEM_STATUS,
    FAILED_ITEM_STATUS,
    SKIPPED_ITEM_STATUS,
]);

const ACTIVE_QUEUE_STATUSES: ReadonlySet<string> = new Set([
    "running",
    "paused",
    "preparing",
]);

export function isEditableStatus(status: string | null | undefined): boolean {
    if (status == null) {
        return true;
    }
    return EDITABLE_ITEM_STATUSES.has(status);
}

export function isActiveQueueStatus(status: string | null | undefined): boolean {
    if (status == null) {
        return false;
    }
    return ACTIVE_QUEUE_STATUSES.has(status);
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

function isQueueStatus(value: unknown): value is QueueStatus {
    if (!isRecord(value)) return false;
    return (
        typeof value.run_status === "string" &&
        typeof value.current_index === "number" &&
        Array.isArray(value.items)
    );
}

function stoppedMessage(value: unknown): string | null {
    if (typeof value === "string") {
        const reason = value.trim();
        return reason === "" ? null : reason;
    }
    if (!isRecord(value)) return null;
    const reason = typeof value.reason === "string" ? value.reason.trim() : "";
    return reason === "" ? null : reason;
}

export function useTaskScheduler() {
    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [status, setStatus] = useState<QueueStatus | null>(null);
    const [error, setError] = useState<string | null>(null);
    const isMountedRef = useRef(true);

    const availablePanes: PaneSnapshot[] = useMemo(() => {
        if (!activeSession) return [];
        const session = sessions.find((s) => s.name === activeSession);
        if (!session) return [];
        return session.windows.flatMap((w) => w.panes);
    }, [sessions, activeSession]);

    const refreshStatus = useCallback(() => {
        void GetTaskSchedulerStatus()
            .then((result) => {
                if (!isMountedRef.current) return;
                setStatus(result);
            })
            .catch((err) => {
                if (!isMountedRef.current) return;
                console.warn("[task-scheduler] failed to get status", err);
            });
    }, []);

    // Load status on mount + listen for events.
    // Re-subscribe when activeSession changes to pick up the correct session.
    useEffect(() => {
        isMountedRef.current = true;
        refreshStatus();

        const cancelUpdated = EventsOn("task-scheduler:updated", (data: unknown) => {
            if (!isMountedRef.current) return;
            if (isQueueStatus(data)) {
                const incoming = data as QueueStatus;
                // Filter: only accept events for the active session.
                if (incoming.session_name && incoming.session_name !== activeSession) return;
                setStatus(incoming);
            } else {
                refreshStatus();
            }
        });
        const cancelStopped = EventsOn("task-scheduler:stopped", (data: unknown) => {
            if (!isMountedRef.current) return;
            // Filter: only accept events for the active session.
            if (isRecord(data) && typeof data.session_name === "string") {
                if (data.session_name !== activeSession) return;
            }
            const reason = stoppedMessage(data);
            if (reason !== null) {
                setError(`Task failed: ${reason}`);
            }
            refreshStatus();
        });

        return () => {
            isMountedRef.current = false;
            cancelUpdated();
            cancelStopped();
        };
    }, [refreshStatus, activeSession]);

    const start = useCallback(async (config: QueueConfig, items: QueueItem[]): Promise<boolean> => {
        setError(null);
        try {
            await StartTaskScheduler(config, items);
            return true;
        } catch (e) {
            setError(String(e));
            return false;
        }
    }, []);

    const stop = useCallback(async () => {
        setError(null);
        try {
            await StopTaskScheduler();
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const pause = useCallback(async () => {
        setError(null);
        try {
            await PauseTaskScheduler();
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const resume = useCallback(async () => {
        setError(null);
        try {
            await ResumeTaskScheduler();
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const addItem = useCallback(async (
        title: string, message: string, targetPaneID: string,
        clearBefore: boolean, clearCommand: string,
    ) => {
        setError(null);
        try {
            await AddTaskSchedulerItem(title, message, targetPaneID, clearBefore, clearCommand);
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const removeItem = useCallback(async (id: string) => {
        setError(null);
        try {
            await RemoveTaskSchedulerItem(id);
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const reorderItems = useCallback(async (orderedIDs: string[]) => {
        setError(null);
        try {
            await ReorderTaskSchedulerItems(orderedIDs);
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const updateItem = useCallback(async (
        id: string, title: string, message: string, targetPaneID: string,
        clearBefore: boolean, clearCommand: string,
    ) => {
        setError(null);
        try {
            await UpdateTaskSchedulerItem(id, title, message, targetPaneID, clearBefore, clearCommand);
        } catch (e) {
            setError(String(e));
        }
    }, []);

    const checkOrchestratorReady = useCallback(async (): Promise<OrchestratorReadiness> => {
        try {
            return await CheckTaskSchedulerOrchestratorReady();
        } catch {
            return new main.TaskSchedulerOrchestratorReadiness({
                ready: false, db_exists: false, agent_count: 0, has_panes: false,
            });
        }
    }, []);

    return {
        status,
        error,
        setError,
        availablePanes,
        start,
        stop,
        pause,
        resume,
        addItem,
        removeItem,
        reorderItems,
        updateItem,
        refreshStatus,
        checkOrchestratorReady,
    };
}
