export type TerminalFocusRecoveryReason = "window-focus" | "visibilitychange" | "composition-blur";

const interactiveFocusSelector = [
    "button",
    "a[href]",
    "input",
    "textarea",
    "select",
    "[role=\"button\"]",
    "[role=\"menuitem\"]",
    "[tabindex]:not([tabindex=\"-1\"])",
].join(", ");

const protectedFocusContainerSelector = [
    "[role=\"dialog\"]",
    "[aria-modal=\"true\"]",
    "[role=\"menu\"]",
    ".terminal-search-bar",
].join(", ");

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

function isTextEntryElement(element: HTMLElement): boolean {
    if (element instanceof HTMLTextAreaElement || element instanceof HTMLSelectElement) {
        return true;
    }
    if (element instanceof HTMLInputElement) {
        return !nonTextInputTypes.has(element.type);
    }
    return element.isContentEditable;
}

function isInteractiveElement(element: HTMLElement): boolean {
    return element.matches(interactiveFocusSelector);
}

function shouldPreserveFocusedElement(element: HTMLElement): boolean {
    if (isTextEntryElement(element)) {
        return true;
    }
    return element.closest(protectedFocusContainerSelector) !== null;
}

export function shouldRecoverTerminalFocus(
    reason: TerminalFocusRecoveryReason,
    activeElement: Element | null,
    terminalElement: HTMLElement | null,
    compositionTextarea: HTMLTextAreaElement | null,
): boolean {
    if (activeElement === compositionTextarea) {
        return false;
    }
    if (activeElement === null) {
        return true;
    }

    const ownerDocument = activeElement.ownerDocument;
    if (activeElement === ownerDocument.body) {
        return true;
    }
    if (terminalElement?.contains(activeElement)) {
        return true;
    }
    if (!(activeElement instanceof HTMLElement)) {
        return true;
    }
    if (shouldPreserveFocusedElement(activeElement)) {
        return false;
    }
    if (reason === "composition-blur" && isInteractiveElement(activeElement)) {
        return false;
    }
    return true;
}
