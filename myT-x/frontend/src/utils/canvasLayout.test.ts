import {describe, expect, it} from "vitest";
import {computeTreeLayout} from "./canvasLayout";
import type {OrchestratorTask} from "../types/canvas";

function makeTask(overrides: Partial<OrchestratorTask> = {}): OrchestratorTask {
    return {
        task_id: "t-001",
        agent_name: "agent-a",
        sender_pane_id: "%0",
        assignee_pane_id: "%1",
        sender_name: "sender-a",
        status: "completed",
        sent_at: "2026-01-01T00:00:00Z",
        completed_at: "2026-01-01T00:01:00Z",
        ...overrides,
    };
}

describe("computeTreeLayout", () => {
    it("returns empty object for empty pane list", () => {
        expect(computeTreeLayout([], [])).toEqual({});
    });

    it("assigns positions to a single pane with no tasks", () => {
        const result = computeTreeLayout(["%0"], []);
        expect(result).toHaveProperty("%0");
        expect(result["%0"]).toHaveProperty("x");
        expect(result["%0"]).toHaveProperty("y");
    });

    it("places two connected panes in a tree (sender above assignee)", () => {
        const tasks = [makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"})];
        const result = computeTreeLayout(["%0", "%1"], tasks);

        // sender (root) should be at level 0, assignee at level 1
        expect(result["%0"].y).toBeLessThan(result["%1"].y);
    });

    it("handles a linear chain: A → B → C", () => {
        const tasks = [
            makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-2", sender_pane_id: "%1", assignee_pane_id: "%2"}),
        ];
        const result = computeTreeLayout(["%0", "%1", "%2"], tasks);

        expect(result["%0"].y).toBeLessThan(result["%1"].y);
        expect(result["%1"].y).toBeLessThan(result["%2"].y);
    });

    it("handles branching tree: A → B, A → C", () => {
        const tasks = [
            makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-2", sender_pane_id: "%0", assignee_pane_id: "%2"}),
        ];
        const result = computeTreeLayout(["%0", "%1", "%2"], tasks);

        // root at top
        expect(result["%0"].y).toBeLessThan(result["%1"].y);
        expect(result["%0"].y).toBeLessThan(result["%2"].y);
        // children at same level
        expect(result["%1"].y).toBe(result["%2"].y);
    });

    it("places orphan panes (no task connections) separately", () => {
        const tasks = [makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"})];
        const result = computeTreeLayout(["%0", "%1", "%2"], tasks);

        expect(result).toHaveProperty("%2");
        // orphan should not be at tree level positions
        expect(result["%2"].x).not.toBe(result["%0"].x);
    });

    it("handles all orphans (no tasks)", () => {
        const result = computeTreeLayout(["%0", "%1", "%2"], []);
        expect(Object.keys(result)).toHaveLength(3);
        // all panes should have unique positions
        const positions = Object.values(result);
        const unique = new Set(positions.map((p) => `${p.x},${p.y}`));
        expect(unique.size).toBe(3);
    });

    it("filters out self-referential tasks (sender === assignee)", () => {
        const tasks = [makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%0"})];
        const result = computeTreeLayout(["%0"], tasks);
        expect(result).toHaveProperty("%0");
    });

    it("filters out tasks referencing panes not in the list", () => {
        const tasks = [makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%99"})];
        const result = computeTreeLayout(["%0"], tasks);
        expect(result).toHaveProperty("%0");
        expect(result).not.toHaveProperty("%99");
    });

    it("handles cyclic task graphs without infinite loop", () => {
        const tasks = [
            makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-2", sender_pane_id: "%1", assignee_pane_id: "%0"}),
        ];
        // Should not hang; both panes should be placed
        const result = computeTreeLayout(["%0", "%1"], tasks);
        expect(Object.keys(result)).toHaveLength(2);
    });

    it("deduplicates edges (same sender → same assignee via multiple tasks)", () => {
        const tasks = [
            makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-2", sender_pane_id: "%0", assignee_pane_id: "%1"}),
        ];
        const result = computeTreeLayout(["%0", "%1"], tasks);
        // Should still form a simple tree
        expect(result["%0"].y).toBeLessThan(result["%1"].y);
    });

    it("handles tasks with empty pane IDs", () => {
        const tasks = [makeTask({task_id: "t-1", sender_pane_id: "", assignee_pane_id: "%1"})];
        const result = computeTreeLayout(["%0", "%1"], tasks);
        expect(Object.keys(result)).toHaveLength(2);
    });
});
