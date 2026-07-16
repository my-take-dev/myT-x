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

function setEntries(entries: InputHistoryEntry[]): void {
    useInputHistoryStore.getState().setSnapshot({scope_key: "", entries});
}

describe("inputHistoryStore", () => {
    beforeEach(() => {
        useInputHistoryStore.setState({
            scopeKey: "",
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
            readSeqByScope: {},
        });
    });

    it("treats initial snapshot as already read", () => {
        setEntries([entry(1), entry(2), entry(3)]);

        const state = useInputHistoryStore.getState();
        expect(state.unreadCount).toBe(0);
        expect(state.lastReadSeq).toBe(3);
    });

    it("counts new entries as unread after initial load", () => {
        setEntries([entry(1), entry(2)]);

        setEntries([entry(1), entry(2), entry(3), entry(4)]);

        expect(useInputHistoryStore.getState().unreadCount).toBe(2);
    });

    it("handles backend sequence restart without false unread count", () => {
        setEntries([entry(10)]);
        setEntries([entry(1), entry(2)]);

        let state = useInputHistoryStore.getState();
        expect(state.lastReadSeq).toBe(2);
        expect(state.unreadCount).toBe(0);

        setEntries([entry(1), entry(2), entry(3)]);
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

        setEntries(malformed);

        const state = useInputHistoryStore.getState();
        expect(state.entries).toHaveLength(1);
        expect(state.entries[0].seq).toBe(2);
    });

    it("sorts entries by seq ascending", () => {
        setEntries([entry(3), entry(1), entry(2)]);

        const seqs = useInputHistoryStore.getState().entries.map((e) => e.seq);
        expect(seqs).toEqual([1, 2, 3]);
    });

    it("markAllRead sets unreadCount to 0", () => {
        setEntries([entry(1), entry(2)]);
        setEntries([entry(1), entry(2), entry(3)]);

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

    it("counts appended entries as unread for the same scope key", () => {
        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2)]});

        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2), entry(3)]});

        const state = useInputHistoryStore.getState();
        expect(state.scopeKey).toBe("scope-a");
        expect(state.unreadCount).toBe(1);
        expect(state.lastReadSeq).toBe(2);
    });

    it("treats initial entries for a new scope key as already read", () => {
        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2)]});
        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2), entry(3)]});
        expect(useInputHistoryStore.getState().unreadCount).toBe(1);

        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-b", entries: [entry(1), entry(2, {input: "new scope"})]});

        const state = useInputHistoryStore.getState();
        expect(state.scopeKey).toBe("scope-b");
        expect(state.unreadCount).toBe(0);
        expect(state.lastReadSeq).toBe(2);
        expect(state.entries.map((e) => e.input)).toEqual(["input-1", "new scope"]);
    });

    it("preserves unread state when returning to a previous scope key", () => {
        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2)]});
        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2), entry(3)]});
        expect(useInputHistoryStore.getState().unreadCount).toBe(1);

        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-b", entries: [entry(1), entry(2)]});
        expect(useInputHistoryStore.getState().unreadCount).toBe(0);

        useInputHistoryStore.getState().setSnapshot({scope_key: "scope-a", entries: [entry(1), entry(2), entry(3)]});

        const state = useInputHistoryStore.getState();
        expect(state.scopeKey).toBe("scope-a");
        expect(state.unreadCount).toBe(1);
        expect(state.lastReadSeq).toBe(2);
    });

    it("rejects entries with non-string fields", () => {
        const malformed = [
            {seq: 1, ts: "ts", pane_id: 42, input: "x", source: "s", session: "s"},
        ] as unknown as InputHistoryEntry[];

        setEntries(malformed);

        expect(useInputHistoryStore.getState().entries).toHaveLength(0);
    });
});
