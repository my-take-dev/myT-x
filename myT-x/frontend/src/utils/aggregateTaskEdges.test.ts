import {describe, expect, it} from "vitest";
import {aggregateTaskEdges} from "./aggregateTaskEdges";
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

const allPanes = new Set(["%0", "%1", "%2", "%3"]);

describe("aggregateTaskEdges", () => {
    it("returns empty array for empty tasks", () => {
        expect(aggregateTaskEdges([], allPanes)).toEqual([]);
    });

    it("returns one edge for a single task", () => {
        const tasks = [makeTask()];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(1);
        expect(result[0].paneA).toBe("%0");
        expect(result[0].paneB).toBe("%1");
        expect(result[0].totalCount).toBe(1);
        expect(result[0].aggregateStatus).toBe("completed");
    });

    it("merges two tasks in same direction into one edge", () => {
        const tasks = [
            makeTask({task_id: "t-001", sent_at: "2026-01-01T00:00:00Z"}),
            makeTask({task_id: "t-002", sent_at: "2026-01-01T00:01:00Z"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(1);
        expect(result[0].totalCount).toBe(2);
        expect(result[0].forwardTasks).toHaveLength(2);
        expect(result[0].reverseTasks).toHaveLength(0);
    });

    it("merges bidirectional tasks (A→B and B→A) into one edge", () => {
        const tasks = [
            makeTask({task_id: "t-001", sender_pane_id: "%0", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-002", sender_pane_id: "%1", assignee_pane_id: "%0", sender_name: "sender-b"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(1);
        expect(result[0].totalCount).toBe(2);
        expect(result[0].forwardTasks).toHaveLength(1);
        expect(result[0].reverseTasks).toHaveLength(1);
    });

    it("uses latest task status when no pending (latest-task-priority)", () => {
        const tasks = [
            makeTask({task_id: "t-001", status: "failed", sent_at: "2026-01-01T00:00:00Z"}),
            makeTask({task_id: "t-002", status: "completed", sent_at: "2026-01-01T00:02:00Z"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(1);
        // 最新タスク(t-002)がcompletedなので、古いfailedではなくcompletedが採用される
        expect(result[0].aggregateStatus).toBe("completed");
    });

    it("pending always takes priority regardless of task order", () => {
        const tasks = [
            makeTask({task_id: "t-001", status: "completed", sent_at: "2026-01-01T00:02:00Z"}),
            makeTask({task_id: "t-002", status: "pending", sent_at: "2026-01-01T00:01:00Z"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(1);
        expect(result[0].aggregateStatus).toBe("pending");
        expect(result[0].pendingCount).toBe(1);
        expect(result[0].completedCount).toBe(1);
    });

    it("filters out tasks with invalid pane IDs", () => {
        const tasks = [
            makeTask({task_id: "t-001", sender_pane_id: "%0", assignee_pane_id: "%99"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(0);
    });

    it("filters out self-referencing tasks", () => {
        const tasks = [
            makeTask({task_id: "t-001", sender_pane_id: "%0", assignee_pane_id: "%0"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(0);
    });

    it("filters out tasks with empty pane IDs", () => {
        const tasks = [
            makeTask({task_id: "t-001", sender_pane_id: "", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-002", sender_pane_id: "%0", assignee_pane_id: ""}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(0);
    });

    it("creates separate edges for different terminal pairs", () => {
        const tasks = [
            makeTask({task_id: "t-001", sender_pane_id: "%0", assignee_pane_id: "%1"}),
            makeTask({task_id: "t-002", sender_pane_id: "%0", assignee_pane_id: "%2"}),
            makeTask({task_id: "t-003", sender_pane_id: "%1", assignee_pane_id: "%2"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result).toHaveLength(3);
    });

    it("sorts forward and reverse tasks by sent_at descending", () => {
        const tasks = [
            makeTask({task_id: "t-001", sent_at: "2026-01-01T00:00:00Z"}),
            makeTask({task_id: "t-002", sent_at: "2026-01-01T00:03:00Z"}),
            makeTask({task_id: "t-003", sent_at: "2026-01-01T00:01:00Z"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result[0].forwardTasks[0].sentAt).toBe("2026-01-01T00:03:00Z");
        expect(result[0].forwardTasks[1].sentAt).toBe("2026-01-01T00:01:00Z");
        expect(result[0].forwardTasks[2].sentAt).toBe("2026-01-01T00:00:00Z");
    });

    it("counts statuses correctly with mixed tasks", () => {
        const tasks = [
            makeTask({task_id: "t-001", status: "pending", sent_at: "2026-01-01T00:00:00Z"}),
            makeTask({task_id: "t-002", status: "completed", sent_at: "2026-01-01T00:01:00Z"}),
            makeTask({task_id: "t-003", status: "failed", sent_at: "2026-01-01T00:02:00Z"}),
            makeTask({task_id: "t-004", status: "abandoned", sent_at: "2026-01-01T00:03:00Z"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result[0].pendingCount).toBe(1);
        expect(result[0].completedCount).toBe(1);
        expect(result[0].failedCount).toBe(1);
        expect(result[0].abandonedCount).toBe(1);
        // pending があるので aggregateStatus は pending
        expect(result[0].aggregateStatus).toBe("pending");
    });

    it("paneA is always the lexicographically smaller ID", () => {
        // sender_pane_id > assignee_pane_id のケース
        const tasks = [
            makeTask({task_id: "t-001", sender_pane_id: "%1", assignee_pane_id: "%0"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result[0].paneA).toBe("%0");
        expect(result[0].paneB).toBe("%1");
        // %1→%0 は reverse 方向（paneB→paneA）
        expect(result[0].reverseTasks).toHaveLength(1);
        expect(result[0].forwardTasks).toHaveLength(0);
    });

    it("handles unknown status as abandoned", () => {
        const tasks = [
            makeTask({task_id: "t-001", status: "unknown_status", sent_at: "2026-01-01T00:00:00Z"}),
        ];
        const result = aggregateTaskEdges(tasks, allPanes);
        expect(result[0].aggregateStatus).toBe("abandoned");
        expect(result[0].abandonedCount).toBe(1);
    });
});
