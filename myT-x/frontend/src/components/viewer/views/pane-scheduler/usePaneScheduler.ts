import {useState, useEffect, useCallback, useMemo, useRef} from "react";
import {
    DeleteScheduler,
    GetSchedulerStatuses,
    StartScheduler,
    ResumeScheduler,
    StopScheduler,
    LoadSchedulerTemplates,
    SaveSchedulerTemplate,
    DeleteSchedulerTemplate,
} from "../../../../../wailsjs/go/main/App";
import {main} from "../../../../../wailsjs/go/models";
import {EventsOn} from "../../../../../wailsjs/runtime";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {PaneSnapshot} from "../../../../types/tmux";

export type SchedulerEntry = main.SchedulerEntryStatus;
export type SchedulerTemplate = main.SchedulerTemplate;

export interface SchedulerStartValues {
    title: string;
    paneID: string;
    message: string;
    intervalMinutes: number;
    maxCount: number;
}

export interface SchedulerEditDraft extends SchedulerStartValues {
    sourceID: string;
    running: boolean;
}

export const SCHEDULER_INFINITE_COUNT = 0;

const SCHEDULER_MIN_COUNT = 0;

export function clampSchedulerMaxCount(maxCount: number): number {
    if (!Number.isFinite(maxCount)) {
        return SCHEDULER_MIN_COUNT;
    }
    const normalized = Math.trunc(maxCount);
    return Math.max(SCHEDULER_MIN_COUNT, normalized);
}

export function isSchedulerMaxCountValid(maxCount: number): boolean {
    if (!Number.isFinite(maxCount) || Math.trunc(maxCount) !== maxCount) {
        return false;
    }
    return maxCount >= SCHEDULER_MIN_COUNT;
}

export function isSchedulerInfiniteCount(maxCount: number): boolean {
    return maxCount === SCHEDULER_INFINITE_COUNT;
}

export function createSchedulerEditDraft(entry: SchedulerEntry): SchedulerEditDraft {
    return {
        sourceID: entry.id,
        running: entry.running,
        title: entry.title,
        paneID: entry.pane_id,
        message: entry.message,
        intervalMinutes: entry.interval_minutes,
        maxCount: entry.max_count,
    };
}

