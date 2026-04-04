import type React from "react";
import type {QueueConfig} from "./useTaskScheduler";

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

const defaultPreExecResetDelay = 10;
const defaultPreExecIdleTimeout = 120;
const minPreExecResetDelay = 0;
const maxPreExecResetDelay = 60;
const minPreExecIdleTimeout = 10;
const maxPreExecIdleTimeout = 600;

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
    return value === "all_panes" ? "all_panes" : "task_panes";
}

export function resolveInitialPreExecResetDelay(value: number | null | undefined): number {
    const roundedValue = roundFiniteNumber(value ?? Number.NaN);
    if (roundedValue === null || roundedValue < 0) {
        return defaultPreExecResetDelay;
    }
    return clampInteger(roundedValue, minPreExecResetDelay, maxPreExecResetDelay);
}

export function resolveInitialPreExecIdleTimeout(value: number | null | undefined): number {
    const roundedValue = roundFiniteNumber(value ?? Number.NaN);
    if (roundedValue === null || roundedValue <= 0) {
        return defaultPreExecIdleTimeout;
    }
    return clampInteger(roundedValue, minPreExecIdleTimeout, maxPreExecIdleTimeout);
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

export const preExecFieldBounds = {
    idleTimeout: {
        max: maxPreExecIdleTimeout,
        min: minPreExecIdleTimeout,
    },
    resetDelay: {
        max: maxPreExecResetDelay,
        min: minPreExecResetDelay,
    },
} as const;
