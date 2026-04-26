import {memo, useCallback, useEffect, useMemo, useRef, useState} from "react";
import type {ParsedDiffGap, ParsedDiffHunk} from "../../../../utils/diffParser";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {buildDiffReviewDraftKey} from "./diffReviewKeys";
import {DiffReviewLineRow} from "./DiffReviewLineRow";
import {useDiffReviewSessionKey} from "./diffReviewSession";

interface DiffHunkSectionWithReviewProps {
    readonly filePath: string;
    readonly oldFilePath?: string;
    readonly hunk: ParsedDiffHunk;
    readonly gap?: ParsedDiffGap;
}

interface PendingRangeSelectionRequest {
    readonly anchorLineKey: string;
    readonly endIndex: number;
    readonly token: number;
}

interface DragRangeSelection {
    readonly anchorIndex: number;
    readonly hoveredIndex: number;
    readonly hasMoved: boolean;
}

export const DiffHunkSectionWithReview = memo(function DiffHunkSectionWithReview({
    filePath,
    oldFilePath,
    hunk,
    gap,
}: DiffHunkSectionWithReviewProps) {
    const activeSessionKey = useDiffReviewSessionKey();
    const setActiveCommentLineKey = useDiffReviewStore((state) => state.setActiveCommentLineKey);
    const [dragRangeSelection, setDragRangeSelection] = useState<DragRangeSelection | null>(null);
    const [pendingRangeSelectionRequest, setPendingRangeSelectionRequest] = useState<PendingRangeSelectionRequest | null>(null);
    const dragRangeSelectionRef = useRef<DragRangeSelection | null>(null);
    const selectionRequestTokenRef = useRef(0);
    const suppressNextAddClickLineKeyRef = useRef<string | null>(null);
    const suppressResetFrameRef = useRef<number | null>(null);
    const lineRows = useMemo(
        () => hunk.lines.map((line, li) => ({
            line,
            lineIndex: li,
            lineKey: `${hunk.header}:${line.oldLineNum ?? "n"}:${line.newLineNum ?? "n"}:${li}`,
        })),
        [hunk.header, hunk.lines],
    );

    const resetSuppressedAddClick = useCallback(() => {
        if (suppressResetFrameRef.current != null) {
            cancelAnimationFrame(suppressResetFrameRef.current);
            suppressResetFrameRef.current = null;
        }
        suppressNextAddClickLineKeyRef.current = null;
    }, []);

    const scheduleSuppressedAddClickReset = useCallback(() => {
        if (suppressResetFrameRef.current != null) {
            cancelAnimationFrame(suppressResetFrameRef.current);
        }
        suppressResetFrameRef.current = requestAnimationFrame(() => {
            suppressResetFrameRef.current = null;
            suppressNextAddClickLineKeyRef.current = null;
        });
    }, []);

    const consumePendingAddClickSuppression = useCallback((lineKey: string) => {
        if (suppressNextAddClickLineKeyRef.current !== lineKey) {
            return false;
        }
        resetSuppressedAddClick();
        return true;
    }, [resetSuppressedAddClick]);

    const handleRangeSelectionStart = useCallback((anchorIndex: number) => {
        setDragRangeSelection({
            anchorIndex,
            hoveredIndex: anchorIndex,
            hasMoved: false,
        });
    }, []);

    const handleRangeSelectionHover = useCallback((lineIndex: number) => {
        setDragRangeSelection((current) => {
            if (current == null) {
                return current;
            }
            const nextHoveredIndex = lineIndex;
            const hasMoved = current.hasMoved || nextHoveredIndex !== current.anchorIndex;
            if (nextHoveredIndex === current.hoveredIndex && hasMoved === current.hasMoved) {
                return current;
            }
            return {
                ...current,
                hoveredIndex: nextHoveredIndex,
                hasMoved,
            };
        });
    }, []);

    useEffect(() => {
        dragRangeSelectionRef.current = dragRangeSelection;
    }, [dragRangeSelection]);

    useEffect(() => () => {
        resetSuppressedAddClick();
    }, [resetSuppressedAddClick]);

    useEffect(() => {
        if (dragRangeSelection == null) {
            return;
        }

        function handleMouseUp() {
            const current = dragRangeSelectionRef.current;
            dragRangeSelectionRef.current = null;
            setDragRangeSelection(null);
            if (current == null || !current.hasMoved) {
                return;
            }

            const selectionStartIndex = Math.min(current.anchorIndex, current.hoveredIndex);
            const selectionEndIndex = Math.max(current.anchorIndex, current.hoveredIndex);
            const anchorLineKey = lineRows[selectionStartIndex]?.lineKey;
            const suppressedLineKey = lineRows[current.anchorIndex]?.lineKey;
            if (anchorLineKey == null || suppressedLineKey == null || activeSessionKey === "") {
                return;
            }

            suppressNextAddClickLineKeyRef.current = suppressedLineKey;
            scheduleSuppressedAddClickReset();
            selectionRequestTokenRef.current += 1;
            setPendingRangeSelectionRequest({
                anchorLineKey,
                endIndex: selectionEndIndex,
                token: selectionRequestTokenRef.current,
            });
            setActiveCommentLineKey(buildDiffReviewDraftKey(activeSessionKey, filePath, anchorLineKey));
        }

        window.addEventListener("mouseup", handleMouseUp);
        return () => {
            window.removeEventListener("mouseup", handleMouseUp);
        };
    }, [activeSessionKey, dragRangeSelection, filePath, lineRows, scheduleSuppressedAddClickReset, setActiveCommentLineKey]);

    const selectedRangeStartIndex = dragRangeSelection == null ? null : Math.min(
        dragRangeSelection.anchorIndex,
        dragRangeSelection.hoveredIndex,
    );
    const selectedRangeEndIndex = dragRangeSelection == null ? null : Math.max(
        dragRangeSelection.anchorIndex,
        dragRangeSelection.hoveredIndex,
    );

    return (
        <div role="grid" aria-label="Diff review lines">
            <div className="diff-hunk-header">{hunk.header}</div>
            {/* li index suffix breaks ties when duplicate line numbers appear in a hunk.
                The hunk line list is never reordered, so index-based keys are stable. */}
            {lineRows.map(({line, lineIndex, lineKey}) => (
                <DiffReviewLineRow
                    key={lineKey}
                    filePath={filePath}
                    oldFilePath={oldFilePath}
                    lineKey={lineKey}
                    line={line}
                    hunkLines={hunk.lines}
                    lineIndex={lineIndex}
                    requestedSelectionEndIndex={
                        pendingRangeSelectionRequest?.anchorLineKey === lineKey
                            ? pendingRangeSelectionRequest.endIndex
                            : undefined
                    }
                    requestedSelectionToken={
                        pendingRangeSelectionRequest?.anchorLineKey === lineKey
                            ? pendingRangeSelectionRequest.token
                            : undefined
                    }
                    onRangeSelectionStart={handleRangeSelectionStart}
                    onRangeSelectionHover={handleRangeSelectionHover}
                    consumePendingAddClickSuppression={consumePendingAddClickSuppression}
                    isInDragSelection={
                        selectedRangeStartIndex != null
                        && selectedRangeEndIndex != null
                        && lineIndex >= selectedRangeStartIndex
                        && lineIndex <= selectedRangeEndIndex
                    }
                    isDragSelectionAnchor={dragRangeSelection?.anchorIndex === lineIndex}
                />
            ))}
            {gap && (
                <div className="diff-expand-bar diff-expand-bar--disabled"
                     title="Hidden context expansion is not available.">
                    Hidden context: {gap.hiddenLineCount} lines
                </div>
            )}
        </div>
    );
});
