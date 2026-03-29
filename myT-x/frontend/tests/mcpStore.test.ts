import {beforeEach, describe, expect, it} from "vitest";
import {useMCPStore} from "../src/stores/mcpStore";
import type {MCPSnapshot} from "../src/types/mcp";

function snapshot(id: string, overrides: Partial<MCPSnapshot> = {}): MCPSnapshot {
    return {
        id,
        name: `mcp-${id}`,
        description: "",
        enabled: true,
        status: "running",
        ...overrides,
    };
}

describe("mcpStore", () => {
    beforeEach(() => {
        useMCPStore.setState({
            snapshots: {},
            sessionStates: {},
        });
    });

    // ── setSnapshots ──

    describe("setSnapshots", () => {
        it("stores snapshots for a session", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a"), snapshot("b")]);

            expect(useMCPStore.getState().snapshots["sess1"]).toHaveLength(2);
        });

        it("replaces existing snapshots for the same session", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);
            useMCPStore.getState().setSnapshots("sess1", [snapshot("b")]);

            const items = useMCPStore.getState().snapshots["sess1"];
            expect(items).toHaveLength(1);
            expect(items[0].id).toBe("b");
        });

        it("does not affect other sessions", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);
            useMCPStore.getState().setSnapshots("sess2", [snapshot("b")]);

            expect(useMCPStore.getState().snapshots["sess1"][0].id).toBe("a");
            expect(useMCPStore.getState().snapshots["sess2"][0].id).toBe("b");
        });
    });

    // ── upsertSnapshot ──

    describe("upsertSnapshot", () => {
        it("inserts new snapshot into empty session", () => {
            useMCPStore.getState().upsertSnapshot("sess1", snapshot("a"));

            expect(useMCPStore.getState().snapshots["sess1"]).toHaveLength(1);
            expect(useMCPStore.getState().snapshots["sess1"][0].id).toBe("a");
        });

        it("appends new snapshot to existing session", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);

            useMCPStore.getState().upsertSnapshot("sess1", snapshot("b"));

            expect(useMCPStore.getState().snapshots["sess1"]).toHaveLength(2);
        });

        it("replaces existing snapshot by id", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a", {status: "stopped"})]);

            useMCPStore.getState().upsertSnapshot("sess1", snapshot("a", {status: "running"}));

            const items = useMCPStore.getState().snapshots["sess1"];
            expect(items).toHaveLength(1);
            expect(items[0].status).toBe("running");
        });

        it("preserves order when replacing existing snapshot", () => {
            useMCPStore.getState().setSnapshots("sess1", [
                snapshot("a"), snapshot("b"), snapshot("c"),
            ]);

            useMCPStore.getState().upsertSnapshot("sess1", snapshot("b", {name: "updated-b"}));

            const ids = useMCPStore.getState().snapshots["sess1"].map((s) => s.id);
            expect(ids).toEqual(["a", "b", "c"]);
            expect(useMCPStore.getState().snapshots["sess1"][1].name).toBe("updated-b");
        });

        it("does not affect other sessions", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);
            useMCPStore.getState().setSnapshots("sess2", [snapshot("x")]);

            useMCPStore.getState().upsertSnapshot("sess1", snapshot("b"));

            expect(useMCPStore.getState().snapshots["sess2"]).toHaveLength(1);
        });
    });

    // ── Session load state ──

    describe("session load state", () => {
        it("beginSessionLoad sets loading=true, error=null", () => {
            useMCPStore.getState().beginSessionLoad("sess1");

            const loadState = useMCPStore.getState().sessionStates["sess1"];
            expect(loadState.loading).toBe(true);
            expect(loadState.error).toBeNull();
        });

        it("beginSessionLoad clears previous error", () => {
            useMCPStore.getState().setSessionError("sess1", "prev-error");

            useMCPStore.getState().beginSessionLoad("sess1");

            expect(useMCPStore.getState().sessionStates["sess1"].error).toBeNull();
        });

        it("setSessionLoading updates loading flag", () => {
            useMCPStore.getState().beginSessionLoad("sess1");
            useMCPStore.getState().setSessionLoading("sess1", false);

            expect(useMCPStore.getState().sessionStates["sess1"].loading).toBe(false);
        });

        it("setSessionError updates error message", () => {
            useMCPStore.getState().setSessionError("sess1", "connection failed");

            expect(useMCPStore.getState().sessionStates["sess1"].error).toBe("connection failed");
        });

        it("setSessionError with null clears error", () => {
            useMCPStore.getState().setSessionError("sess1", "old error");
            useMCPStore.getState().setSessionError("sess1", null);

            expect(useMCPStore.getState().sessionStates["sess1"].error).toBeNull();
        });

        it("session state defaults to loading=false, error=null for unknown session", () => {
            useMCPStore.getState().setSessionLoading("new-sess", true);

            const loadState = useMCPStore.getState().sessionStates["new-sess"];
            expect(loadState.loading).toBe(true);
            expect(loadState.error).toBeNull();
        });
    });

    // ── clearSession ──

    describe("clearSession", () => {
        it("removes snapshots and session state for the target session", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);
            useMCPStore.getState().beginSessionLoad("sess1");

            useMCPStore.getState().clearSession("sess1");

            expect(useMCPStore.getState().snapshots["sess1"]).toBeUndefined();
            expect(useMCPStore.getState().sessionStates["sess1"]).toBeUndefined();
        });

        it("does not affect other sessions", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);
            useMCPStore.getState().setSnapshots("sess2", [snapshot("b")]);

            useMCPStore.getState().clearSession("sess1");

            expect(useMCPStore.getState().snapshots["sess2"]).toHaveLength(1);
        });

        it("is a no-op for non-existent session", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);

            useMCPStore.getState().clearSession("nonexistent");

            expect(useMCPStore.getState().snapshots["sess1"]).toHaveLength(1);
        });
    });

    // ── clearAllSessions ──

    describe("clearAllSessions", () => {
        it("removes all snapshots and session states", () => {
            useMCPStore.getState().setSnapshots("sess1", [snapshot("a")]);
            useMCPStore.getState().setSnapshots("sess2", [snapshot("b")]);
            useMCPStore.getState().beginSessionLoad("sess1");

            useMCPStore.getState().clearAllSessions();

            expect(useMCPStore.getState().snapshots).toEqual({});
            expect(useMCPStore.getState().sessionStates).toEqual({});
        });

        it("is a no-op when already empty", () => {
            useMCPStore.getState().clearAllSessions();

            expect(useMCPStore.getState().snapshots).toEqual({});
            expect(useMCPStore.getState().sessionStates).toEqual({});
        });
    });
});
