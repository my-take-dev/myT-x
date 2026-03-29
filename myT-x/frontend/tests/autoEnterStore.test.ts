import {describe, expect, it, vi} from "vitest";

// Mock Wails runtime and App bindings before importing the store.
vi.mock("../wailsjs/runtime/runtime", () => ({
    EventsOn: vi.fn(() => vi.fn()),
}));

vi.mock("../wailsjs/go/main/App", () => ({
    GetSchedulerStatuses: vi.fn(() => Promise.resolve([])),
    StartScheduler: vi.fn(() => Promise.resolve("")),
    StopScheduler: vi.fn(() => Promise.resolve()),
    DeleteScheduler: vi.fn(() => Promise.resolve()),
}));

import {
    AUTO_ENTER_TITLE_PREFIX,
    buildActiveEntries,
    extractPaneId,
    isAutoEnterTitle,
    isSchedulerEntryLike,
    syncFromSchedulerData,
    useAutoEnterStore,
    type SchedulerEntryLike,
} from "../src/stores/autoEnterStore";

// ---------------------------------------------------------------------------
// isAutoEnterTitle
// ---------------------------------------------------------------------------

describe("isAutoEnterTitle", () => {
    it("returns true for titles with the auto-enter prefix", () => {
        expect(isAutoEnterTitle("__auto_enter_%1")).toBe(true);
        expect(isAutoEnterTitle("__auto_enter_abc")).toBe(true);
    });

    it("returns false for titles without the prefix", () => {
        expect(isAutoEnterTitle("my-scheduler")).toBe(false);
        expect(isAutoEnterTitle("")).toBe(false);
        expect(isAutoEnterTitle("auto_enter_%1")).toBe(false);
    });
});

// ---------------------------------------------------------------------------
// extractPaneId
// ---------------------------------------------------------------------------

describe("extractPaneId", () => {
    it("extracts the pane id by stripping the prefix", () => {
        expect(extractPaneId("__auto_enter_%1")).toBe("%1");
        expect(extractPaneId("__auto_enter_abc-def")).toBe("abc-def");
    });

    it("returns empty string when title equals the prefix exactly", () => {
        expect(extractPaneId(AUTO_ENTER_TITLE_PREFIX)).toBe("");
    });
});

// ---------------------------------------------------------------------------
// isSchedulerEntryLike
// ---------------------------------------------------------------------------

describe("isSchedulerEntryLike", () => {
    const validEntry: SchedulerEntryLike = {
        id: "sched-1",
        title: "__auto_enter_%1",
        pane_id: "%1",
        running: true,
        interval_seconds: 5,
    };

    it("accepts a valid entry", () => {
        expect(isSchedulerEntryLike(validEntry)).toBe(true);
    });

    it("rejects null and non-objects", () => {
        expect(isSchedulerEntryLike(null)).toBe(false);
        expect(isSchedulerEntryLike(undefined)).toBe(false);
        expect(isSchedulerEntryLike("string")).toBe(false);
        expect(isSchedulerEntryLike(42)).toBe(false);
    });

    it("rejects entries with empty id", () => {
        expect(isSchedulerEntryLike({...validEntry, id: ""})).toBe(false);
    });

    it("rejects entries with wrong field types", () => {
        expect(isSchedulerEntryLike({...validEntry, running: "true"})).toBe(false);
        expect(isSchedulerEntryLike({...validEntry, interval_seconds: "5"})).toBe(false);
    });

    it("rejects entries with NaN interval_seconds", () => {
        expect(isSchedulerEntryLike({...validEntry, interval_seconds: NaN})).toBe(false);
    });

    it("rejects entries with non-positive interval_seconds", () => {
        expect(isSchedulerEntryLike({...validEntry, interval_seconds: 0})).toBe(false);
        expect(isSchedulerEntryLike({...validEntry, interval_seconds: -1})).toBe(false);
    });

    it("rejects entries with Infinity interval_seconds", () => {
        expect(isSchedulerEntryLike({...validEntry, interval_seconds: Infinity})).toBe(false);
    });

    it("rejects entries missing pane_id", () => {
        const {pane_id: _, ...noPaneId} = validEntry;
        expect(isSchedulerEntryLike(noPaneId)).toBe(false);
    });
});

