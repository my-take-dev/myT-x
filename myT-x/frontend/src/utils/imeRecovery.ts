const nonTextInputTypes = new Set([
    "button",
    "checkbox",
    "color",
    "file",
    "hidden",
    "image",
    "radio",
    "range",
    "reset",
    "submit",
]);

export const TERMINAL_IME_RECOVERY_EVENT = "mytx:terminal-ime-recovery";
export const IME_RECOVERY_AUTO_COOLDOWN_MS = 1_000;
export const IME_RECOVERY_SURFACE_DELAY_MS = 30;

const IME_RECOVERY_SURFACE_ATTRIBUTE = "data-ime-recovery-surface";

export type TerminalImeRecoveryReason = "manual" | "visibility-change" | "window-focus" | "text-entry-reentry";

export interface TerminalImeRecoveryDetail {
    paneId: string;
    reason: TerminalImeRecoveryReason;
}

const terminalImeRecoveryReasons = new Set<TerminalImeRecoveryReason>([
    "manual",
    "visibility-change",
    "window-focus",
    "text-entry-reentry",
]);

function isRecoverySurface(element: HTMLElement): boolean {
    return element.getAttribute(IME_RECOVERY_SURFACE_ATTRIBUTE) === "true";
}

function isTextInputElement(element: HTMLElement): boolean {
    if (element instanceof HTMLTextAreaElement || element instanceof HTMLSelectElement) {
        return true;
    }
    if (element instanceof HTMLInputElement) {
        return !nonTextInputTypes.has(element.type);
    }
    return element.isContentEditable;
}

export function asRecoverableTextEntryTarget(value: EventTarget | null): HTMLElement | null {
    if (!(value instanceof HTMLElement)) {
        return null;
    }
    if (isRecoverySurface(value) || !isTextInputElement(value)) {
        return null;
    }
    return value;
}

export function isTerminalTextEntryElement(element: HTMLElement | null): boolean {
    return element !== null && element.closest(".xterm") !== null;
}

export function resolveImeRecoveryTarget(
    activeElement: Element | null,
    lastTextEntryTarget: HTMLElement | null,
): HTMLElement | null {
    const activeTarget = asRecoverableTextEntryTarget(activeElement);
    if (activeTarget?.isConnected) {
        return activeTarget;
    }
    if (lastTextEntryTarget?.isConnected) {
        return lastTextEntryTarget;
    }
    return null;
}

export function dispatchTerminalImeRecovery(detail: TerminalImeRecoveryDetail): void {
    if (typeof window === "undefined") {
        return;
    }
    try {
        window.dispatchEvent(new CustomEvent<TerminalImeRecoveryDetail>(TERMINAL_IME_RECOVERY_EVENT, {detail}));
    } catch (err) {
        console.warn("[IME-RECOVERY] failed to dispatch terminal recovery event", err);
    }
}

function isTerminalImeRecoveryReason(value: unknown): value is TerminalImeRecoveryReason {
    return typeof value === "string" && terminalImeRecoveryReasons.has(value as TerminalImeRecoveryReason);
}

export function isTerminalImeRecoveryEvent(event: Event): event is CustomEvent<TerminalImeRecoveryDetail> {
    if (!(event instanceof CustomEvent) || event.type !== TERMINAL_IME_RECOVERY_EVENT) {
        return false;
    }

    const detail = event.detail;
    return typeof detail?.paneId === "string" && isTerminalImeRecoveryReason(detail.reason);
}

function focusHTMLElement(element: HTMLElement): void {
    try {
        element.focus({preventScroll: true});
    } catch {
        try {
            element.focus();
        } catch {
            // Best-effort recovery only.
        }
    }
}

export function restoreImeRecoveryTarget(target: HTMLElement | null): void {
    if (!target?.isConnected) {
        return;
    }
    focusHTMLElement(target);
    if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
        try {
            const cursorOffset = target.value.length;
            target.setSelectionRange(cursorOffset, cursorOffset);
        } catch {
            // Selection is optional and unsupported for some input types.
        }
    }
}

function waitForSurfaceDelay(): Promise<void> {
    return new Promise((resolve) => {
        window.setTimeout(resolve, IME_RECOVERY_SURFACE_DELAY_MS);
    });
}

export async function runImeRecoverySurfaceCycle(
    recoverySurface: HTMLTextAreaElement | null,
    target: HTMLElement | null,
): Promise<void> {
    const focusTarget = target?.isConnected ? target : null;
    if (!recoverySurface?.isConnected) {
        restoreImeRecoveryTarget(focusTarget);
        return;
    }

    try {
        recoverySurface.focus({preventScroll: true});
    } catch {
        restoreImeRecoveryTarget(focusTarget);
        return;
    }

    try {
        recoverySurface.select();
    } catch {
        // Best-effort recovery only.
    }

    await waitForSurfaceDelay();

    try {
        recoverySurface.blur();
    } catch {
        // Best-effort recovery only.
    }

    restoreImeRecoveryTarget(focusTarget);
}
