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
    GetTaskSchedulerSettings,
    SaveTaskSchedulerSettings,
} from "../../../../../wailsjs/go/main/App";
import {config, main, taskscheduler} from "../../../../../wailsjs/go/models";
import {EventsOn} from "../../../../../wailsjs/runtime";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {AppConfigTaskScheduler, PaneSnapshot} from "../../../../types/tmux";
import {parseConfigUpdatedPayload} from "../../../../hooks/sync/configUpdatedEvent";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {
    matchesCapturedSessionKey,
    shouldIgnoreSessionMutation,
    shouldSkipSessionMutationRequest,
} from "../../../../utils/sessionGuard";
import {normalizeGenerationId, stoppedMessage} from "../shared/queueRuntimeUtils";
import {PRE_EXEC_TARGET_MODE_TASK_PANES} from "./preExecTargetModes";

export type QueueStatus = taskscheduler.QueueStatus;
export type QueueItem = taskscheduler.QueueItem;
export type QueueConfig = taskscheduler.QueueConfig;
export type TaskSchedulerSettings = config.TaskSchedulerConfig;
export type MessageTemplate = config.MessageTemplate;

export type OrchestratorReadiness = main.TaskSchedulerOrchestratorReadiness;

export const RUNNING_ITEM_STATUS = "running";
export const PENDING_ITEM_STATUS = "pending";
export const COMPLETED_ITEM_STATUS = "completed";
export const FAILED_ITEM_STATUS = "failed";
export const SKIPPED_ITEM_STATUS = "skipped";
const userStoppedReason = "Stopped by user";
const shutdownStoppedReason = "Application shutdown";

function formatStoppedBanner(reason: string): string {
    if (reason === userStoppedReason || reason === shutdownStoppedReason) {
        return `Task stopped: ${reason}`;
    }
    return `Task failed: ${reason}`;
}

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

const defaultTaskSchedulerSettings = {
    pre_exec_reset_delay_s: 0,
    pre_exec_idle_timeout_s: 30,
    pre_exec_target_mode: PRE_EXEC_TARGET_MODE_TASK_PANES,
    message_templates: [] as Array<{name: string; message: string}>,
};

function cloneMessageTemplates(
    templates: ReadonlyArray<{name: string; message: string}> | null | undefined,
): MessageTemplate[] {
    return (templates ?? defaultTaskSchedulerSettings.message_templates)
        .map(({name, message}) => ({name, message}));
}

