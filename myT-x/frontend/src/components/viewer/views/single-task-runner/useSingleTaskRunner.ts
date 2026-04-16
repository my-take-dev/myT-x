import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {
    AddSingleTaskRunnerItem,
    GetSingleTaskRunnerClearDelay,
    GetSingleTaskRunnerStatus,
    RemoveSingleTaskRunnerItem,
    ReorderSingleTaskRunnerItems,
    SetSingleTaskRunnerClearDelay,
    StartSingleTaskRunner,
    StopSingleTaskRunner,
    UpdateSingleTaskRunnerItem,
} from "../../../../../wailsjs/go/main/App";
import {singletaskrunner} from "../../../../../wailsjs/go/models";
import {EventsOn} from "../../../../../wailsjs/runtime";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {PaneSnapshot} from "../../../../types/tmux";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {
    matchesCapturedSessionKey,
    shouldIgnoreSessionMutation,
    shouldSkipSessionMutationRequest,
} from "../../../../utils/sessionGuard";
import {normalizeGenerationId, stoppedMessage} from "../shared/queueRuntimeUtils";

export type QueueStatus = singletaskrunner.QueueStatus;
export type QueueItem = singletaskrunner.QueueItem;

// Keep this in sync with backend QueueItemStatus.IsEditable().
// Runtime-only states such as "sending" and "active" stay outside this allowlist.
const EDITABLE_ITEM_STATUSES: ReadonlySet<string> = new Set([
    "pending",
    "done",
    "failed",
    "cancelled",
]);

