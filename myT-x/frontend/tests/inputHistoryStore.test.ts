import {beforeEach, describe, expect, it} from "vitest";
import {type InputHistoryEntry, useInputHistoryStore} from "../src/stores/inputHistoryStore";

function entry(seq: number, overrides: Partial<InputHistoryEntry> = {}): InputHistoryEntry {
    return {
        seq,
        ts: "20260222120000",
        pane_id: "%0",
        input: `input-${seq}`,
        source: "test",
        session: "sess1",
        ...overrides,
    };
}

describe("inputHistoryStore", () => {
    beforeEach(() => {
        useInputHistoryStore.setState({
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
        });
    });

    it("treats initial snapshot as already read", () => {
        useInputHistoryStore.getState().setEntries([entry(1), entry(2), entry(3)]);

        const state = useInputHistoryStore.getState();
        expect(state.unreadCount).toBe(0);
        expect(state.lastReadSeq).toBe(3);
    });

    it("counts new entries as unread after initial load", () => {
        useInputHistoryStore.getState().setEntries([entry(1), entry(2)]);

        useInputHistoryStore.getState().setEntries([entry(1), entry(2), entry(3), entry(4)]);

        expect(useInputHistoryStore.getState().unreadCount).toBe(2);
    });

    it("handles backend sequence restart without false unread count", () => {
        useInputHistoryStore.getState().setEntries([entry(10)]);
        useInputHistoryStore.getState().setEntries([entry(1), entry(2)]);

        let state = useInputHistoryStore.getState();
        expect(state.lastReadSeq).toBe(2);
        expect(state.unreadCount).toBe(0);

        useInputHistoryStore.getState().setEntries([entry(1), entry(2), entry(3)]);
        state = useInputHistoryStore.getState();
        expect(state.lastReadSeq).toBe(2);
        expect(state.unreadCount).toBe(1);
    });

    it("filters malformed entries defensively", () => {
        const malformed = [
            {seq: 0, ts: "20260222120000", pane_id: "%0", input: "bad", source: "test", session: "s"},
            {seq: 1, ts: 123, pane_id: "%0", input: "bad", source: "test", session: "s"},
            {seq: 2, ts: "20260222120000", pane_id: "%0", input: "ok", source: "test", session: "s"},
        ] as unknown as InputHistoryEntry[];

        useInputHistoryStore.getState().setEntries(malformed);

        const state = useInputHistoryStore.getState();
        expect(state.entries).toHaveLength(1);
        expect(state.entries[0].seq).toBe(2);
    });

    it("sorts entries by seq ascending", () => {
        useInputHistoryStore.getState().setEntries([entry(3), entry(1), entry(2)]);

        const seqs = useInputHistoryStore.getState().entries.map((e) => e.seq);
        expect(seqs).toEqual([1, 2, 3]);
    });

    it("markAllRead sets unreadCount to 0", () => {
        useInputHistoryStore.getState().setEntries([entry(1), entry(2)]);
        useInputHistoryStore.getState().setEntries([entry(1), entry(2), entry(3)]);

        expect(useInputHistoryStore.getState().unreadCount).toBe(1);

        useInputHistoryStore.getState().markAllRead();

        expect(useInputHistoryStore.getState().unreadCount).toBe(0);
        expect(useInputHistoryStore.getState().lastReadSeq).toBe(3);
    });

    it("markAllRead with empty entries preserves lastReadSeq", () => {
        useInputHistoryStore.setState({lastReadSeq: 5});

        useInputHistoryStore.getState().markAllRead();

        expect(useInputHistoryStore.getState().lastReadSeq).toBe(5);
    });

    it("rejects entries with non-string fields", () => {
        const malformed = [
            {seq: 1, ts: "ts", pane_id: 42, input: "x", source: "s", session: "s"},
        ] as unknown as InputHistoryEntry[];

        useInputHistoryStore.getState().setEntries(malformed);

        expect(useInputHistoryStore.getState().entries).toHaveLength(0);
    });
});
