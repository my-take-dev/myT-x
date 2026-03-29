import {create} from "zustand";
import {
    GetSchedulerStatuses,
    StartScheduler,
    StopScheduler,
    DeleteScheduler,
} from "../../wailsjs/go/main/App";
import {EventsOn} from "../../wailsjs/runtime/runtime";

export const AUTO_ENTER_TITLE_PREFIX = "__auto_enter_";

interface AutoEnterEntry {
    schedulerId: string;
    intervalSeconds: number;
}

interface AutoEnterState {
    /** Map from paneId to active auto-enter info. */
    activeEntries: Record<string, AutoEnterEntry>;
}

export function extractPaneId(title: string): string {
    return title.slice(AUTO_ENTER_TITLE_PREFIX.length);
}

export function isAutoEnterTitle(title: string): boolean {
    return title.startsWith(AUTO_ENTER_TITLE_PREFIX);
}

/** Subset of scheduler.EntryStatus fields used by auto-enter sync. */
export interface SchedulerEntryLike {
    id: string;
    title: string;
    pane_id: string;
    running: boolean;
    interval_seconds: number;
}

export function isSchedulerEntryLike(v: unknown): v is SchedulerEntryLike {
    if (typeof v !== "object" || v === null) return false;
    const r = v as Record<string, unknown>;
    return (
        typeof r.id === "string" && r.id !== "" &&
        typeof r.title === "string" &&
        typeof r.pane_id === "string" &&
        typeof r.running === "boolean" &&
        typeof r.interval_seconds === "number" &&
        Number.isFinite(r.interval_seconds) && r.interval_seconds > 0
    );
}

export function buildActiveEntries(entries: SchedulerEntryLike[]): Record<string, AutoEnterEntry> {
    const result: Record<string, AutoEnterEntry> = {};
    for (const e of entries) {
        if (isAutoEnterTitle(e.title) && e.running) {
            result[e.pane_id] = {schedulerId: e.id, intervalSeconds: e.interval_seconds};
        }
    }
    return result;
}

export const useAutoEnterStore = create<AutoEnterState>(() => ({
    activeEntries: {},
}));

export function syncFromSchedulerData(data: unknown): void {
    if (!Array.isArray(data)) {
        console.warn("[DEBUG-auto-enter] syncFromSchedulerData: expected array, got", typeof data);
        return;
    }
    const valid = data.filter(isSchedulerEntryLike);
    if (valid.length !== data.length) {
        console.warn("[DEBUG-auto-enter] syncFromSchedulerData: skipped invalid entries", data.length - valid.length);
    }
    useAutoEnterStore.setState({activeEntries: buildActiveEntries(valid)});
}

// Module-level event subscription (runs once on import).
// Store cleanup function for HMR support.
let _unsubSchedulerUpdated: (() => void) | null = null;

function ensureEventSubscription(): void {
    if (_unsubSchedulerUpdated != null) return;
    _unsubSchedulerUpdated = EventsOn("scheduler:updated", (data: unknown) => {
        syncFromSchedulerData(data);
    });
}

ensureEventSubscription();

// Initial load.
void GetSchedulerStatuses()
    .then((entries) => {
        syncFromSchedulerData(entries);
    })
    .catch((err) => {
        console.warn("[DEBUG-auto-enter] initial load failed", err);
    });

/**
 * Start auto-enter for a pane.
 * If already running, attempts to stop the existing one first.
 * If the existing stop/delete fails, aborts to prevent duplicate schedulers.
 */
export async function startAutoEnter(paneId: string, intervalSeconds: number): Promise<void> {
    const existing = useAutoEnterStore.getState().activeEntries[paneId];
    if (existing) {
        try {
            await StopScheduler(existing.schedulerId);
            await DeleteScheduler(existing.schedulerId);
        } catch (err) {
            console.warn("[DEBUG-auto-enter] stop existing failed, aborting start", err);
            throw new Error("Failed to stop existing auto-enter before starting new one");
        }
    }
    const title = AUTO_ENTER_TITLE_PREFIX + paneId;
    await StartScheduler(title, paneId, "", intervalSeconds, 0);
}

/**
 * Stop auto-enter for a pane.
 * Always attempts DeleteScheduler even if StopScheduler fails.
 */
export async function stopAutoEnter(paneId: string): Promise<void> {
    const entry = useAutoEnterStore.getState().activeEntries[paneId];
    if (!entry) return;
    try {
        await StopScheduler(entry.schedulerId);
    } catch (err) {
        console.warn("[DEBUG-auto-enter] StopScheduler failed, attempting DeleteScheduler anyway", err);
    }
    await DeleteScheduler(entry.schedulerId);
}