// Keep this in sync with backend queue run status handling.
const ACTIVE_QUEUE_STATUSES: ReadonlySet<string> = new Set([
    "running",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

function isOptionalString(value: unknown): value is string | undefined {
    return value === undefined || typeof value === "string";
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
        typeof value.created_at === "string" &&
        isOptionalString(value.started_at) &&
        isOptionalString(value.completed_at) &&
        isOptionalString(value.error_message) &&
        isOptionalString(value.result_message) &&
        typeof value.clear_before === "boolean" &&
        isOptionalString(value.clear_command)
    );
}

export function isQueueStatus(value: unknown): value is QueueStatus {
    if (!isRecord(value)) return false;
    return (
        typeof value.run_status === "string" &&
        typeof value.current_index === "number" &&
        typeof value.session_name === "string" &&
        typeof value.generation_id === "string" &&
        typeof value.clear_delay_sec === "number" &&
        isOptionalString(value.last_stop_reason) &&
        Array.isArray(value.items) &&
        value.items.every(isQueueItem)
    );
}

function stoppedMessageFromStatus(status: QueueStatus): string | null {
    if (isActiveQueueStatus(status.run_status)) {
        return null;
    }
    return stoppedMessage(status.last_stop_reason);
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

export function useSingleTaskRunner() {
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
    const [defaultClearDelay, setDefaultClearDelay] = useState<number | null>(null);
    const isMountedRef = useRef(true);
    const generationIdRef = useRef<string | null>(null);
    const sessionRef = useRef(activeSession);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const hasResolvedSessionKeyRef = useRef(activeSession === null || activeSessionSnapshot !== null);
    const refreshRequestTokenRef = useRef(0);
    const clearDelayRequestTokenRef = useRef(0);

    const setError = useCallback((value: string | null) => {
        setStatusError(value);
        setStoppedError(null);
    }, []);

    const error = statusError ?? stoppedError;

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
        const requestToken = ++refreshRequestTokenRef.current;
        try {
            const result = await GetSingleTaskRunnerStatus(capturedSessionKey);
            if (
                !isMountedRef.current
                || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)
                || refreshRequestTokenRef.current !== requestToken
            ) {
                return false;
            }
            if (capturedSession && result.session_name && result.session_name !== capturedSession) {
                return false;
            }
            setStatus(result);
            setStatusError(null);
            setDefaultClearDelay(result.clear_delay_sec);
            const stopReason = stoppedMessageFromStatus(result);
            if (stopReason !== null) {
                setStoppedError(`Task stopped: ${stopReason}`);
            }
            generationIdRef.current = normalizeGenerationId(result.generation_id);
            return true;
        } catch (err: unknown) {
            if (
                !isMountedRef.current
                || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)
                || refreshRequestTokenRef.current !== requestToken
            ) {
                return false;
            }
            console.warn("[single-task-runner] failed to get status", err);
            setStatus(null);
            setStatusError(toErrorMessage(err, "Failed to refresh task runner status"));
            generationIdRef.current = null;
            return false;
        }
    }, []);

    const loadDefaultClearDelay = useCallback(async (capturedSessionKey: string = latestSessionKeyRef.current): Promise<boolean> => {
        if (!hasResolvedSessionKeyRef.current) {
            return false;
        }
        const requestToken = ++clearDelayRequestTokenRef.current;
        try {
            const delay = await GetSingleTaskRunnerClearDelay(capturedSessionKey);
            if (
                !isMountedRef.current
                || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)
                || clearDelayRequestTokenRef.current !== requestToken
            ) {
                return false;
            }
            setDefaultClearDelay(delay);
            return true;
        } catch (err: unknown) {
            if (
                !isMountedRef.current
                || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)
                || clearDelayRequestTokenRef.current !== requestToken
            ) {
                return false;
            }
            console.warn("[single-task-runner] failed to load clear delay", err);
            setDefaultClearDelay(null);
            return false;
        }
    }, []);

    useEffect(() => {
        isMountedRef.current = true;
        setStatus(null);
        setStatusError(null);
        setStoppedError(null);
        setDefaultClearDelay(null);
        generationIdRef.current = null;
        void refreshStatus(activeSessionKey);
        void loadDefaultClearDelay(activeSessionKey);

        const cancelUpdated = EventsOn("single-task-runner:updated", (data: unknown) => {
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
                const stopReason = stoppedMessageFromStatus(data);
                if (stopReason !== null) {
                    setStoppedError(`Task stopped: ${stopReason}`);
                }
                generationIdRef.current = eventGenerationId;
                return;
            }
            console.warn("[single-task-runner] received invalid updated event payload", data);
            void refreshStatus();
        });
        const cancelStopped = EventsOn("single-task-runner:stopped", (data: unknown) => {
            if (!isMountedRef.current) return;
            if (isRecord(data) && typeof data.session_name === "string" && data.session_name !== (activeSession ?? "")) {
                return;
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
                setStoppedError(`Task stopped: ${reason}`);
            }
            void refreshStatus();
        });

        return () => {
            isMountedRef.current = false;
            cancelUpdated();
            cancelStopped();
        };
    }, [activeSession, activeSessionKey, loadDefaultClearDelay, refreshStatus]);

    const start = useCallback(async (): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await StartSingleTaskRunner(capturedSessionKey);
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
            setError(toErrorMessage(err, "Failed to start task runner"));
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
            await StopSingleTaskRunner(capturedSessionKey);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to stop task runner"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const addItem = useCallback(async (
        title: string,
        message: string,
        targetPaneID: string,
        clearBefore: boolean,
        clearCommand: string,
    ): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await AddSingleTaskRunnerItem(capturedSessionKey, title, message, targetPaneID, clearBefore, clearCommand);
            return !shouldIgnoreMutationResult(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            setError(toErrorMessage(err, "Failed to add task"));
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
            await RemoveSingleTaskRunnerItem(capturedSessionKey, id);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to remove task"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const updateItem = useCallback(async (
        id: string,
        title: string,
        message: string,
        targetPaneID: string,
        clearBefore: boolean,
        clearCommand: string,
    ): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await UpdateSingleTaskRunnerItem(capturedSessionKey, id, title, message, targetPaneID, clearBefore, clearCommand);
            return !shouldIgnoreMutationResult(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            setError(toErrorMessage(err, "Failed to update task"));
            return false;
        }
    }, [resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const reorderItems = useCallback(async (orderedIDs: string[]) => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return;
        }
        try {
            await ReorderSingleTaskRunnerItems(capturedSessionKey, orderedIDs);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            void refreshStatus(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to reorder tasks"));
        }
    }, [refreshStatus, resolveMutationSessionKey, shouldIgnoreMutationResult]);

    const setClearDelay = useCallback(async (delaySec: number): Promise<boolean> => {
        const capturedSessionKey = resolveMutationSessionKey();
        setError(null);
        if (capturedSessionKey === null) {
            return false;
        }
        try {
            await SetSingleTaskRunnerClearDelay(capturedSessionKey, delaySec);
            return !shouldIgnoreMutationResult(capturedSessionKey);
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return false;
            }
            setError(toErrorMessage(err, "Failed to update clear delay"));
            return false;
        }
    }, [resolveMutationSessionKey, shouldIgnoreMutationResult]);

    return {
        status,
        error,
        setError,
        availablePanes,
        defaultClearDelay,
        start,
        stop,
        addItem,
        removeItem,
        updateItem,
        reorderItems,
        setClearDelay,
        refreshStatus,
    };
}
