import {describe, expect, it} from "vitest";
import {
    isActiveQueueStatus,
    isEditableStatus,
} from "../src/components/viewer/views/task-scheduler/useTaskScheduler";

describe("isEditableStatus", () => {
    it("allows only statuses that the backend accepts plus new items", () => {
        expect(isEditableStatus(undefined)).toBe(true);
        expect(isEditableStatus(null)).toBe(true);
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
