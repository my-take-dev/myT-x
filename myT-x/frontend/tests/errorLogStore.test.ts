import {beforeEach, describe, expect, it} from "vitest";
import {type ErrorLogEntry, useErrorLogStore} from "../src/stores/errorLogStore";

function entry(seq: number, overrides: Partial<ErrorLogEntry> = {}): ErrorLogEntry {
    return {
        seq,
        ts: "20260222120000",
        level: "warn",
        msg: `message-${seq}`,
        source: "test",
        ...overrides,
    };
}

describe("errorLogStore", () => {
    beforeEach(() => {
        useErrorLogStore.setState({
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
        });
    });

    it("treats initial snapshot as already read", () => {
        useErrorLogStore.getState().setEntries([entry(1), entry(2), entry(3)]);

        const state = useErrorLogStore.getState();
        expect(state.unreadCount).toBe(0);
        expect(state.lastReadSeq).toBe(3);
    });

    it("handles backend sequence restart without creating false unread count", () => {
        useErrorLogStore.getState().setEntries([entry(10)]);
        useErrorLogStore.getState().setEntries([entry(1), entry(2)]);

        let state = useErrorLogStore.getState();
        expect(state.lastReadSeq).toBe(2);
        expect(state.unreadCount).toBe(0);

        useErrorLogStore.getState().setEntries([entry(1), entry(2), entry(3)]);
        state = useErrorLogStore.getState();
        expect(state.lastReadSeq).toBe(2);
        expect(state.unreadCount).toBe(1);
    });

    it("filters malformed entries defensively", () => {
        const malformed = [
            {seq: 0, ts: "20260222120000", level: "warn", msg: "bad", source: "test"},
            {seq: 1, ts: 123, level: "warn", msg: "bad", source: "test"},
            {seq: 2, ts: "20260222120000", level: "warn", msg: "ok", source: "test"},
        ] as unknown as ErrorLogEntry[];

        useErrorLogStore.getState().setEntries(malformed);

        const state = useErrorLogStore.getState();
        expect(state.entries).toHaveLength(1);
        expect(state.entries[0].seq).toBe(2);
    });
});
