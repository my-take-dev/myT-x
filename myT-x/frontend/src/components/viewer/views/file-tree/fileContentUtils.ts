import type {ClipboardEvent, KeyboardEvent as ReactKeyboardEvent} from "react";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {codePointLength} from "../../../../utils/codePointUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {FILE_CONTENT_ROW_HEIGHT_FALLBACK, TYPOGRAPHY_SIGNATURE_ERROR} from "./fileContentConstants";

// ── Clipboard helpers ──

export function handleClipboardError(err: unknown): void {
    notifyClipboardFailure();
    console.warn("[DEBUG-file-content] clipboard write failed", err);
}

/**
 * Write text to the clipboard using the copy event's clipboardData API.
 *
 * Two-stage clipboard write: First attempts copy event's clipboardData API
 * for synchronous write. Falls back to Wails ClipboardSetText IPC on failure.
 *
 * IMPORTANT: Callers must call `event.preventDefault()` before invoking this function.
 * This function does NOT call preventDefault itself - responsibility is centralized in
 * handleBodyCopy to avoid redundant calls and to keep the control flow explicit.
 * (checklist #134 DRY - single responsibility for preventDefault)
 */
export function setClipboardText(event: ClipboardEvent<HTMLDivElement>, text: string): void {
    // SyntheticEvent pooling was removed in React 17. clipboardData is always
    // accessible in this synchronous handler - no need for event.persist().
    const clipboardData = event.clipboardData;
    if (clipboardData) {
        try {
            clipboardData.setData("text/plain", text);
            // NOTE: clipboardData.setData() is a synchronous browser API that writes directly to the
            // clipboard event's data transfer. When it succeeds (no exception), the data is reliably
            // set for the clipboard operation. No additional verification or notification is needed.
            return;
        } catch (err: unknown) {
            console.warn("[DEBUG-file-content] clipboardData.setData failed, falling back", err);
            // NOTE: setData failed - fall through to writeClipboardText fallback below.
        }
    }
    // NOTE: writeClipboardText is async - it uses Wails ClipboardSetText (sync IPC) in production,
    // falling back to navigator.clipboard.writeText in dev. The dev fallback may silently fail
    // if called outside a user-gesture context, but this is acceptable for development only.
    void writeClipboardText(text).catch(handleClipboardError);
}

// ── Selection DOM helpers ──

