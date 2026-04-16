import {describe, expect, it} from "vitest";
import {
    isQueueItem,
    isQueueStatus,
    isActiveQueueStatus,
    isEditableStatus,
} from "../src/components/viewer/views/task-scheduler/useTaskScheduler";

describe("isEditableStatus", () => {
    it("allows only statuses that the backend accepts", () => {
        expect(isEditableStatus(undefined)).toBe(false);
        expect(isEditableStatus(null)).toBe(false);
        expect(isEditableStatus("")).toBe(false);
        expect(isEditableStatus("pending")).toBe(true);
        expect(isEditableStatus("completed")).toBe(true);
        expect(isEditableStatus("failed")).toBe(true);
        expect(isEditableStatus("skipped")).toBe(true);

        expect(isEditableStatus("running")).toBe(false);
        expect(isEditableStatus("queued")).toBe(false);
        expect(isEditableStatus("cancelled")).toBe(false);
    });
});

describe("isActiveQueueStatus", () => {
    it("treats preparing as an active queue state", () => {
        expect(isActiveQueueStatus(undefined)).toBe(false);
        expect(isActiveQueueStatus(null)).toBe(false);
        expect(isActiveQueueStatus("running")).toBe(true);
        expect(isActiveQueueStatus("paused")).toBe(true);
        expect(isActiveQueueStatus("preparing")).toBe(true);
        expect(isActiveQueueStatus("completed")).toBe(false);
        expect(isActiveQueueStatus("idle")).toBe(false);
    });
});

describe("task-scheduler queue validators", () => {
    it("rejects malformed queue items", () => {
        expect(isQueueItem(null)).toBe(false);
        expect(isQueueItem(undefined)).toBe(false);
        expect(isQueueItem({
            id: "item-1",
            title: "Title",
            message: "Message",
            target_pane_id: "%1",
            order_index: 0,
            status: "",
            created_at: "2026-04-12T00:00:00Z",
            clear_before: false,
            clear_command: "",
        })).toBe(false);
        expect(isQueueItem({
            id: "item-1",
            title: "Title",
            message: "Message",
            target_pane_id: "%1",
            order_index: "0",
            status: "pending",
            created_at: "2026-04-12T00:00:00Z",
            clear_before: false,
            clear_command: "",
        })).toBe(false);
    });

    it("rejects malformed queue payloads", () => {
        expect(isQueueStatus(null)).toBe(false);
        expect(isQueueStatus(undefined)).toBe(false);
        expect(isQueueStatus({
            run_status: "idle",
            current_index: 0,
            session_name: "session-a",
            generation_id: "gen-1",
            items: [{
                id: "item-1",
                title: "Title",
                message: "Message",
                target_pane_id: "%1",
                order_index: 0,
                status: "",
                created_at: "2026-04-12T00:00:00Z",
                clear_before: false,
                clear_command: "",
            }],
        })).toBe(false);
    });
});
