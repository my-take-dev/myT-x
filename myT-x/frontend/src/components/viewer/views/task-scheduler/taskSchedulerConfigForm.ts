import type React from "react";
import type {QueueConfig} from "./useTaskScheduler";
import type {ValidationRules} from "../../../../types/tmux";
import {
    PRE_EXEC_TARGET_MODE_ALL_PANES,
    PRE_EXEC_TARGET_MODE_TASK_PANES,
} from "./preExecTargetModes";

const numericKeyAllowList = new Set([
    "Backspace", "Delete", "Tab", "Escape", "Enter",
    "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown",
    "Home", "End",
]);

export function blockNonNumericKeys(event: React.KeyboardEvent<HTMLInputElement>): void {
    if (event.ctrlKey || event.metaKey || event.altKey) {
        return;
    }
    if (numericKeyAllowList.has(event.key)) {
        return;
    }
    if (event.key >= "0" && event.key <= "9") {
        return;
    }
    event.preventDefault();
}

// Keep frontend fallbacks aligned with GetTaskSchedulerSettings() so
// pre-rendered UI never drifts from the backend contract.
const defaultPreExecResetDelay = 0;
const defaultPreExecIdleTimeout = 30;

export interface TaskSchedulerValidationRules {
    readonly minPreExecResetDelay: number;
    readonly maxPreExecResetDelay: number;
    readonly minPreExecIdleTimeout: number;
    readonly maxPreExecIdleTimeout: number;
    readonly maxMessageTemplates: number;
    readonly maxTemplateNameLen: number;
    readonly maxTemplateMessageLen: number;
}

const taskSchedulerValidationFallbacks: TaskSchedulerValidationRules = {
    minPreExecResetDelay: 0,
    maxPreExecResetDelay: 60,
    minPreExecIdleTimeout: 10,
    maxPreExecIdleTimeout: 600,
    maxMessageTemplates: 50,
    maxTemplateNameLen: 100,
    maxTemplateMessageLen: 5000,
};

function normalizePositiveInteger(value: unknown, fallback: number): number {
    return typeof value === "number" && Number.isFinite(value) && value >= 0
        ? Math.trunc(value)
        : fallback;
}

export function normalizeTaskSchedulerValidationRules(
    rules?: ValidationRules | null,
): TaskSchedulerValidationRules {
    return {
        minPreExecResetDelay: normalizePositiveInteger(
            rules?.min_pre_exec_reset_delay,
            taskSchedulerValidationFallbacks.minPreExecResetDelay,
        ),
        maxPreExecResetDelay: normalizePositiveInteger(
            rules?.max_pre_exec_reset_delay,
            taskSchedulerValidationFallbacks.maxPreExecResetDelay,
        ),
        minPreExecIdleTimeout: normalizePositiveInteger(
            rules?.min_pre_exec_idle_timeout,
            taskSchedulerValidationFallbacks.minPreExecIdleTimeout,
        ),
        maxPreExecIdleTimeout: normalizePositiveInteger(
            rules?.max_pre_exec_idle_timeout,
            taskSchedulerValidationFallbacks.maxPreExecIdleTimeout,
        ),
        maxMessageTemplates: normalizePositiveInteger(
            rules?.max_message_templates,
            taskSchedulerValidationFallbacks.maxMessageTemplates,
        ),
        maxTemplateNameLen: normalizePositiveInteger(
            rules?.max_template_name_len,
            taskSchedulerValidationFallbacks.maxTemplateNameLen,
        ),
        maxTemplateMessageLen: normalizePositiveInteger(
            rules?.max_template_message_len,
            taskSchedulerValidationFallbacks.maxTemplateMessageLen,
        ),
    };
}

export function getPreExecFieldBounds(rules?: ValidationRules | null) {
    const normalized = normalizeTaskSchedulerValidationRules(rules);
    return {
        idleTimeout: {
            max: normalized.maxPreExecIdleTimeout,
            min: normalized.minPreExecIdleTimeout,
        },
        resetDelay: {
            max: normalized.maxPreExecResetDelay,
            min: normalized.minPreExecResetDelay,
        },
    } as const;
}

export function getTemplateFieldLimits(rules?: ValidationRules | null) {
    const normalized = normalizeTaskSchedulerValidationRules(rules);
    return {
        maxMessageTemplates: normalized.maxMessageTemplates,
        maxTemplateNameLen: normalized.maxTemplateNameLen,
        maxTemplateMessageLen: normalized.maxTemplateMessageLen,
    } as const;
}

function clampInteger(value: number, min: number, max: number): number {
    if (value < min) {
        return min;
    }
    if (value > max) {
        return max;
    }
    return value;
}

function roundFiniteNumber(value: number): number | null {
    if (!Number.isFinite(value)) {
        return null;
    }
    return Math.round(value);
}

export function resolveInitialPreExecTargetMode(
    value: QueueConfig["pre_exec_target_mode"] | "" | null | undefined,
): QueueConfig["pre_exec_target_mode"] {
    return value === PRE_EXEC_TARGET_MODE_ALL_PANES
        ? PRE_EXEC_TARGET_MODE_ALL_PANES
        : PRE_EXEC_TARGET_MODE_TASK_PANES;
}

export function resolveInitialPreExecResetDelay(value: number | null | undefined): number {
    return resolveInitialPreExecResetDelayWithRules(value);
}

export function resolveInitialPreExecResetDelayWithRules(
    value: number | null | undefined,
    rules?: ValidationRules | null,
): number {
    const bounds = getPreExecFieldBounds(rules);
    const roundedValue = roundFiniteNumber(value ?? Number.NaN);
    const resolvedValue = roundedValue === null || roundedValue < 0
        ? defaultPreExecResetDelay
        : roundedValue;
    return clampInteger(resolvedValue, bounds.resetDelay.min, bounds.resetDelay.max);
}

export function resolveInitialPreExecIdleTimeout(value: number | null | undefined): number {
    return resolveInitialPreExecIdleTimeoutWithRules(value);
}

export function resolveInitialPreExecIdleTimeoutWithRules(
    value: number | null | undefined,
    rules?: ValidationRules | null,
): number {
    const bounds = getPreExecFieldBounds(rules);
    const roundedValue = roundFiniteNumber(value ?? Number.NaN);
    const resolvedValue = roundedValue === null || roundedValue <= 0
        ? defaultPreExecIdleTimeout
        : roundedValue;
    return clampInteger(resolvedValue, bounds.idleTimeout.min, bounds.idleTimeout.max);
}

export function readNumberInput(
    nextValue: string,
    previousValue: number,
    min: number,
    max: number,
): number {
    if (nextValue.trim() === "") {
        return previousValue;
    }
    const parsedValue = roundFiniteNumber(Number(nextValue));
    if (parsedValue === null) {
        return previousValue;
    }
    return clampInteger(parsedValue, min, max);
}

export const preExecFieldBounds = getPreExecFieldBounds();
