import {describe, expect, it, vi} from "vitest";
import {
    createSchedulerEditDraft,
    submitSchedulerChanges,
    type SchedulerEntry,
    type SchedulerStartValues,
} from "../src/components/viewer/views/pane-scheduler/usePaneScheduler";

function makeEntry(overrides: Partial<SchedulerEntry> = {}): SchedulerEntry {
    return {
        id: "scheduler-1",
        title: "Nightly",
        pane_id: "%3",
        message: "run sync",
        interval_seconds: 15,
        max_count: 4,
        current_count: 1,
        running: true,
        ...overrides,
    };
}

function makeValues(overrides: Partial<SchedulerStartValues> = {}): SchedulerStartValues {
    return {
        title: "Nightly",
        paneID: "%3",
        message: "run sync",
        intervalSeconds: 15,
        maxCount: 4,
        ...overrides,
    };
}

describe("pane scheduler editing helpers", () => {
    it("creates an edit draft from an existing scheduler entry", () => {
        const draft = createSchedulerEditDraft(makeEntry());

        expect(draft).toEqual({
            sourceID: "scheduler-1",
            running: true,
            title: "Nightly",
            paneID: "%3",
            message: "run sync",
            intervalSeconds: 15,
            maxCount: 4,
        });
    });

    it("starts a new scheduler without stopping anything", async () => {
        const start = vi.fn().mockResolvedValue(undefined);
        const stop = vi.fn().mockResolvedValue(undefined);
        const remove = vi.fn().mockResolvedValue(undefined);

        await submitSchedulerChanges(start, stop, remove, makeValues());

        expect(stop).not.toHaveBeenCalled();
        expect(start).toHaveBeenCalledWith(makeValues());
        expect(remove).not.toHaveBeenCalled();
    });

    it("replaces a running scheduler by stop, start, then delete", async () => {
        const calls: string[] = [];
        const start = vi.fn().mockImplementation(async () => {
            calls.push("start");
        });
        const stop = vi.fn().mockImplementation(async () => {
            calls.push("stop");
        });
        const remove = vi.fn().mockImplementation(async () => {
            calls.push("delete");
        });

        await submitSchedulerChanges(
            start,
            stop,
            remove,
            makeValues({intervalSeconds: 30}),
            createSchedulerEditDraft(makeEntry()),
        );

        expect(stop).toHaveBeenCalledWith("scheduler-1");
        expect(start).toHaveBeenCalledWith(makeValues({intervalSeconds: 30}));
        expect(remove).toHaveBeenCalledWith("scheduler-1");
        expect(calls).toEqual(["stop", "start", "delete"]);
    });

    it("replaces a stopped scheduler without stopping it again", async () => {
        const calls: string[] = [];
        const start = vi.fn().mockImplementation(async () => {
            calls.push("start");
        });
        const stop = vi.fn().mockImplementation(async () => {
            calls.push("stop");
        });
        const remove = vi.fn().mockImplementation(async () => {
            calls.push("delete");
        });

        await submitSchedulerChanges(
            start,
            stop,
            remove,
            makeValues({intervalSeconds: 30}),
            createSchedulerEditDraft(makeEntry({running: false})),
        );

        expect(stop).not.toHaveBeenCalled();
        expect(start).toHaveBeenCalledWith(makeValues({intervalSeconds: 30}));
        expect(remove).toHaveBeenCalledWith("scheduler-1");
        expect(calls).toEqual(["start", "delete"]);
    });
});