export async function submitSchedulerChanges(
    start: (values: SchedulerStartValues) => Promise<void>,
    stop: (id: string) => Promise<void>,
    remove: (id: string) => Promise<void>,
    values: SchedulerStartValues,
    source: SchedulerEditDraft | null = null,
): Promise<void> {
    if (source?.running) {
        await stop(source.sourceID);
    }
    await start(values);
    if (source !== null) {
        await remove(source.sourceID);
    }
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

function isSchedulerEntry(value: unknown): value is SchedulerEntry {
    if (!isRecord(value)) {
        return false;
    }
    return (
        typeof value.id === "string" &&
        typeof value.title === "string" &&
        typeof value.pane_id === "string" &&
        typeof value.message === "string" &&
        typeof value.interval_minutes === "number" &&
        typeof value.max_count === "number" &&
        typeof value.current_count === "number" &&
        typeof value.running === "boolean"
    );
}

function isSchedulerEntriesPayload(value: unknown): value is SchedulerEntry[] {
    return Array.isArray(value) && value.every(isSchedulerEntry);
}

function schedulerStoppedMessage(value: unknown): string | null {
    if (typeof value === "string") {
        const reason = value.trim();
        return reason === "" ? null : reason;
    }
    if (!isRecord(value)) {
        return null;
    }

    const reason = typeof value.reason === "string" ? value.reason.trim() : "";
    const message = typeof value.message === "string" ? value.message.trim() : "";
    const title = typeof value.title === "string" ? value.title.trim() : "";
    const paneID = typeof value.pane_id === "string" ? value.pane_id.trim() : "";
    const base = reason || message;
    if (base === "") {
        return null;
    }
    if (title !== "") {
        return `${title}: ${base}`;
    }
    if (paneID !== "") {
        return `${paneID}: ${base}`;
    }
    return base;
}

export function usePaneScheduler() {
    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [entries, setEntries] = useState<SchedulerEntry[]>([]);
    const [templates, setTemplates] = useState<SchedulerTemplate[]>([]);
    const [error, setError] = useState<string | null>(null);
    const isMountedRef = useRef(true);
    const hasLoadedStatusesRef = useRef(false);
    const hasLoadedTemplatesRef = useRef(false);

    // Derive available panes from active session.
    const availablePanes: PaneSnapshot[] = useMemo(() => {
        if (!activeSession) return [];
        const session = sessions.find((s) => s.name === activeSession);
        if (!session) return [];
        return session.windows.flatMap((w) => w.panes);
    }, [sessions, activeSession]);

    const refreshStatuses = useCallback(() => {
        void GetSchedulerStatuses()
            .then((result) => {
                if (!isMountedRef.current) return;
                setEntries(result ?? []);
                hasLoadedStatusesRef.current = true;
            })
            .catch((err) => {
                if (!isMountedRef.current) return;
                console.warn("[pane-scheduler] failed to get statuses", err);
                if (!hasLoadedStatusesRef.current) {
                    setError("Failed to load scheduler statuses. Please refresh.");
                }
            });
    }, []);

    const refreshTemplates = useCallback(() => {
        if (!activeSession) {
            setTemplates([]);
            hasLoadedTemplatesRef.current = false;
            return;
        }
        setTemplates([]);
        void LoadSchedulerTemplates(activeSession)
            .then((result) => {
                if (!isMountedRef.current) return;
                setTemplates(result ?? []);
                hasLoadedTemplatesRef.current = true;
            })
            .catch((err) => {
                if (!isMountedRef.current) return;
                console.warn("[pane-scheduler] failed to load templates", err);
                if (!hasLoadedTemplatesRef.current) {
                    setError(`Failed to load templates for session ${activeSession}.`);
                }
            });
    }, [activeSession]);

    // Load statuses on mount + listen for scheduler events.
    useEffect(() => {
        isMountedRef.current = true;
        refreshStatuses();

        const cancelUpdated = EventsOn("scheduler:updated", (data: unknown) => {
            if (!isMountedRef.current) return;
            if (isSchedulerEntriesPayload(data)) {
                setEntries(data);
                hasLoadedStatusesRef.current = true;
            } else {
                refreshStatuses();
            }
        });
        const cancelStopped = EventsOn("scheduler:stopped", (data: unknown) => {
            if (!isMountedRef.current) return;
            const reason = schedulerStoppedMessage(data);
            if (reason !== null) {
                setError(`Scheduler stopped: ${reason}`);
            }
            refreshStatuses();
        });

        return () => {
            isMountedRef.current = false;
            cancelUpdated();
            cancelStopped();
        };
    }, [refreshStatuses]);

    // Load templates when active session changes.
    useEffect(() => {
        hasLoadedTemplatesRef.current = false;
        refreshTemplates();
    }, [refreshTemplates]);

    const start = useCallback(
        async ({
            title,
            paneID,
            message,
            intervalMinutes,
            maxCount,
        }: SchedulerStartValues) => {
            setError(null);
            try {
                await StartScheduler(title, paneID, message, intervalMinutes, maxCount);
            } catch (e) {
                const msg = String(e);
                setError(msg);
                throw e;
            }
        },
        [],
    );

    const stopOrThrow = useCallback(async (id: string) => {
        setError(null);
        try {
            await StopScheduler(id);
        } catch (e) {
            setError(String(e));
            throw e;
        }
    }, []);

    const stop = useCallback(async (id: string) => {
        try {
            await stopOrThrow(id);
        } catch {
            // Errors are already surfaced via setError for list-triggered stops.
        }
    }, [stopOrThrow]);

    const resumeOrThrow = useCallback(async (id: string) => {
        setError(null);
        try {
            await ResumeScheduler(id);
        } catch (e) {
            setError(String(e));
            throw e;
        }
    }, []);

    const resume = useCallback(async (id: string) => {
        try {
            await resumeOrThrow(id);
        } catch {
            // Errors are already surfaced via setError for list-triggered resumes.
        }
    }, [resumeOrThrow]);

    const deleteSchedulerOrThrow = useCallback(async (id: string) => {
        setError(null);
        try {
            await DeleteScheduler(id);
        } catch (e) {
            setError(String(e));
            throw e;
        }
    }, []);

    const deleteScheduler = useCallback(async (id: string) => {
        try {
            await deleteSchedulerOrThrow(id);
        } catch {
            // Errors are already surfaced via setError for list-triggered deletes.
        }
    }, [deleteSchedulerOrThrow]);

    const saveTemplate = useCallback(
        async (tmpl: SchedulerTemplate) => {
            if (!activeSession) return;
            setError(null);
            try {
                await SaveSchedulerTemplate(activeSession, tmpl);
                refreshTemplates();
            } catch (e) {
                setError(String(e));
            }
        },
        [activeSession, refreshTemplates],
    );

    const deleteTemplate = useCallback(
        async (title: string) => {
            if (!activeSession) return;
            setError(null);
            try {
                await DeleteSchedulerTemplate(activeSession, title);
                refreshTemplates();
            } catch (e) {
                setError(String(e));
            }
        },
        [activeSession, refreshTemplates],
    );

    return {
        entries,
        templates,
        error,
        setError,
        availablePanes,
        start,
        stop,
        stopOrThrow,
        resume,
        resumeOrThrow,
        deleteScheduler,
        deleteSchedulerOrThrow,
        saveTemplate,
        deleteTemplate,
        refreshStatuses,
    };
}