export function isSelectAllShortcut(event: ReactKeyboardEvent<HTMLDivElement>): boolean {
    return (event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === "a";
}

export function resolveLineElement(node: Node | null): HTMLElement | null {
    if (!node) return null;
    if (node instanceof HTMLElement) {
        return node.closest<HTMLElement>("[data-line-index]");
    }
    if (node.parentElement) {
        return node.parentElement.closest<HTMLElement>("[data-line-index]");
    }
    return null;
}

export function parseLineIndex(el: HTMLElement | null): number | null {
    const raw = el?.dataset.lineIndex;
    if (!raw) return null;
    const parsed = Number(raw);
    return Number.isInteger(parsed) && parsed >= 0 ? parsed : null;
}

export function textOffsetInLine(node: Node, offset: number, lineElement: HTMLElement): number | null {
    if (!lineElement.contains(node)) {
        return null;
    }

    const textSpan = lineElement.querySelector<HTMLElement>(".file-content-line-text");
    if (!textSpan) return null;

    // Node is outside the text span (e.g. inside line number) -> treat as text start.
    if (!textSpan.contains(node)) return 0;

    const range = document.createRange();
    range.selectNodeContents(textSpan);
    try {
        range.setEnd(node, offset);
    } catch (err: unknown) {
        console.warn("[DEBUG-file-content] failed to resolve selection range end", {offset, err});
        return null;
    }
    return codePointLength(range.toString());
}

// ── Row height measurement ──

/**
 * Build a signature string from computed styles that affect row height.
 * Only tracks properties from the line element (box model, font) and the text span
 * (font, whitespace). Line-number font properties are omitted because they do not
 * independently influence the overall row height - the line element's style already
 * captures the effective row dimensions.
 */
export function getTypographyStyleSignature(container: HTMLElement): string {
    const lineElement = container.querySelector<HTMLElement>(".file-content-line");
    // First try to find .file-content-line-text as a child of the line element (normal case).
    // Fall back to a container-level querySelector only when lineElement is null (no rows rendered).
    // Two querySelector calls are acceptable here because this runs only on resize/style changes,
    // not per-frame, and the DOM subtree is shallow.
    const lineTextElement = lineElement?.querySelector<HTMLElement>(".file-content-line-text")
        ?? container.querySelector<HTMLElement>(".file-content-line-text");

    const lineStyleTarget = lineElement ?? container;
    const lineTextStyleTarget = lineTextElement ?? lineStyleTarget;

    try {
        const lineStyle = window.getComputedStyle(lineStyleTarget);
        const lineTextStyle = window.getComputedStyle(lineTextStyleTarget);
        return [
            lineStyle.fontFamily,
            lineStyle.fontSize,
            lineStyle.fontWeight,
            lineStyle.fontStyle,
            lineStyle.lineHeight,
            lineStyle.paddingTop,
            lineStyle.paddingBottom,
            lineStyle.borderTopWidth,
            lineStyle.borderBottomWidth,
            lineTextStyle.fontFamily,
            lineTextStyle.fontSize,
            lineTextStyle.fontWeight,
            lineTextStyle.lineHeight,
            lineTextStyle.whiteSpace,
            lineTextStyle.wordBreak,
        ].join("|");
    } catch (err: unknown) {
        console.warn("[DEBUG-file-content] typography signature unavailable", err);
        // Use a fixed string so that repeated failures (e.g. on every resize event)
        // hit the cached value instead of creating a new DOM probe each time.
        // A failed getComputedStyle indicates a detached or invalid element, so
        // re-measuring with calculateRowHeight would produce the same fallback anyway.
        return TYPOGRAPHY_SIGNATURE_ERROR;
    }
}

export function calculateRowHeight(container: HTMLElement): number {
    // Early exit: document.body must exist before any DOM element creation (checklist #65 constraint check order).
    if (!document.body) return FILE_CONTENT_ROW_HEIGHT_FALLBACK;

    const probe = document.createElement("div");
    probe.className = "file-content-line";
    probe.style.position = "absolute";
    probe.style.visibility = "hidden";
    probe.style.pointerEvents = "none";
    probe.style.left = "-9999px";

    // Copy font styles from the container so the probe measures correctly
    // even though it is appended to document.body (outside the container's subtree).
    const containerStyle = window.getComputedStyle(container);
    probe.style.fontFamily = containerStyle.fontFamily;
    probe.style.fontSize = containerStyle.fontSize;
    probe.style.fontWeight = containerStyle.fontWeight;
    probe.style.lineHeight = containerStyle.lineHeight;
    // Copy box model properties that affect row height measurement.
    // These are tracked by getTypographyStyleSignature and must be consistent with the probe.
    probe.style.paddingTop = containerStyle.paddingTop;
    probe.style.paddingBottom = containerStyle.paddingBottom;
    probe.style.borderTopWidth = containerStyle.borderTopWidth;
    probe.style.borderBottomWidth = containerStyle.borderBottomWidth;
    probe.style.boxSizing = containerStyle.boxSizing;

    const numberSpan = document.createElement("span");
    numberSpan.className = "file-content-line-number";
    numberSpan.textContent = "1";

    const textSpan = document.createElement("span");
    textSpan.className = "file-content-line-text";
    textSpan.textContent = "M";

    // Copy lineHeight from the live .file-content-line-text element to the probe's text span.
    // The text span may have a different lineHeight than the line element (e.g. via CSS cascade),
    // and this can affect the measured row height. getTypographyStyleSignature also tracks
    // lineTextStyle.lineHeight, so the probe must be consistent to avoid cache invalidation loops.
    const liveTextSpan = container.querySelector<HTMLElement>(".file-content-line-text");
    if (liveTextSpan) {
        try {
            const liveTextStyle = window.getComputedStyle(liveTextSpan);
            textSpan.style.fontFamily = liveTextStyle.fontFamily;
            textSpan.style.fontSize = liveTextStyle.fontSize;
            textSpan.style.fontWeight = liveTextStyle.fontWeight;
            textSpan.style.lineHeight = liveTextStyle.lineHeight;
        } catch (err: unknown) {
            console.warn("[DEBUG-file-content] live text style probe unavailable", err);
            // NOTE: getComputedStyle can fail if the element is detached.
            // The probe will still work with inherited styles from the container copy above.
        }
    }

    probe.appendChild(numberSpan);
    probe.appendChild(textSpan);
    let measured = FILE_CONTENT_ROW_HEIGHT_FALLBACK;
    try {
        document.body.appendChild(probe);
        measured = Math.ceil(probe.getBoundingClientRect().height);
    } finally {
        // Check parentNode identity instead of contains() - contains() returns true for
        // subtree nodes too, but removeChild only works for direct children.
        if (probe.parentNode === document.body) {
            document.body.removeChild(probe);
        }
    }

    if (!Number.isFinite(measured) || measured <= 0) {
        // Log in production - row height measurement failure affects layout correctness
        // and is a signal of environmental issues (detached DOM, zero-size container, etc.).
        console.error(
            "[DEBUG-file-content] calculateRowHeight failed: measured=%s, using fallback=%s",
            measured,
            FILE_CONTENT_ROW_HEIGHT_FALLBACK,
        );
        return FILE_CONTENT_ROW_HEIGHT_FALLBACK;
    }
    return measured;
}
