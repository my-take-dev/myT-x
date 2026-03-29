import {beforeEach, describe, expect, it} from "vitest";
import {useTmuxStore} from "../src/stores/tmuxStore";
import type {SessionSnapshot} from "../src/types/tmux";

/** Create a minimal SessionSnapshot for testing. */
function session(id: number, name: string, overrides: Partial<SessionSnapshot> = {}): SessionSnapshot {
    return {
        id,
        name,
        created_at: "2026-01-01T00:00:00Z",
        is_idle: false,
        active_window_id: id * 10,
        windows: [],
        ...overrides,
    };
}

describe("tmuxStore", () => {
    beforeEach(() => {
        useTmuxStore.setState({
            config: null,
            sessions: [],
            sessionOrder: [],
            activeSession: null,
            activeWindowId: null,
            zoomPaneId: null,
            pendingPrefixKillPaneId: null,
            prefixMode: false,
            syncInputMode: false,
            fontSize: 13,
            imeResetSignal: 0,
        });
    });

    // ── setSessions ──

    describe("setSessions", () => {
        it("sets sessions and builds order from empty state", () => {
            useTmuxStore.getState().setSessions([session(2, "b"), session(1, "a")]);

            const state = useTmuxStore.getState();
            expect(state.sessions.map((s) => s.name)).toEqual(["a", "b"]);
            expect(state.sessionOrder).toEqual(["a", "b"]);
        });

        it("activates first session when no previous active session", () => {
            useTmuxStore.getState().setSessions([session(2, "b"), session(1, "a")]);

            expect(useTmuxStore.getState().activeSession).toBe("a");
        });

        it("preserves active session when it still exists", () => {
            useTmuxStore.setState({activeSession: "b", sessionOrder: ["b", "a"]});

            useTmuxStore.getState().setSessions([session(1, "a"), session(2, "b")]);

            expect(useTmuxStore.getState().activeSession).toBe("b");
        });

        it("falls back to first session when active session is removed", () => {
            useTmuxStore.setState({activeSession: "removed", sessionOrder: ["removed"]});

            useTmuxStore.getState().setSessions([session(1, "x"), session(2, "y")]);

            expect(useTmuxStore.getState().activeSession).toBe("x");
        });

        it("sets activeSession to null when sessions are empty", () => {
            useTmuxStore.setState({activeSession: "old"});

            useTmuxStore.getState().setSessions([]);

            const state = useTmuxStore.getState();
            expect(state.activeSession).toBeNull();
            expect(state.activeWindowId).toBeNull();
        });

        it("resolves activeWindowId from active session snapshot", () => {
            useTmuxStore.getState().setSessions([session(1, "s1", {active_window_id: 42})]);

            expect(useTmuxStore.getState().activeWindowId).toBe("42");
        });

        it("resolves activeWindowId=null when active_window_id is undefined", () => {
            const s = session(1, "s1");
            // Simulate backend omitting the field via JSON omitempty
            delete (s as Record<string, unknown>).active_window_id;
            useTmuxStore.getState().setSessions([s]);

            expect(useTmuxStore.getState().activeWindowId).toBeNull();
        });

        it("preserves existing session order for known sessions", () => {
            useTmuxStore.setState({sessionOrder: ["c", "a", "b"]});

            useTmuxStore.getState().setSessions([
                session(1, "a"), session(2, "b"), session(3, "c"),
            ]);

            expect(useTmuxStore.getState().sessions.map((s) => s.name)).toEqual(["c", "a", "b"]);
        });

        it("appends new sessions after existing order sorted by id", () => {
            useTmuxStore.setState({sessionOrder: ["a"]});

            useTmuxStore.getState().setSessions([
                session(1, "a"), session(3, "c"), session(2, "b"),
            ]);

            expect(useTmuxStore.getState().sessionOrder).toEqual(["a", "b", "c"]);
        });

        it("prunes destroyed sessions from order", () => {
            useTmuxStore.setState({sessionOrder: ["a", "removed", "b"]});

            useTmuxStore.getState().setSessions([session(1, "a"), session(2, "b")]);

            expect(useTmuxStore.getState().sessionOrder).toEqual(["a", "b"]);
        });

        it("handles active_window_id=0 as valid window ID", () => {
            useTmuxStore.getState().setSessions([session(1, "s1", {active_window_id: 0})]);

            // 0 is a valid window ID, should NOT be null
            expect(useTmuxStore.getState().activeWindowId).toBe("0");
        });
    });

    // ── applySessionDelta ──

    describe("applySessionDelta", () => {
        it("adds new sessions via upserts", () => {
            useTmuxStore.getState().setSessions([session(1, "a")]);

            useTmuxStore.getState().applySessionDelta([session(2, "b")], []);

            const names = useTmuxStore.getState().sessions.map((s) => s.name);
            expect(names).toContain("a");
            expect(names).toContain("b");
        });

        it("replaces existing session via upsert", () => {
            useTmuxStore.getState().setSessions([session(1, "a", {is_idle: false})]);

            useTmuxStore.getState().applySessionDelta([session(1, "a", {is_idle: true})], []);

            expect(useTmuxStore.getState().sessions.find((s) => s.name === "a")?.is_idle).toBe(true);
        });

        it("removes sessions by name", () => {
            useTmuxStore.getState().setSessions([session(1, "a"), session(2, "b")]);

            useTmuxStore.getState().applySessionDelta([], ["a"]);

            expect(useTmuxStore.getState().sessions.map((s) => s.name)).toEqual(["b"]);
        });

        it("handles simultaneous upsert and remove", () => {
            useTmuxStore.getState().setSessions([session(1, "a"), session(2, "b")]);

            useTmuxStore.getState().applySessionDelta([session(3, "c")], ["a"]);

            const names = useTmuxStore.getState().sessions.map((s) => s.name);
            expect(names).toEqual(expect.arrayContaining(["b", "c"]));
            expect(names).not.toContain("a");
        });

        it("filters out undefined items in removed array", () => {
            useTmuxStore.getState().setSessions([session(1, "a"), session(2, "b")]);

            // Simulate malformed backend delta
            useTmuxStore.getState().applySessionDelta([], [undefined as unknown as string, "a"]);

            const names = useTmuxStore.getState().sessions.map((s) => s.name);
            expect(names).toEqual(["b"]);
        });

        it("filters out null/undefined items in upserts array", () => {
            useTmuxStore.getState().setSessions([session(1, "a")]);

            useTmuxStore.getState().applySessionDelta(
                [null as unknown as SessionSnapshot, session(2, "b")],
                [],
            );

            const names = useTmuxStore.getState().sessions.map((s) => s.name);
            expect(names).toEqual(expect.arrayContaining(["a", "b"]));
        });

        it("filters out upsert items with non-string name", () => {
            useTmuxStore.getState().setSessions([session(1, "a")]);

            const malformed = {id: 99, name: 123} as unknown as SessionSnapshot;
            useTmuxStore.getState().applySessionDelta([malformed, session(2, "b")], []);

            const names = useTmuxStore.getState().sessions.map((s) => s.name);
            expect(names).toContain("b");
            expect(names).not.toContain("123");
        });

        it("falls back to first session when active is removed", () => {
            useTmuxStore.getState().setSessions([session(1, "a"), session(2, "b")]);
            useTmuxStore.getState().setActiveSession("a");

            useTmuxStore.getState().applySessionDelta([], ["a"]);

            expect(useTmuxStore.getState().activeSession).toBe("b");
        });

        it("updates activeWindowId when active session is upserted", () => {
            useTmuxStore.getState().setSessions([session(1, "a", {active_window_id: 10})]);
            useTmuxStore.getState().setActiveSession("a");

            useTmuxStore.getState().applySessionDelta(
                [session(1, "a", {active_window_id: 20})],
                [],
            );

            expect(useTmuxStore.getState().activeWindowId).toBe("20");
        });
    });

    // ── setActiveSession ──

    describe("setActiveSession", () => {
        it("sets active session and resolves window ID", () => {
            useTmuxStore.getState().setSessions([
                session(1, "a", {active_window_id: 10}),
                session(2, "b", {active_window_id: 20}),
            ]);

            useTmuxStore.getState().setActiveSession("b");

            expect(useTmuxStore.getState().activeSession).toBe("b");
            expect(useTmuxStore.getState().activeWindowId).toBe("20");
        });

        it("sets null clears both activeSession and activeWindowId", () => {
            useTmuxStore.setState({activeSession: "a", activeWindowId: "10"});

            useTmuxStore.getState().setActiveSession(null);

            expect(useTmuxStore.getState().activeSession).toBeNull();
            expect(useTmuxStore.getState().activeWindowId).toBeNull();
        });

        it("resolves activeWindowId=null for non-existent session", () => {
            useTmuxStore.getState().setSessions([session(1, "a")]);

            useTmuxStore.getState().setActiveSession("nonexistent");

            expect(useTmuxStore.getState().activeSession).toBe("nonexistent");
            expect(useTmuxStore.getState().activeWindowId).toBeNull();
        });
    });

    // ── setActiveWindowId ──

    describe("setActiveWindowId", () => {
        const cases: { name: string; input: string | null; expected: string | null }[] = [
            {name: "normal window ID", input: "42", expected: "42"},
            {name: "null normalizes to null", input: null, expected: null},
            {name: "empty string normalizes to null", input: "", expected: null},
            {name: "whitespace-only normalizes to null", input: "   ", expected: null},
            {name: "non-string normalizes to null", input: 123 as unknown as string, expected: null},
        ];

        it.each(cases)("$name", ({input, expected}) => {
            useTmuxStore.getState().setActiveWindowId(input);
            expect(useTmuxStore.getState().activeWindowId).toBe(expected);
        });
    });

    // ── reorderSession ──

    describe("reorderSession", () => {
        beforeEach(() => {
            useTmuxStore.getState().setSessions([
                session(1, "a"), session(2, "b"), session(3, "c"),
            ]);
        });

        it("moves session forward in order", () => {
            useTmuxStore.getState().reorderSession(0, 2);

            expect(useTmuxStore.getState().sessionOrder).toEqual(["b", "c", "a"]);
            expect(useTmuxStore.getState().sessions.map((s) => s.name)).toEqual(["b", "c", "a"]);
        });

        it("moves session backward in order", () => {
            useTmuxStore.getState().reorderSession(2, 0);

            expect(useTmuxStore.getState().sessionOrder).toEqual(["c", "a", "b"]);
        });

        it("returns current state for negative fromIndex", () => {
            const before = useTmuxStore.getState().sessionOrder;

            useTmuxStore.getState().reorderSession(-1, 0);

            expect(useTmuxStore.getState().sessionOrder).toEqual(before);
        });

        it("returns current state for out-of-bounds fromIndex", () => {
            const before = useTmuxStore.getState().sessionOrder;

            useTmuxStore.getState().reorderSession(99, 0);

            expect(useTmuxStore.getState().sessionOrder).toEqual(before);
        });

        it("returns current state for negative toIndex", () => {
            const before = useTmuxStore.getState().sessionOrder;

            useTmuxStore.getState().reorderSession(0, -1);

            expect(useTmuxStore.getState().sessionOrder).toEqual(before);
        });

        it("returns current state for out-of-bounds toIndex", () => {
            const before = useTmuxStore.getState().sessionOrder;

            useTmuxStore.getState().reorderSession(0, 99);

            expect(useTmuxStore.getState().sessionOrder).toEqual(before);
        });

        it("no-op when fromIndex equals toIndex", () => {
            useTmuxStore.getState().reorderSession(1, 1);

            expect(useTmuxStore.getState().sessionOrder).toEqual(["a", "b", "c"]);
        });
    });

    // ── Simple setters ──

    describe("simple setters", () => {
        it("setConfig stores config", () => {
            const config = {shell: "bash"} as never;
            useTmuxStore.getState().setConfig(config);
            expect(useTmuxStore.getState().config).toBe(config);
        });

        it("setZoomPaneId stores value", () => {
            useTmuxStore.getState().setZoomPaneId("%42");
            expect(useTmuxStore.getState().zoomPaneId).toBe("%42");
        });

        it("setPendingPrefixKillPaneId stores value", () => {
            useTmuxStore.getState().setPendingPrefixKillPaneId("%1");
            expect(useTmuxStore.getState().pendingPrefixKillPaneId).toBe("%1");
        });

        it("setPrefixMode stores value", () => {
            useTmuxStore.getState().setPrefixMode(true);
            expect(useTmuxStore.getState().prefixMode).toBe(true);
        });

        it("setSyncInputMode stores value", () => {
            useTmuxStore.getState().setSyncInputMode(true);
            expect(useTmuxStore.getState().syncInputMode).toBe(true);
        });

        it("toggleSyncInputMode flips the value", () => {
            expect(useTmuxStore.getState().syncInputMode).toBe(false);
            useTmuxStore.getState().toggleSyncInputMode();
            expect(useTmuxStore.getState().syncInputMode).toBe(true);
            useTmuxStore.getState().toggleSyncInputMode();
            expect(useTmuxStore.getState().syncInputMode).toBe(false);
        });

        it("setFontSize stores value", () => {
            useTmuxStore.getState().setFontSize(18);
            expect(useTmuxStore.getState().fontSize).toBe(18);
        });
    });
});
