import {describe, expect, it} from "vitest";
import {resolveActivePane, resolveActivePaneID, resolveActiveWindow} from "../src/utils/session";
import type {PaneSnapshot, SessionSnapshot, WindowSnapshot} from "../src/types/tmux";

function pane(id: string, active = false): PaneSnapshot {
    return {id, title: id, active, index: 0, width: 100, height: 30};
}

function win(id: string, panes: PaneSnapshot[]): WindowSnapshot {
    return {id: Number(id.replace(/\D/g, "") || "0"), name: id, active_pane: 0, panes};
}

function session(windows: WindowSnapshot[], activeWindowID: number): SessionSnapshot {
    return {
        id: 1,
        name: "s",
        created_at: "2026-03-01T00:00:00Z",
        is_idle: false,
        active_window_id: activeWindowID,
        windows,
    };
}

describe("session utils", () => {
    it("resolveActiveWindow prioritizes active_window_id", () => {
        const result = resolveActiveWindow(session([
            win("w1", [pane("p1")]),
            win("w2", [pane("p2", true)]),
        ], 1));
        expect(result?.id).toBe(1);
    });

    it("resolveActiveWindow falls back to window containing active pane", () => {
        const result = resolveActiveWindow(session([
            win("w1", [pane("p1")]),
            win("w2", [pane("p2", true)]),
        ], 999));
        expect(result?.id).toBe(2);
    });

    it("resolveActiveWindow falls back to first window when no active markers exist", () => {
        const result = resolveActiveWindow(session([
            win("w1", [pane("p1")]),
            win("w2", [pane("p2")]),
        ], 999));
        expect(result?.id).toBe(1);
    });

    it("resolveActivePane falls back to first pane", () => {
        const result = resolveActivePane(win("w", [pane("p1"), pane("p2")]));
        expect(result?.id).toBe("p1");
    });

    it("resolveActivePaneID returns null safely for missing state", () => {
        expect(resolveActivePaneID(null)).toBeNull();
        expect(resolveActivePaneID(undefined)).toBeNull();
        expect(resolveActivePaneID(session([], 999))).toBeNull();
    });
});