export function isEditableStatus(status: string | null | undefined): boolean {
    if (status == null) {
        return false;
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

function isOptionalString(value: unknown): value is string | undefined {
    return value === undefined || typeof value === "string";
}

function createTaskSchedulerSettings(
    raw: AppConfigTaskScheduler | TaskSchedulerSettings | null | undefined,
): TaskSchedulerSettings {
    const source = raw ? new config.TaskSchedulerConfig(raw) : new config.TaskSchedulerConfig(defaultTaskSchedulerSettings);
    return new config.TaskSchedulerConfig({
        ...defaultTaskSchedulerSettings,
        ...source,
        message_templates: cloneMessageTemplates(source.message_templates),
    });
}

export function isQueueItem(value: unknown): value is QueueItem {
    if (!isRecord(value)) return false;
    return (
        typeof value.id === "string" &&
        typeof value.title === "string" &&
        typeof value.message === "string" &&
        typeof value.target_pane_id === "string" &&
        typeof value.order_index === "number" &&
        typeof value.status === "string" &&
        value.status.trim() !== "" &&
        isOptionalString(value.orc_task_id) &&
        typeof value.created_at === "string" &&
        isOptionalString(value.started_at) &&
        isOptionalString(value.completed_at) &&
        isOptionalString(value.error_message) &&
        typeof value.clear_before === "boolean" &&
        isOptionalString(value.clear_command)
    );
}

export function isQueueStatus(value: unknown): value is QueueStatus {
    if (!isRecord(value)) return false;
    // TaskScheduler snapshots intentionally omit STR-only fields such as
    // clear_delay_sec and last_stop_reason.
    return (
        typeof value.run_status === "string" &&
        typeof value.current_index === "number" &&
        typeof value.session_name === "string" &&
        typeof value.generation_id === "string" &&
        Array.isArray(value.items) &&
        value.items.every(isQueueItem)
    );
}

function defaultOrchestratorReadiness(): OrchestratorReadiness {
    return {
        ready: false,
        db_exists: false,
        agent_count: 0,
        has_panes: false,
    };
}

export function useTaskScheduler() {
    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";

    const [status, setStatus] = useState<QueueStatus | null>(null);
    const [statusError, setStatusError] = useState<string | null>(null);
    const [stoppedError, setStoppedError] = useState<string | null>(null);
    const [settings, setSettings] = useState<TaskSchedulerSettings | null>(null);
    const [settingsError, setSettingsError] = useState<string | null>(null);
    const isMountedRef = useRef(true);
    const sessionRef = useRef(activeSession);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const hasResolvedSessionKeyRef = useRef(activeSession === null || activeSessionSnapshot !== null);
    const statusRequestIDRef = useRef(0);
    const generationIdRef = useRef<string | null>(null);
    const settingsRequestIDRef = useRef(0);
    const settingsEventVersionRef = useRef(0);
    const pendingSettingsCommitVersionRef = useRef<number | null>(null);

    const setError = useCallback((value: string | null) => {
        setStatusError(value);
        setStoppedError(null);
        setSettingsError(null);
    }, []);

    const error = statusError ?? stoppedError ?? settingsError;

    const availablePanes: PaneSnapshot[] = useMemo(() => {
        if (!activeSessionSnapshot) return [];
        return activeSessionSnapshot.windows.flatMap((window) => window.panes);
    }, [activeSessionSnapshot]);

    const shouldIgnoreMutationResult = useCallback((capturedSessionKey: string): boolean => {
        return shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef);
    }, []);

    useEffect(() => {
        sessionRef.current = activeSession;
        latestSessionKeyRef.current = activeSessionKey;
        hasResolvedSessionKeyRef.current = activeSession === null || activeSessionSnapshot !== null;
    }, [activeSession, activeSessionKey, activeSessionSnapshot]);

    const resolveMutationSessionKey = useCallback((): string | null => {
        const capturedSessionKey = latestSessionKeyRef.current;
        if (shouldSkipSessionMutationRequest(capturedSessionKey, hasResolvedSessionKeyRef)) {
            return null;
        }
        return capturedSessionKey;
    }, []);

    const refreshStatus = useCallback(async (capturedSessionKey: string = latestSessionKeyRef.current): Promise<boolean> => {
        if (!hasResolvedSessionKeyRef.current) {
            return false;
        }
        const capturedSession = sessionRef.current?.trim() ?? null;
        const requestID = ++statusRequestIDRef.current;
        try {
            const result = await GetTaskSchedulerStatus(capturedSessionKey);
            if (
                !isMountedRef.current
                || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)
                || statusRequestIDRef.current !== requestID
            ) {
                return false;
            }
            if (capturedSession && result.session_name && result.session_name !== capturedSession) {
                return false;
            }
            setStatus(result);
            setStatusError(null);
            generationIdRef.current = normalizeGenerationId(result.generation_id);
            return true;
        } catch (err: unknown) {
            if (
                !isMountedRef.current
                || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)
                || statusRequestIDRef.current !== requestID
            ) {
                return false;
            }
            console.warn("[task-scheduler] failed to get status", err);
            setStatus(null);
            setStatusError(toErrorMessage(err, "Failed to refresh task scheduler status"));
            generationIdRef.current = null;
            return false;
        }
    }, []);

    // Load status on mount + listen for events.
    // Re-subscribe when activeSession changes to pick up the correct session.
    useEffect(() => {
        isMountedRef.current = true;
        setStatus(null);
        setStatusError(null);
        setStoppedError(null);
        generationIdRef.current = null;
        void refreshStatus(activeSessionKey);

        const cancelUpdated = EventsOn("task-scheduler:updated", (data: unknown) => {
            if (!isMountedRef.current) return;
            if (isQueueStatus(data)) {
                if (data.session_name !== (activeSession ?? "")) return;
                const eventGenerationId = normalizeGenerationId(data.generation_id);
                if (generationIdRef.current === null || eventGenerationId === null) {
                    void refreshStatus();
                    return;
                }
                if (eventGenerationId !== generationIdRef.current) return;
                setStatus(data);
                setStatusError(null);
                generationIdRef.current = eventGenerationId;
            } else {
                console.warn("[task-scheduler] received invalid updated event payload", data);
                void refreshStatus();
            }
        });
        const cancelStopped = EventsOn("task-scheduler:stopped", (data: unknown) => {
            if (!isMountedRef.current) return;
            if (isRecord(data) && typeof data.session_name === "string") {
                if (data.session_name !== (activeSession ?? "")) return;
            }
            if (isRecord(data) && typeof data.generation_id === "string") {
                const eventGenerationId = normalizeGenerationId(data.generation_id);
                if (eventGenerationId === null) {
                    void refreshStatus();
                    return;
                }
                if (generationIdRef.current === null) {
                    void refreshStatus();
                    return;
                }
                if (eventGenerationId !== generationIdRef.current) {
                    return;
                }
            }
            const reason = stoppedMessage(data);
            if (reason !== null) {
                setStoppedError(formatStoppedBanner(reason));
            }
            void refreshStatus();
        });

        return () => {
            isMountedRef.current = false;
            cancelUpdated();
            cancelStopped();
        };
    }, [activeSession, activeSessionKey, refreshStatus]);

    const start = useCallback(async (config: QueueConfig, items: QueueItem[]): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await StartTaskScheduler(capturedSessionKey, config, items);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            generationIdRef.current = null;
            void refreshStatus(capturedSessionKey);
            return true;
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            setError(toErrorMessage(err, "Failed to start task scheduler"));
            return false;
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const stop = useCallback(async () => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return;
        }
        try {
            await StopTaskScheduler(capturedSessionKey);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to stop task scheduler"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const pause = useCallback(async () => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return;
        }
        try {
            await PauseTaskScheduler(capturedSessionKey);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            // Pause/resume updates arrive through task-scheduler:updated, so pause
            // intentionally avoids a second eager refresh here.
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to pause task scheduler"));
        }
    }, [resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const resume = useCallback(async () => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return;
        }
        try {
            await ResumeTaskScheduler(capturedSessionKey);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            generationIdRef.current = null;
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to resume task scheduler"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const addItem = useCallback(async (
        title: string, message: string, targetPaneID: string,
        clearBefore: boolean, clearCommand: string,
    ): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await AddTaskSchedulerItem(capturedSessionKey, title, message, targetPaneID, clearBefore, clearCommand);
            return !shouldIgnoreMutationResult(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            setError(toErrorMessage(err, "Failed to add task scheduler item"));
            return false;
        }
    }, [resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const removeItem = useCallback(async (id: string) => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return;
        }
        try {
            await RemoveTaskSchedulerItem(capturedSessionKey, id);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to remove task scheduler item"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const reorderItems = useCallback(async (orderedIDs: string[]) => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return;
        }
        try {
            await ReorderTaskSchedulerItems(capturedSessionKey, orderedIDs);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to reorder task scheduler items"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const updateItem = useCallback(async (
        id: string, title: string, message: string, targetPaneID: string,
        clearBefore: boolean, clearCommand: string,
    ): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await UpdateTaskSchedulerItem(capturedSessionKey, id, title, message, targetPaneID, clearBefore, clearCommand);
            return !shouldIgnoreMutationResult(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            setError(toErrorMessage(err, "Failed to update task scheduler item"));
            return false;
        }
    }, [resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const loadSettings = useCallback(async (requestID?: number): Promise<boolean> => {
        if (requestID !== undefined && settingsRequestIDRef.current !== requestID) {
            return false;
        }
        const effectiveRequestID = requestID ?? settingsRequestIDRef.current + 1;
        if (requestID === undefined) {
            settingsRequestIDRef.current = effectiveRequestID;
        }
        try {
            const result = await GetTaskSchedulerSettings();
            if (!isMountedRef.current || settingsRequestIDRef.current !== effectiveRequestID) {
                return false;
            }
            setSettings(createTaskSchedulerSettings(result));
            setSettingsError(null);
            return true;
        } catch (err: unknown) {
            if (!isMountedRef.current || settingsRequestIDRef.current !== effectiveRequestID) {
                return false;
            }
            setSettingsError(toErrorMessage(err, "Failed to load task scheduler settings"));
            return false;
        }
    }, []);

    const saveSettings = useCallback(async (s: TaskSchedulerSettings): Promise<boolean> => {
        const requestID = ++settingsRequestIDRef.current;
        const previousSettingsVersion = settingsEventVersionRef.current;
        pendingSettingsCommitVersionRef.current = previousSettingsVersion;
        setStatusError(null);
        setSettingsError(null);
        try {
            await SaveTaskSchedulerSettings(s);
            if (!isMountedRef.current) {
                if (pendingSettingsCommitVersionRef.current === previousSettingsVersion) {
                    pendingSettingsCommitVersionRef.current = null;
                }
                return false;
            }
            if (pendingSettingsCommitVersionRef.current !== previousSettingsVersion) {
                return true;
            }
            const reloaded = await loadSettings(requestID);
            if (pendingSettingsCommitVersionRef.current === previousSettingsVersion) {
                pendingSettingsCommitVersionRef.current = null;
            }
            if (!isMountedRef.current) {
                return false;
            }
            return reloaded || settingsRequestIDRef.current !== requestID;
        } catch (err: unknown) {
            if (pendingSettingsCommitVersionRef.current === previousSettingsVersion) {
                pendingSettingsCommitVersionRef.current = null;
            }
            if (!isMountedRef.current) {
                return false;
            }
            setSettingsError(toErrorMessage(err, "Failed to save task scheduler settings"));
            return false;
        }
    }, [loadSettings]);

    // Load settings on mount and sync when config changes externally.
    useEffect(() => {
        void loadSettings();
        const cancelConfigUpdated = EventsOn("config:updated", (payload) => {
            if (!isMountedRef.current) return;
            const event = parseConfigUpdatedPayload(payload);
            if (!event) {
                void loadSettings();
                return;
            }

            const nextVersion = event.version ?? settingsEventVersionRef.current + 1;
            if (nextVersion <= settingsEventVersionRef.current) {
                return;
            }

            settingsRequestIDRef.current += 1;
            settingsEventVersionRef.current = nextVersion;
            if (
                pendingSettingsCommitVersionRef.current !== null
                && nextVersion > pendingSettingsCommitVersionRef.current
            ) {
                pendingSettingsCommitVersionRef.current = null;
            }
            if (event.config.task_scheduler === undefined) {
                void loadSettings();
                return;
            }
            setSettings(createTaskSchedulerSettings(event.config.task_scheduler));
            setSettingsError(null);
        });
        return () => {
            cancelConfigUpdated();
        };
    }, [loadSettings]);

    const checkOrchestratorReady = useCallback(async (): Promise<OrchestratorReadiness> => {
        const capturedSessionKey = resolveMutationSessionKey();
        if (capturedSessionKey === null) {
            return defaultOrchestratorReadiness();
        }
        try {
            return await CheckTaskSchedulerOrchestratorReady(capturedSessionKey);
        } catch (err: unknown) {
            if (!shouldIgnoreMutationResult(capturedSessionKey)) {
                // View callbacks intentionally stop after this throw because the
                // shared hook error state already owns the user-visible message.
                setError(toErrorMessage(err, "Failed to check task scheduler readiness"));
            }
            throw err;
        }
    }, [resolveMutationSessionKey, shouldIgnoreMutationResult]);

    return {
        status,
        error,
        setError,
        availablePanes,
        settings,
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
        saveSettings,
    };
}