// ---------------------------------------------------------------------------
// buildActiveEntries
// ---------------------------------------------------------------------------

describe("buildActiveEntries", () => {
    it("builds entries from running auto-enter schedulers only", () => {
        const entries: SchedulerEntryLike[] = [
            {id: "s1", title: "__auto_enter_%1", pane_id: "%1", running: true, interval_seconds: 5},
            {id: "s2", title: "__auto_enter_%2", pane_id: "%2", running: false, interval_seconds: 10},
            {id: "s3", title: "manual-scheduler", pane_id: "%3", running: true, interval_seconds: 1},
        ];

        const result = buildActiveEntries(entries);

        expect(Object.keys(result)).toEqual(["%1"]);
        expect(result["%1"]).toEqual({schedulerId: "s1", intervalSeconds: 5});
    });

    it("returns empty record when no entries match", () => {
        const entries: SchedulerEntryLike[] = [
            {id: "s1", title: "not-auto", pane_id: "%1", running: true, interval_seconds: 5},
        ];

        expect(buildActiveEntries(entries)).toEqual({});
    });

    it("returns empty record for empty input", () => {
        expect(buildActiveEntries([])).toEqual({});
    });

    it("uses pane_id field directly", () => {
        const entries: SchedulerEntryLike[] = [
            {id: "s1", title: "__auto_enter_%1", pane_id: "%99", running: true, interval_seconds: 3},
        ];

        const result = buildActiveEntries(entries);
        expect(result["%99"]).toEqual({schedulerId: "s1", intervalSeconds: 3});
        expect(result["%1"]).toBeUndefined();
    });
});

// ---------------------------------------------------------------------------
// syncFromSchedulerData
// ---------------------------------------------------------------------------

describe("syncFromSchedulerData", () => {
    it("populates store with valid auto-enter entries", () => {
        const data = [
            {id: "s1", title: "__auto_enter_%1", pane_id: "%1", running: true, interval_seconds: 5},
        ];

        syncFromSchedulerData(data);

        const state = useAutoEnterStore.getState();
        expect(state.activeEntries["%1"]).toEqual({schedulerId: "s1", intervalSeconds: 5});
    });

    it("clears store when given empty array", () => {
        useAutoEnterStore.setState({
            activeEntries: {"%1": {schedulerId: "s1", intervalSeconds: 5}},
        });

        syncFromSchedulerData([]);

        expect(useAutoEnterStore.getState().activeEntries).toEqual({});
    });

    it("does not crash on non-array input (null, string, number)", () => {
        useAutoEnterStore.setState({
            activeEntries: {"%1": {schedulerId: "s1", intervalSeconds: 5}},
        });

        // Non-array input should be a no-op (state unchanged)
        syncFromSchedulerData(null);
        expect(useAutoEnterStore.getState().activeEntries["%1"]).toBeDefined();

        syncFromSchedulerData("bad data");
        expect(useAutoEnterStore.getState().activeEntries["%1"]).toBeDefined();

        syncFromSchedulerData(42);
        expect(useAutoEnterStore.getState().activeEntries["%1"]).toBeDefined();
    });

    it("filters out invalid entries from mixed array", () => {
        const data = [
            {id: "s1", title: "__auto_enter_%1", pane_id: "%1", running: true, interval_seconds: 5},
            {id: "", title: "__auto_enter_%2", pane_id: "%2", running: true, interval_seconds: 3},
            null,
            "garbage",
        ];

        syncFromSchedulerData(data);

        const state = useAutoEnterStore.getState();
        expect(Object.keys(state.activeEntries)).toEqual(["%1"]);
    });
});
