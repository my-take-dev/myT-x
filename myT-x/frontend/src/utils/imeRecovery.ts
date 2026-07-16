import {isTextEntryElement} from "./terminalFocus";

export const TERMINAL_IME_RECOVERY_EVENT = "mytx:terminal-ime-recovery";
export const IME_RECOVERY_AUTO_COOLDOWN_MS = 1_000;
export const IME_RECOVERY_SURFACE_DELAY_MS = 30;

const IME_RECOVERY_SURFACE_ATTRIBUTE = "data-ime-recovery-surface";
export const TERMINAL_PANE_ID_ATTRIBUTE = "data-terminal-pane-id";
const TERMINAL_PANE_SELECTOR = `[${TERMINAL_PANE_ID_ATTRIBUTE}]`;
const TERMINAL_FOCUS_SUPPRESSION_MS = 250;

const terminalFocusRecoverySuppressions = new Map<string, number>();

export type TerminalImeRecoveryReason =
    | "manual"
    | "visibility-change"
    | "window-focus"
    | "text-entry-reentry"
    | "terminal-focus";

export interface TerminalImeRecoveryDetail {
    paneId: string;
    reason: TerminalImeRecoveryReason;
}

const terminalImeRecoveryReasons = new Set<TerminalImeRecoveryReason>([
    "manual",
    "visibility-change",
    "window-focus",
    "text-entry-reentry",
    "terminal-focus",
]);

function isRecoverySurface(element: HTMLElement): boolean {
    return element.getAttribute(IME_RECOVERY_SURFACE_ATTRIBUTE) === "true";
}

export function asRecoverableTextEntryTarget(value: EventTarget | null): HTMLElement | null {
    if (!(value instanceof HTMLElement)) {
        return null;
    }
    if (isRecoverySurface(value) || !isTextEntryElement(value)) {
        return null;
    }
    return value;
}

export function isTerminalTextEntryElement(element: HTMLElement | null): boolean {
    return element !== null && element.closest(".xterm") !== null;
}

function normalizePaneId(paneId: string | null | undefined): string | null {
    const normalized = paneId?.trim();
    return normalized ? normalized : null;
}

function pruneExpiredTerminalFocusSuppressions(now: number): void {
    for (const [paneId, suppressedUntil] of terminalFocusRecoverySuppressions) {
        if (suppressedUntil < now) {
            terminalFocusRecoverySuppressions.delete(paneId);
        }
    }
}

export function getTerminalTextEntryPaneId(element: HTMLElement | null): string | null {
    if (element === null || !isTerminalTextEntryElement(element)) {
        return null;
    }
    const terminalPane = element.closest<HTMLElement>(TERMINAL_PANE_SELECTOR);
    return normalizePaneId(terminalPane?.getAttribute(TERMINAL_PANE_ID_ATTRIBUTE));
}

export function isActiveTerminalTextEntryElement(
    element: HTMLElement | null,
    activePaneId: string | null,
): boolean {
    const normalizedActivePaneId = normalizePaneId(activePaneId);
    return normalizedActivePaneId !== null && getTerminalTextEntryPaneId(element) === normalizedActivePaneId;
}

export function suppressNextTerminalFocusImeRecovery(paneId: string): void {
    const normalizedPaneId = normalizePaneId(paneId);
    if (normalizedPaneId === null) {
        return;
    }
    const now = Date.now();
    pruneExpiredTerminalFocusSuppressions(now);
    terminalFocusRecoverySuppressions.set(normalizedPaneId, now + TERMINAL_FOCUS_SUPPRESSION_MS);
}

export function consumeTerminalFocusImeRecoverySuppression(paneId: string): boolean {
    const normalizedPaneId = normalizePaneId(paneId);
    if (normalizedPaneId === null) {
        return false;
    }
    const now = Date.now();
    pruneExpiredTerminalFocusSuppressions(now);
    const suppressedUntil = terminalFocusRecoverySuppressions.get(normalizedPaneId);
    if (suppressedUntil === undefined) {
        return false;
    }
    terminalFocusRecoverySuppressions.delete(normalizedPaneId);
    return now <= suppressedUntil;
}

export function __resetTerminalFocusSuppressionsForTest(): void {
    terminalFocusRecoverySuppressions.clear();
}

export function focusTerminalTextEntryByPaneId(paneId: string): boolean {
    const normalizedPaneId = normalizePaneId(paneId);
    if (normalizedPaneId === null) {
        return false;
    }
    const terminalPane = findTerminalPaneById(normalizedPaneId);
    const target = terminalPane?.querySelector<HTMLElement>(".xterm textarea") ?? null;
    if (target === null || !isTerminalTextEntryElement(target) || asRecoverableTextEntryTarget(target) === null) {
        return false;
    }
    focusHTMLElement(target);
    return document.activeElement === target;
}

function findTerminalPaneById(normalizedPaneId: string): HTMLElement | null {
    if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
        return document.querySelector<HTMLElement>(
            `[${TERMINAL_PANE_ID_ATTRIBUTE}="${CSS.escape(normalizedPaneId)}"]`,
        );
    }
    for (const terminalPane of document.querySelectorAll<HTMLElement>(TERMINAL_PANE_SELECTOR)) {
        if (normalizePaneId(terminalPane.getAttribute(TERMINAL_PANE_ID_ATTRIBUTE)) === normalizedPaneId) {
            return terminalPane;
        }
    }
    return null;
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
    } catch (err: unknown) {
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
