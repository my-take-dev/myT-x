import {describe, expect, it} from "vitest";
import {computeTreeLayout} from "./canvasLayout";
import type {CanvasNodeSize, OrchestratorTask} from "../types/canvas";

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
        message_preview: "",
        response_preview: "",
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

    describe("nodeSizes support", () => {
        const H_GAP = 80;
        const V_GAP = 60;
        const DEFAULT_W = 450;
        const DEFAULT_H = 350;

        it("uses actual node widths for X spacing — no overlap", () => {
            const tasks = [
                makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
                makeTask({task_id: "t-2", sender_pane_id: "%0", assignee_pane_id: "%2"}),
            ];
            const sizes: Record<string, CanvasNodeSize> = {
                "%1": {width: 800, height: DEFAULT_H},
                "%2": {width: 600, height: DEFAULT_H},
            };
            const result = computeTreeLayout(["%0", "%1", "%2"], tasks, sizes);

            // %1 と %2 は同一レベル。%2 の左端は %1 の右端 + H_GAP 以上
            expect(result["%2"].x).toBeGreaterThanOrEqual(result["%1"].x + 800 + H_GAP);
        });

        it("uses actual node heights for Y level spacing", () => {
            const tasks = [
                makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
                makeTask({task_id: "t-2", sender_pane_id: "%1", assignee_pane_id: "%2"}),
            ];
            const sizes: Record<string, CanvasNodeSize> = {
                "%1": {width: DEFAULT_W, height: 700},
            };
            const result = computeTreeLayout(["%0", "%1", "%2"], tasks, sizes);

            // %2 の Y は %1 の Y + 700 (高さ) + V_GAP 以上
            expect(result["%2"].y).toBeGreaterThanOrEqual(result["%1"].y + 700 + V_GAP);
        });

        it("falls back to default size for nodes without size entry", () => {
            const tasks = [
                makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
                makeTask({task_id: "t-2", sender_pane_id: "%0", assignee_pane_id: "%2"}),
            ];
            const sizes: Record<string, CanvasNodeSize> = {
                "%1": {width: 600, height: DEFAULT_H},
            };
            const result = computeTreeLayout(["%0", "%1", "%2"], tasks, sizes);

            // %0 (root) はデフォルトサイズ。%1 の Y は root の Y + DEFAULT_H + V_GAP
            expect(result["%1"].y).toBe(result["%0"].y + DEFAULT_H + V_GAP);
            // %2 は幅デフォルト。%2 の左端は %1 の右端 + H_GAP 以上
            expect(result["%2"].x).toBeGreaterThanOrEqual(result["%1"].x + 600 + H_GAP);
        });

        it("orphan placement uses actual sizes — no overlap", () => {
            // no tasks → all orphans
            const sizes: Record<string, CanvasNodeSize> = {
                "%0": {width: 800, height: 500},
                "%1": {width: 600, height: 400},
                "%2": {width: 700, height: 300},
            };
            const result = computeTreeLayout(["%0", "%1", "%2"], [], sizes);

            // 同一行内: 各ノードの右端が次のノードの左端より左
            const ids = ["%0", "%1", "%2"];
            for (let i = 0; i < ids.length - 1; i++) {
                const rightEdge = result[ids[i]].x + (sizes[ids[i]]?.width ?? DEFAULT_W);
                expect(result[ids[i + 1]].x).toBeGreaterThanOrEqual(rightEdge + H_GAP);
            }
        });

        it("wide node does not overlap siblings in the same level", () => {
            const tasks = [
                makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"}),
                makeTask({task_id: "t-2", sender_pane_id: "%0", assignee_pane_id: "%2"}),
            ];
            const sizes: Record<string, CanvasNodeSize> = {
                "%1": {width: 900, height: DEFAULT_H},
            };
            const result = computeTreeLayout(["%0", "%1", "%2"], tasks, sizes);

            // %2 の左端 >= %1 の右端 + H_GAP
            expect(result["%2"].x).toBeGreaterThanOrEqual(result["%1"].x + 900 + H_GAP);
        });

        it("empty nodeSizes behaves like no argument", () => {
            const tasks = [makeTask({task_id: "t-1", sender_pane_id: "%0", assignee_pane_id: "%1"})];
            const withEmpty = computeTreeLayout(["%0", "%1"], tasks, {});
            const withoutArg = computeTreeLayout(["%0", "%1"], tasks);
            expect(withEmpty).toEqual(withoutArg);
        });

        it("single node with custom size gets positioned", () => {
            const sizes: Record<string, CanvasNodeSize> = {
                "%0": {width: 600, height: 500},
            };
            const result = computeTreeLayout(["%0"], [], sizes);
            expect(result).toHaveProperty("%0");
            expect(result["%0"]).toHaveProperty("x");
            expect(result["%0"]).toHaveProperty("y");
        });
    });
});
