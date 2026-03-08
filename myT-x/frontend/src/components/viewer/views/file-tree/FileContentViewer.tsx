import {
    memo,
    useCallback,
    useEffect,
    useLayoutEffect,
    useMemo,
    useRef,
    useState,
    type ClipboardEvent,
    type KeyboardEvent as ReactKeyboardEvent
} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {useCopyPathNotice} from "../../../../hooks/useCopyPathNotice";
import {useShikiHighlight} from "../../../../hooks/useShikiHighlight";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {detectLineEnding, LINE_ENDING_LF, splitLines, type LineEnding} from "../../../../utils/textLines";
import {sanitizeCssColor} from "../../../../utils/cssUtils";
import {codePointLength} from "../../../../utils/codePointUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {extractSelectedText, type SelectionSpan} from "../../../../utils/selectionUtils";
import {isMarkdownLang, pathToShikiLang} from "../../../../utils/shikiHighlighter";
import {makeScrollStableOuter} from "../shared/TreeOuter";
import {CopyPathButton} from "../shared/CopyPathButton";
import type {FileContentResult} from "./fileTreeTypes";
import {MarkdownPreview} from "./MarkdownPreview";
import {formatFileSize} from "./treeUtils";
import type {ThemedToken} from "shiki/core";

interface FileContentViewerProps {
    content: FileContentResult | null;
    isLoading: boolean;
}

interface FileContentRowData {
    readonly lines: readonly string[];
    readonly tokens: readonly (readonly ThemedToken[])[] | null;
}

const FILE_CONTENT_ROW_HEIGHT_FALLBACK = 20;
/** Debounce delay before copying a mouse-drag selection to clipboard. */
const COPY_ON_SELECT_DEBOUNCE_MS = 100;
/** Duration the selection notice is shown in the header bar. */
const COPY_SELECTION_NOTICE_MS = 1800;
/**
 * Extra rows beyond one viewport to keep mounted for overscan.
 * Combined with the dynamically computed viewport row count, this ensures selection
 * anchors survive moderate scrolling without being unmounted.
 * Value rationale: 4 rows provides a comfortable buffer for mouse-drag selections
 * that extend slightly beyond the visible area, without over-rendering on small screens.
 */
const OVERSCAN_BUFFER = 4;
/** Safety cap to avoid excessive offscreen row rendering on very tall viewports. */
const MAX_OVERSCAN_ROWS = 120;
/** Reserve a sensible initial viewport so the list does not collapse before ResizeObserver reports. */
const MIN_BODY_VIEWPORT_ROWS = 12;
/**
 * Sentinel value returned by getTypographyStyleSignature when getComputedStyle fails
 * (e.g. detached elements). Using a fixed string ensures repeated failures hit the
 * cache instead of triggering new DOM probe measurements each time.
 */
const TYPOGRAPHY_SIGNATURE_ERROR = "typography-signature-error";

/**
 * Module-level factory call — must not be inside a render function (see makeScrollStableOuter).
 * overflowX: "scroll" keeps the horizontal scrollbar always present so viewport height
 * does not oscillate when near-bottom rows have wider content than current rows.
 */
const FileContentListOuter = makeScrollStableOuter({
    role: "list",
    ariaLabel: "File content",
    overflowX: "scroll",
});

function handleClipboardError(err: unknown): void {
    notifyClipboardFailure();
    console.warn("[DEBUG-file-content] clipboard write failed", err);
}

const FileContentRow = memo(function FileContentRow({index, style, data}: ListChildComponentProps<FileContentRowData>) {
    const line = data.lines[index] ?? "";
    const lineTokens = data.tokens?.[index];
    return (
        <div style={style} className="file-content-line" data-line-index={index}>
            <span className="file-content-line-number" aria-hidden="true">{index + 1}</span>
            <span className="file-content-line-text">
                {lineTokens ? (
                    lineTokens.map((token, tokenIndex) => (
                        <span key={`token-${index}-${tokenIndex}`} style={{color: sanitizeCssColor(token.color)}}>
                            {token.content}
                        </span>
                    ))
                ) : (
                    line
                )}
            </span>
        </div>
    );
});

