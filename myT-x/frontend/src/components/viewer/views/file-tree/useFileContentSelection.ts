import {
    useCallback,
    useEffect,
    useLayoutEffect,
    useRef,
    useState,
    type ClipboardEvent,
    type KeyboardEvent as ReactKeyboardEvent,
    type RefObject,
} from "react";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {extractSelectedText, type SelectionSpan} from "../../../../utils/selectionUtils";
import {LINE_ENDING_LF, type LineEnding} from "../../../../utils/textLines";
import {COPY_ON_SELECT_DEBOUNCE_MS, COPY_SELECTION_NOTICE_MS} from "./fileContentConstants";
import {
    handleClipboardError,
    isSelectAllShortcut,
    parseLineIndex,
    resolveLineElement,
    setClipboardText,
    textOffsetInLine,
} from "./fileContentUtils";

interface UseFileContentSelectionOptions {
    /** Raw file content for Ctrl+A full copy. */
    readonly rawContent: string | undefined;
    /** Split lines for selection text extraction. */
    readonly lines: string[];
    /** Detected line ending for preserving CRLF/LF in copied text. */
    readonly sourceLineEnding: LineEnding;
    /** Ref to the virtualized list container for selection boundary checks. */
    readonly listBodyRef: RefObject<HTMLDivElement | null>;
    /** Whether the virtualized body is currently rendered. Gates all selection handling. */
    readonly shouldShowVirtualizedBody: boolean;
    /** Reset key (typically file path) - resets selection notice when changed. */
    readonly resetKey: string | undefined;
}

export interface UseFileContentSelectionResult {
    readonly copySelectionNotice: string | null;
    readonly handleBodyKeyDown: (event: ReactKeyboardEvent<HTMLDivElement>) => void;
    readonly handleBodyMouseDown: () => void;
    readonly handleBodyBlur: () => void;
    readonly handleBodyCopy: (event: ClipboardEvent<HTMLDivElement>) => void;
}

export function useFileContentSelection({
    rawContent,
    lines,
    sourceLineEnding,
    listBodyRef,
    shouldShowVirtualizedBody,
    resetKey,
}: UseFileContentSelectionOptions): UseFileContentSelectionResult {
    const [copySelectionNotice, setCopySelectionNotice] = useState<string | null>(null);
    const copySelectionNoticeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const selectionTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const isSelectAllArmedRef = useRef(false);
    const shouldHandleSelectionRef = useRef(false);
    const linesRef = useRef<string[]>([]);
    const sourceLineEndingRef = useRef<LineEnding>(LINE_ENDING_LF);

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

    // Sync refs in layout phase so copy handlers always observe current lines in the same paint.
    useLayoutEffect(() => {
        linesRef.current = lines;
        sourceLineEndingRef.current = sourceLineEnding;
    }, [lines, sourceLineEnding]);

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

    // Reset copied state when file changes.
    useEffect(() => {
        setCopySelectionNotice(null);
        clearCopySelectionNoticeTimer();
    }, [resetKey, clearCopySelectionNoticeTimer]);

    // Cleanup timers on unmount.
    useEffect(() => {
        return () => {
            clearCopySelectionNoticeTimer();
            clearSelectionTimer();
        };
    }, [clearCopySelectionNoticeTimer, clearSelectionTimer]);

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
    }, [listBodyRef]);

    /** Resolve the current selection to extracted text, or null if the span is unresolvable. */
    const resolveSelectionText = useCallback((selection: Selection): string | null => {
        const span = getCurrentSelectionSpan(selection);
        if (!span) return null;
        const text = extractSelectedText(linesRef.current, span, sourceLineEndingRef.current);
        return text === "" ? null : text;
    }, [getCurrentSelectionSpan]);

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
    }, [listBodyRef]);

    const handleBodyMouseDown = useCallback(() => {
        isSelectAllArmedRef.current = false;
    }, []);

    const handleBodyBlur = useCallback(() => {
        isSelectAllArmedRef.current = false;
    }, []);

    const handleBodyCopy = useCallback((event: ClipboardEvent<HTMLDivElement>) => {
        if (!shouldHandleSelectionRef.current) return;

        const selection = window.getSelection();
        if (!selection) return;

        if (isSelectAllArmedRef.current) {
            // Ctrl+A copies the raw content directly, bypassing extractSelectedText.
            // This is intentional: extractSelectedText operates on individual lines via linesRef
            // and joins them with the detected lineEnding, which produces the same result as
            // rawContent for well-formed files. Using the raw content avoids unnecessary
            // re-splitting and is guaranteed to preserve the original byte sequence.
            const fullText = rawContent ?? "";
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
    }, [rawContent, resolveSelectionText, showSelectionCopyNotice, listBodyRef]);

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
    }, [resolveSelectionText, shouldShowVirtualizedBody, clearSelectionTimer, listBodyRef]);

    return {
        copySelectionNotice,
        handleBodyKeyDown,
        handleBodyMouseDown,
        handleBodyBlur,
        handleBodyCopy,
    };
}
