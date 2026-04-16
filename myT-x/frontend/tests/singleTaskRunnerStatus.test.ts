import {describe, expect, it} from "vitest";
import {
    isQueueItem,
    isQueueStatus,
    isActiveQueueStatus,
    isEditableStatus,
} from "../src/components/viewer/views/single-task-runner/useSingleTaskRunner";

describe("single-task-runner isEditableStatus", () => {
    it("matches the backend editable states and rejects malformed payload values", () => {
        expect(isEditableStatus(undefined)).toBe(false);
        expect(isEditableStatus(null)).toBe(false);
        expect(isEditableStatus("")).toBe(false);
        expect(isEditableStatus("pending")).toBe(true);
        expect(isEditableStatus("done")).toBe(true);
        expect(isEditableStatus("failed")).toBe(true);
        expect(isEditableStatus("cancelled")).toBe(true);

        // Runtime execution states stay non-editable in sync with backend QueueItemStatus.
        expect(isEditableStatus("sending")).toBe(false);
        expect(isEditableStatus("active")).toBe(false);
    });
});

describe("single-task-runner isActiveQueueStatus", () => {
    it("treats only running as active", () => {
        expect(isActiveQueueStatus(undefined)).toBe(false);
        expect(isActiveQueueStatus(null)).toBe(false);
        expect(isActiveQueueStatus("running")).toBe(true);
        expect(isActiveQueueStatus("idle")).toBe(false);
        expect(isActiveQueueStatus("completed")).toBe(false);
    });
});

describe("single-task-runner queue validators", () => {
    it("rejects malformed queue items", () => {
        expect(isQueueItem(null)).toBe(false);
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
        expect(isQueueStatus(undefined)).toBe(false);
        expect(isQueueStatus({
            run_status: "idle",
            current_index: 0,
            session_name: "session-a",
            generation_id: "gen-1",
            clear_delay_sec: 2,
            last_stop_reason: "",
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