function isSelectAllShortcut(event: ReactKeyboardEvent<HTMLDivElement>): boolean {
    return (event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === "a";
}

function resolveLineElement(node: Node | null): HTMLElement | null {
    if (!node) return null;
    if (node instanceof HTMLElement) {
        return node.closest<HTMLElement>("[data-line-index]");
    }
    if (node.parentElement) {
        return node.parentElement.closest<HTMLElement>("[data-line-index]");
    }
    return null;
}

function parseLineIndex(el: HTMLElement | null): number | null {
    const raw = el?.dataset.lineIndex;
    if (!raw) return null;
    const parsed = Number(raw);
    return Number.isInteger(parsed) && parsed >= 0 ? parsed : null;
}

function textOffsetInLine(node: Node, offset: number, lineElement: HTMLElement): number | null {
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

/**
 * Build a signature string from computed styles that affect row height.
 * Only tracks properties from the line element (box model, font) and the text span
 * (font, whitespace). Line-number font properties are omitted because they do not
 * independently influence the overall row height - the line element's style already
 * captures the effective row dimensions.
 */
function getTypographyStyleSignature(container: HTMLElement): string {
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
function setClipboardText(event: ClipboardEvent<HTMLDivElement>, text: string): void {
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

function calculateRowHeight(container: HTMLElement): number {
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

export function FileContentViewer({content, isLoading}: FileContentViewerProps) {
    const [copySelectionNotice, setCopySelectionNotice] = useState<string | null>(null);
    const [isPreviewMode, setIsPreviewMode] = useState(false);
    // Initial value is FILE_CONTENT_ROW_HEIGHT_FALLBACK (20px). This is replaced by an accurate
    // DOM-measured value once the body element is mounted and calculateRowHeight runs.
    // The first render uses the fallback height, which is close enough to avoid layout jumps.
    const [rowHeight, setRowHeight] = useState(FILE_CONTENT_ROW_HEIGHT_FALLBACK);
    const previewBodyRef = useRef<HTMLDivElement>(null);
    const listBodyRef = useRef<HTMLDivElement>(null);
    // 12 rows keeps the viewer usable before the first ResizeObserver callback.
    // noiseThresholdPx: 1 suppresses ±1px RO churn that causes scroll jitter.
    const bodyHeight = useContainerHeight(
        listBodyRef,
        FILE_CONTENT_ROW_HEIGHT_FALLBACK * MIN_BODY_VIEWPORT_ROWS,
        {noiseThresholdPx: 1},
    );
    const copySelectionNoticeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const selectionTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const rowHeightCacheRef = useRef<{ signature: string; value: number } | null>(null);
    const isSelectAllArmedRef = useRef(false);
    const shouldHandleSelectionRef = useRef(false);
    const linesRef = useRef<string[]>([]);
    const sourceLineEndingRef = useRef<LineEnding>(LINE_ENDING_LF);
    const {copyState: pathCopyState, copyPath} = useCopyPathNotice(content?.path, {
        logPrefix: "[DEBUG-file-content]",
    });
    // Timer clearing helpers - single-point management to avoid scattered clear/null patterns.
    // (checklist #113 timer group cleanup, #134 DRY)
    const clearCopySelectionNoticeTimer = useCallback(() => {
        if (copySelectionNoticeTimerRef.current !== null) {
            clearTimeout(copySelectionNoticeTimerRef.current);
            copySelectionNoticeTimerRef.current = null;
        }
    }, []);
    const clearSelectionTimer = useCallback(() => {
        if (selectionTimerRef.current !== null) {
            clearTimeout(selectionTimerRef.current);
            selectionTimerRef.current = null;
        }
    }, []);

    // Computed early (before content null checks / early returns) so it can be used by
    // effects and callbacks that need to know whether the virtualized text body will render.
    // When content is null, this evaluates to false, which is the correct safe-side fallback
    // - no selection handlers or resize observers will activate. (checklist #92 safe fallback)
    const isTextBodyVisible = Boolean(!isLoading && content && !content.binary);

    // Reset preview mode when the viewed file changes.
    useEffect(() => {
        setIsPreviewMode(false);
    }, [content?.path]);

    // Detect markdown file using the authoritative extension map in shikiHighlighter.
    const isMarkdownFile = useMemo(() => {
        if (!content?.path) return false;
        return isMarkdownLang(pathToShikiLang(content.path));
    }, [content?.path]);

    const shouldShowVirtualizedBody = isTextBodyVisible && (!isMarkdownFile || !isPreviewMode);

    // Syntax highlighting for all code files (including .md in raw mode).
    // Skip highlighting when in markdown preview mode to avoid wasting resources.
    const {tokens, skipInfo} = useShikiHighlight(
        isPreviewMode ? undefined : (content?.content || undefined),
        isPreviewMode ? undefined : content?.path,
    );

    const highlightWarning = useMemo(() => {
        if (!skipInfo) return null;
        // Switch with exhaustive check so TypeScript catches any future additions
        // to HighlightSkipReason at compile time (checklist #142).
        const reason = skipInfo.reason;
        switch (reason) {
            case "size-limit":
                return `Syntax highlighting is intentionally disabled. Showing plain text because file size ${formatFileSize(skipInfo.actual)} exceeds ${formatFileSize(skipInfo.limit)}.`;
            case "line-count-limit":
                return `Syntax highlighting is intentionally disabled. Showing plain text because line count ${skipInfo.actual} exceeds limit ${skipInfo.limit}.`;
            case "line-length-limit":
                return `Syntax highlighting is intentionally disabled. Showing plain text because max line length ${skipInfo.actual} exceeds limit ${skipInfo.limit}.`;
            default: {
                const _exhaustive: never = reason;
                return `Syntax highlighting is intentionally disabled. Showing plain text (unknown reason: ${String(_exhaustive)}).`;
            }
        }
    }, [skipInfo]);

    // Exclusive display: copySelectionNotice (ephemeral) takes priority over
    // highlightWarning (persistent). The copy notice auto-resets after
    // COPY_SELECTION_NOTICE_MS, allowing the highlight warning to reappear.
    const headerNotice = copySelectionNotice ?? highlightWarning;
    const headerNoticeClass = copySelectionNotice
        ? "file-content-copy-warning"
        : "file-content-highlight-warning";

    // Memoize lines to avoid re-splitting on every render.
    const lines = useMemo(() => {
        if (content?.content == null || content.content === "") return [];
        return splitLines(content.content);
    }, [content?.content]);
    // Memoize line ending detection to avoid recalculating on every render.
    const sourceLineEnding = useMemo(
        () => detectLineEnding(content?.content ?? ""),
        [content?.content]
    );

    // Sync refs in layout phase so copy handlers always observe current lines in the same paint.
    useLayoutEffect(() => {
        linesRef.current = lines;
        sourceLineEndingRef.current = sourceLineEnding;
    }, [lines, sourceLineEnding]);

    const rowData = useMemo<FileContentRowData>(() => ({
        lines,
        tokens,
    }), [lines, tokens]);

    // Sync shouldHandleSelectionRef via useEffect to avoid writing refs during the render phase,
    // which is unsafe under Concurrent Mode / StrictMode. The 1-frame delay is acceptable because
    // the selectionchange handler is debounced (COPY_ON_SELECT_DEBOUNCE_MS) and the copy handler
    // runs on user gesture, both of which occur well after layout effects commit.
    // (checklist #95 StrictMode, #105 no side effects in render)
    useEffect(() => {
        shouldHandleSelectionRef.current = shouldShowVirtualizedBody;
        if (!shouldShowVirtualizedBody) {
            isSelectAllArmedRef.current = false;
        }
    }, [shouldShowVirtualizedBody]);

    // Reset copied state when file changes (preview mode is reset synchronously above).
    useEffect(() => {
        setCopySelectionNotice(null);
        clearCopySelectionNoticeTimer();
    }, [content?.path, clearCopySelectionNoticeTimer]);

    // Cleanup timers on unmount.
    // NOTE: resizeFrameRef is NOT cleaned up here - it is exclusively managed by the
    // ResizeObserver useEffect below. This avoids a double-management pattern where two
    // independent effects compete over the same ref, which can cause stale rAF handles
    // under StrictMode's double-invoke behavior.
    // (checklist #95 StrictMode, #101 rAF cleanup, #14 NOTE comment for design intent)
    useEffect(() => {
        return () => {
            clearCopySelectionNoticeTimer();
            clearSelectionTimer();
        };
    }, [clearCopySelectionNoticeTimer, clearSelectionTimer]);

    // Track row height for virtualized list.
    // Body height is handled by useContainerHeight above.
    // Row height is re-measured when bodyHeight changes (which indicates a resize)
    // or when the virtualized body becomes visible.
    useEffect(() => {
        if (!shouldShowVirtualizedBody) return;

        const el = listBodyRef.current;
        if (!el) return;

        const typographySignature = getTypographyStyleSignature(el);
        const cachedRowHeight = rowHeightCacheRef.current;
        if (cachedRowHeight && cachedRowHeight.signature === typographySignature) {
            setRowHeight((prev) => (prev === cachedRowHeight.value ? prev : cachedRowHeight.value));
            return;
        }
        const measuredRowHeight = calculateRowHeight(el);
        rowHeightCacheRef.current = {
            signature: typographySignature,
            value: measuredRowHeight,
        };
        setRowHeight((prev) => (prev === measuredRowHeight ? prev : measuredRowHeight));
    }, [shouldShowVirtualizedBody, bodyHeight]);

    // Copy file path to clipboard.
    const handleCopyPath = useCallback(() => {
        copyPath();
    }, [copyPath]);

    const showSelectionCopyNotice = useCallback((message: string) => {
        setCopySelectionNotice(message);
        clearCopySelectionNoticeTimer();
        copySelectionNoticeTimerRef.current = setTimeout(() => {
            copySelectionNoticeTimerRef.current = null;
            setCopySelectionNotice(null);
        }, COPY_SELECTION_NOTICE_MS);
    }, [clearCopySelectionNoticeTimer]);

    const getCurrentSelectionSpan = useCallback((selection: Selection): SelectionSpan | null => {
        if (selection.rangeCount === 0 || selection.isCollapsed) return null;

        const bodyEl = listBodyRef.current;
        if (!bodyEl) return null;

        const range = selection.getRangeAt(0);
        if (!bodyEl.contains(range.startContainer) || !bodyEl.contains(range.endContainer)) {
            return null;
        }

        const startLineEl = resolveLineElement(range.startContainer);
        const endLineEl = resolveLineElement(range.endContainer);
        if (!startLineEl || !endLineEl) {
            return null;
        }
        const startLineIndex = parseLineIndex(startLineEl);
        const endLineIndex = parseLineIndex(endLineEl);
        if (startLineIndex == null || endLineIndex == null) {
            return null;
        }

        const startOffset = textOffsetInLine(range.startContainer, range.startOffset, startLineEl);
        const endOffset = textOffsetInLine(range.endContainer, range.endOffset, endLineEl);
        if (startOffset == null || endOffset == null) {
            return null;
        }

        return {
            startLineIndex,
            endLineIndex,
            startOffset,
            endOffset,
        };
    }, []);

    const handleBodyKeyDown = useCallback((event: ReactKeyboardEvent<HTMLDivElement>) => {
        if (!shouldHandleSelectionRef.current) return;
        if (isSelectAllShortcut(event)) {
            event.preventDefault();
            isSelectAllArmedRef.current = true;
            const bodyEl = listBodyRef.current;
            const selection = window.getSelection();
            if (bodyEl && selection) {
                const range = document.createRange();
                range.selectNodeContents(bodyEl);
                selection.removeAllRanges();
                selection.addRange(range);
            }
            return;
        }
        if (!event.ctrlKey && !event.metaKey) {
            isSelectAllArmedRef.current = false;
        }
    }, []);

    const handleBodyMouseDown = useCallback(() => {
        isSelectAllArmedRef.current = false;
    }, []);

    const handleBodyBlur = useCallback(() => {
        isSelectAllArmedRef.current = false;
    }, []);

    /** Resolve the current selection to extracted text, or null if the span is unresolvable. */
    const resolveSelectionText = useCallback((selection: Selection): string | null => {
        const span = getCurrentSelectionSpan(selection);
        if (!span) return null;
        const text = extractSelectedText(linesRef.current, span, sourceLineEndingRef.current);
        return text === "" ? null : text;
    }, [getCurrentSelectionSpan]);

    const handleBodyCopy = useCallback((event: ClipboardEvent<HTMLDivElement>) => {
        if (!shouldHandleSelectionRef.current) return;

        const selection = window.getSelection();
        if (!selection) return;

        if (isSelectAllArmedRef.current) {
            // Ctrl+A copies the raw content directly, bypassing extractSelectedText.
            // This is intentional: extractSelectedText operates on individual lines via linesRef
            // and joins them with the detected lineEnding, which produces the same result as
            // content.content for well-formed files. Using the raw content avoids unnecessary
            // re-splitting and is guaranteed to preserve the original byte sequence.
            const fullText = content?.content ?? "";
            // Always prevent default to avoid partial DOM copy in virtual scroll.
            // Without this, an empty fullText would let the browser copy raw DOM nodes
            // (line numbers, partial rows) which produces garbled clipboard content.
            event.preventDefault();
            try {
                if (fullText) {
                    setClipboardText(event, fullText);
                } else {
                    // Empty file: do NOT overwrite clipboard with empty string.
                    // preventDefault already called above to block garbled DOM copy.
                    // Notify user so the no-op is not silently swallowed (checklist #91, #111).
                    showSelectionCopyNotice("File is empty - nothing to copy.");
                }
            } finally {
                // Always clear the armed flag, even if clipboard APIs throw unexpectedly.
                isSelectAllArmedRef.current = false;
            }
            return;
        }

        if (selection.isCollapsed) return;

        const selectedText = resolveSelectionText(selection);
        if (selectedText == null) {
            // Three-tier distinction for unresolvable selections (checklist #123 comment-code alignment):
            //
            // 1. Selection entirely outside body -> no preventDefault, no notice.
            //    The copy event should propagate normally so other elements can handle it.
            //
            // 2. Selection partially extends beyond body (one anchor inside, one outside)
            //    -> preventDefault + notice. We must block the browser's default copy because
            //    it would grab garbled DOM content from the virtual scroll, and the user
            //    needs feedback explaining why the copy didn't work.
            //    NOTE: Nothing is written to the clipboard intentionally - the selection spans
            //    a boundary we cannot reliably resolve, so writing partial/garbled text would
            //    be worse than writing nothing. The user notice explains how to fix the selection.
            //
            // 3. Selection fully inside body but spans unmounted virtual rows
            //    -> preventDefault + different notice.
            //    NOTE: Same rationale as Case 2 - we cannot extract text from unmounted DOM nodes,
            //    so writing nothing is the correct behavior. The notice instructs the user to
            //    scroll to keep lines mounted before copying.
            const bodyEl = listBodyRef.current;
            if (!bodyEl) return;
            if (selection.rangeCount === 0) return;
            const range = selection.getRangeAt(0);
            // Case 1: Selection is entirely outside the file content body; do nothing.
            if (!bodyEl.contains(range.startContainer) && !bodyEl.contains(range.endContainer)) {
                return;
            }
            // Case 2: Selection partially extends beyond the file content area.
            if (!bodyEl.contains(range.startContainer) || !bodyEl.contains(range.endContainer)) {
                event.preventDefault();
                showSelectionCopyNotice("Selection extends beyond the file content area. Select only within the file content to copy.");
                return;
            }
            // Case 3: Selection fully inside body but spans unmounted virtual rows.
            event.preventDefault();
            showSelectionCopyNotice("Selection includes non-rendered lines. Scroll to keep lines mounted, then copy again.");
            return;
        }

        // preventDefault is called before setClipboardText to prevent browser's default
        // DOM-based copy which produces garbled content in virtual scroll containers.
        event.preventDefault();
        setClipboardText(event, selectedText);
    }, [content?.content, resolveSelectionText, showSelectionCopyNotice]);

    // Copy-on-select with debounce (same as terminal).
    useEffect(() => {
        if (!shouldShowVirtualizedBody) {
            clearSelectionTimer();
            return;
        }

        const handleSelectionChange = () => {
            if (!shouldHandleSelectionRef.current) return;

            // Early bail-out: skip DOM traversal when the selection is outside the file body.
            const sel = window.getSelection();
            if (!sel || sel.isCollapsed) return;
            const bodyEl = listBodyRef.current;
            if (bodyEl && sel.anchorNode && !bodyEl.contains(sel.anchorNode)) return;

            clearSelectionTimer();
            selectionTimerRef.current = setTimeout(() => {
                selectionTimerRef.current = null;
                if (!shouldHandleSelectionRef.current) return;
                if (isSelectAllArmedRef.current) return;

                const selection = window.getSelection();
                if (!selection || selection.isCollapsed) return;

                const text = resolveSelectionText(selection);
                if (!text) return;

                void writeClipboardText(text).catch(handleClipboardError);
            }, COPY_ON_SELECT_DEBOUNCE_MS);
        };

        document.addEventListener("selectionchange", handleSelectionChange);
        // Cleanup covers mode switches (shouldShowVirtualizedBody -> false) and unmount.
        // When shouldShowVirtualizedBody becomes false, this effect re-runs: the early return
        // at the top clears selectionTimerRef, then this cleanup removes the listener.
        // Both paths ensure no stale timers or listeners remain.
        return () => {
            document.removeEventListener("selectionchange", handleSelectionChange);
            clearSelectionTimer();
        };
    }, [resolveSelectionText, shouldShowVirtualizedBody, clearSelectionTimer]);

    const listViewportHeight = Math.max(bodyHeight, rowHeight, FILE_CONTENT_ROW_HEIGHT_FALLBACK);

    // Dynamically compute overscan to adapt to varying screen sizes and font configurations.
    // Large screens / small fonts need more overscan rows to keep selection anchors mounted;
    // small screens need fewer to avoid over-rendering. (review item S-05)
    // NOTE: rowHeight starts at FILE_CONTENT_ROW_HEIGHT_FALLBACK (20px), so the initial
    // overscan may be slightly over-estimated for large font sizes. This is acceptable because
    // the effect re-evaluates once rowHeight is measured from the actual DOM.
    const dynamicOverscan = rowHeight > 0
        ? Math.ceil(bodyHeight / rowHeight) + OVERSCAN_BUFFER
        : OVERSCAN_BUFFER;
    const overscanCount = Math.min(MAX_OVERSCAN_ROWS, dynamicOverscan);

    if (isLoading) {
        return <div className="file-content-empty">Loading...</div>;
    }

    if (!content) {
        return <div className="file-content-empty">Select a file to preview</div>;
    }

    if (content.binary) {
        return <div className="file-content-binary">Binary file ({formatFileSize(content.size)})</div>;
    }

    return (
        <div className="file-content-viewer">
            <div className="file-content-header">
                <span className="file-content-path">{content.path}</span>
                <CopyPathButton state={pathCopyState} onClick={handleCopyPath}/>
                {isMarkdownFile && (
                    <button
                        type="button"
                        className={`file-content-toggle-preview${isPreviewMode ? " active" : ""}`}
                        onClick={() => setIsPreviewMode((prev) => !prev)}
                        title={isPreviewMode ? "Show source" : "Show preview"}
                        aria-label={isPreviewMode ? "Show source" : "Show preview"}
                        aria-pressed={isPreviewMode}
                    >
                        {isPreviewMode ? (
                            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                                <path d="M5.5 3L2 8l3.5 5" stroke="currentColor" strokeWidth="1.5"
                                      strokeLinecap="round" strokeLinejoin="round"/>
                                <path d="M10.5 3L14 8l-3.5 5" stroke="currentColor" strokeWidth="1.5"
                                      strokeLinecap="round" strokeLinejoin="round"/>
                            </svg>
                        ) : (
                            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                                <path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z"
                                      stroke="currentColor" strokeWidth="1.5"/>
                                <circle cx="8" cy="8" r="2" stroke="currentColor" strokeWidth="1.5"/>
                            </svg>
                        )}
                    </button>
                )}
                <span className="file-content-size">
                    {formatFileSize(content.size)}
                    {content.truncated ? " (truncated)" : ""}
                </span>
                {headerNotice && (
                    <span className={headerNoticeClass} title={headerNotice}>{headerNotice}</span>
                )}
            </div>
            {isMarkdownFile && isPreviewMode ? (
                <div className="file-content-body" ref={previewBodyRef} tabIndex={0}>
                    <MarkdownPreview content={content.content}/>
                </div>
            ) : (
                <div
                    className="file-content-body file-content-body-virtualized"
                    ref={listBodyRef}
                    tabIndex={0}
                    onKeyDown={handleBodyKeyDown}
                    onMouseDown={handleBodyMouseDown}
                    onBlur={handleBodyBlur}
                    onCopy={handleBodyCopy}
                >
                    <FixedSizeList
                        className="file-content-list"
                        height={listViewportHeight}
                        itemCount={lines.length}
                        itemSize={rowHeight}
                        width="100%"
                        itemData={rowData}
                        outerElementType={FileContentListOuter}
                        // Dynamically computed: viewport rows + OVERSCAN_BUFFER.
                        // Virtual scroll unmounts rows outside the viewport + overscan window.
                        // Selection anchors (start/end nodes) must remain in the DOM for copy to work;
                        // dynamic overscan adapts to screen size and font, preventing the
                        // "non-rendered lines" copy warning without over-rendering on small screens.
                        overscanCount={overscanCount}
                    >
                        {FileContentRow}
                    </FixedSizeList>
                </div>
            )}
        </div>
    );
}
