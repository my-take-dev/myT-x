import {describe, expect, it} from "vitest";
import {
    getPreExecFieldBounds,
    getTemplateFieldLimits,
    normalizeTaskSchedulerValidationRules,
    readNumberInput,
    resolveInitialPreExecIdleTimeoutWithRules,
    resolveInitialPreExecIdleTimeout,
    resolveInitialPreExecResetDelayWithRules,
    resolveInitialPreExecResetDelay,
    resolveInitialPreExecTargetMode,
} from "../src/components/viewer/views/task-scheduler/taskSchedulerConfigForm";
import {
    PRE_EXEC_TARGET_MODE_ALL_PANES,
    PRE_EXEC_TARGET_MODE_TASK_PANES,
} from "../src/components/viewer/views/task-scheduler/preExecTargetModes";

describe("resolveInitialPreExecTargetMode", () => {
    it("falls back to task panes when the backend config is empty", () => {
        expect(resolveInitialPreExecTargetMode(undefined)).toBe(PRE_EXEC_TARGET_MODE_TASK_PANES);
        expect(resolveInitialPreExecTargetMode(null)).toBe(PRE_EXEC_TARGET_MODE_TASK_PANES);
        expect(resolveInitialPreExecTargetMode("")).toBe(PRE_EXEC_TARGET_MODE_TASK_PANES);
        expect(resolveInitialPreExecTargetMode(PRE_EXEC_TARGET_MODE_ALL_PANES)).toBe(PRE_EXEC_TARGET_MODE_ALL_PANES);
    });
});

describe("resolveInitialPreExecResetDelay", () => {
    it("keeps valid values and restores backend defaults for invalid ones", () => {
        // Keep in sync with defaultTaskSchedulerSettings in useTaskScheduler.ts.
        expect(resolveInitialPreExecResetDelay(undefined)).toBe(0);
        expect(resolveInitialPreExecResetDelay(-1)).toBe(0);
        expect(resolveInitialPreExecResetDelay(0)).toBe(0);
        expect(resolveInitialPreExecResetDelay(60.4)).toBe(60);
        expect(resolveInitialPreExecResetDelay(999)).toBe(60);
    });
});

describe("resolveInitialPreExecIdleTimeout", () => {
    it("keeps valid values and restores backend defaults for non-positive ones", () => {
        // Keep in sync with defaultTaskSchedulerSettings in useTaskScheduler.ts.
        expect(resolveInitialPreExecIdleTimeout(undefined)).toBe(30);
        expect(resolveInitialPreExecIdleTimeout(0)).toBe(30);
        expect(resolveInitialPreExecIdleTimeout(-5)).toBe(30);
        expect(resolveInitialPreExecIdleTimeout(9.6)).toBe(10);
        expect(resolveInitialPreExecIdleTimeout(700)).toBe(600);
    });
});

describe("task scheduler fallback values with backend rules", () => {
    it("clamps the reset-delay fallback through stricter backend bounds", () => {
        expect(resolveInitialPreExecResetDelayWithRules(undefined, {
            min_pre_exec_reset_delay: 5,
            max_pre_exec_reset_delay: 9,
        })).toBe(5);
    });

    it("clamps the idle-timeout fallback through stricter backend bounds", () => {
        expect(resolveInitialPreExecIdleTimeoutWithRules(undefined, {
            min_pre_exec_idle_timeout: 45,
            max_pre_exec_idle_timeout: 99,
        })).toBe(45);
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

describe("task scheduler validation rule normalization", () => {
    it("falls back to compiled defaults when backend rules are missing", () => {
        expect(normalizeTaskSchedulerValidationRules(null)).toEqual({
            minPreExecResetDelay: 0,
            maxPreExecResetDelay: 60,
            minPreExecIdleTimeout: 10,
            maxPreExecIdleTimeout: 600,
            maxMessageTemplates: 50,
            maxTemplateNameLen: 100,
            maxTemplateMessageLen: 5000,
        });
    });

    it("uses backend-provided task scheduler limits when available", () => {
        const rules = {
            min_override_name_len: 5,
            min_pre_exec_reset_delay: 1,
            max_pre_exec_reset_delay: 9,
            min_pre_exec_idle_timeout: 20,
            max_pre_exec_idle_timeout: 99,
            max_message_templates: 7,
            max_template_name_len: 11,
            max_template_message_len: 222,
        };
        expect(getPreExecFieldBounds(rules)).toEqual({
            idleTimeout: {min: 20, max: 99},
            resetDelay: {min: 1, max: 9},
        });
        expect(getTemplateFieldLimits(rules)).toEqual({
            maxMessageTemplates: 7,
            maxTemplateNameLen: 11,
            maxTemplateMessageLen: 222,
        });
    });
});
