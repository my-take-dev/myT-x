import {describe, expect, it} from "vitest";
import {
    readNumberInput,
    resolveInitialPreExecIdleTimeout,
    resolveInitialPreExecResetDelay,
    resolveInitialPreExecTargetMode,
} from "../src/components/viewer/views/task-scheduler/taskSchedulerConfigForm";

describe("resolveInitialPreExecTargetMode", () => {
    it("falls back to task panes when the backend config is empty", () => {
        expect(resolveInitialPreExecTargetMode(undefined)).toBe("task_panes");
        expect(resolveInitialPreExecTargetMode(null)).toBe("task_panes");
        expect(resolveInitialPreExecTargetMode("")).toBe("task_panes");
        expect(resolveInitialPreExecTargetMode("all_panes")).toBe("all_panes");
    });
});

describe("resolveInitialPreExecResetDelay", () => {
    it("keeps valid values and restores backend defaults for invalid ones", () => {
        expect(resolveInitialPreExecResetDelay(undefined)).toBe(10);
        expect(resolveInitialPreExecResetDelay(-1)).toBe(10);
        expect(resolveInitialPreExecResetDelay(0)).toBe(0);
        expect(resolveInitialPreExecResetDelay(60.4)).toBe(60);
        expect(resolveInitialPreExecResetDelay(999)).toBe(60);
    });
});

describe("resolveInitialPreExecIdleTimeout", () => {
    it("keeps valid values and restores backend defaults for non-positive ones", () => {
        expect(resolveInitialPreExecIdleTimeout(undefined)).toBe(120);
        expect(resolveInitialPreExecIdleTimeout(0)).toBe(120);
        expect(resolveInitialPreExecIdleTimeout(-5)).toBe(120);
        expect(resolveInitialPreExecIdleTimeout(9.6)).toBe(10);
        expect(resolveInitialPreExecIdleTimeout(700)).toBe(600);
    });
});

describe("readNumberInput", () => {
    it("preserves the previous value for blank or invalid input", () => {
        expect(readNumberInput("", 15, 0, 60)).toBe(15);
        expect(readNumberInput("abc", 15, 0, 60)).toBe(15);
    });

    it("rounds floating-point values and clamps to the configured range", () => {
        expect(readNumberInput("10.5", 15, 0, 60)).toBe(11);
        expect(readNumberInput("-5", 15, 0, 60)).toBe(0);
        expect(readNumberInput("601", 120, 10, 600)).toBe(600);
    });
});
