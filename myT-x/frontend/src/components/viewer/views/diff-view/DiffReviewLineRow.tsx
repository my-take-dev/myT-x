import {memo, type MouseEvent, useCallback, useEffect, useMemo, useState} from "react";
import type {ParsedDiffLine} from "../../../../utils/diffParser";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {type DiffCommentRangeOption, DiffCommentForm} from "./DiffCommentForm";
import {useDiffReviewSessionKey} from "./diffReviewSession";
import {buildDiffReviewDraftKey} from "./diffReviewKeys";
import {buildDiffReviewRange, formatDiffReviewRangeLabel, toDiffReviewLineRef} from "./diffReviewRange";

interface DiffReviewLineRowProps {
    readonly filePath: string;
    readonly oldFilePath?: string;
    readonly lineKey: string;
    readonly line: ParsedDiffLine;
    readonly hunkLines: readonly ParsedDiffLine[];
    readonly lineIndex: number;
    readonly requestedSelectionEndIndex?: number;
    readonly requestedSelectionToken?: number;
    readonly onRangeSelectionStart?: (anchorIndex: number) => void;
    readonly onRangeSelectionHover?: (lineIndex: number) => void;
    readonly consumePendingAddClickSuppression?: (lineKey: string) => boolean;
    readonly isInDragSelection?: boolean;
    readonly isDragSelectionAnchor?: boolean;
    readonly hasPendingComment?: boolean;
}

interface DiffReviewRangeOption extends DiffCommentRangeOption {
    readonly endIndex: number;
}

export const DiffReviewLineRow = memo(function DiffReviewLineRow({
    filePath,
    oldFilePath,
    lineKey,
    line,
    hunkLines,
    lineIndex,
    requestedSelectionEndIndex,
    requestedSelectionToken,
    onRangeSelectionStart,
    onRangeSelectionHover,
    consumePendingAddClickSuppression,
    isInDragSelection = false,
    isDragSelectionAnchor = false,
    hasPendingComment = false,
}: DiffReviewLineRowProps) {
    const activeSessionKey = useDiffReviewSessionKey();
    const scopedLineKey = useMemo(
        () => buildDiffReviewDraftKey(activeSessionKey, filePath, lineKey),
        [activeSessionKey, filePath, lineKey],
    );
    const addComment = useDiffReviewStore((state) => state.addComment);
    const clearDraft = useDiffReviewStore((state) => state.clearDraft);
    const setActiveCommentLineKey = useDiffReviewStore((state) => state.setActiveCommentLineKey);
    const isFormOpen = useDiffReviewStore((state) => state.activeCommentLineKey === scopedLineKey);
    const [selectedRangeEndIndex, setSelectedRangeEndIndex] = useState(lineIndex);
    const canAddComment = activeSessionKey !== "";

    const rangeOptions = useMemo<DiffReviewRangeOption[]>(() => {
        const startRef = toDiffReviewLineRef(line);
        const options: DiffReviewRangeOption[] = [];
        for (let endIndex = lineIndex; endIndex < hunkLines.length; endIndex++) {
            const endLine = hunkLines[endIndex];
            if (endLine == null) {
                continue;
            }
            options.push({
                value: String(endIndex),
                label: formatDiffReviewRangeLabel(startRef, toDiffReviewLineRef(endLine)),
                endIndex,
            });
        }
        return options;
    }, [hunkLines, line, lineIndex]);

    const selectedRange = useMemo(
        () => buildDiffReviewRange(hunkLines, lineIndex, selectedRangeEndIndex),
        [hunkLines, lineIndex, selectedRangeEndIndex],
    );

    useEffect(() => {
        if (isFormOpen) return;
        setSelectedRangeEndIndex(lineIndex);
    }, [isFormOpen, lineIndex]);

    useEffect(() => {
        if (requestedSelectionEndIndex == null) {
            return;
        }
        setSelectedRangeEndIndex(requestedSelectionEndIndex);
    }, [requestedSelectionEndIndex, requestedSelectionToken]);

    const handleAddMouseDown = useCallback((event: MouseEvent<HTMLButtonElement>) => {
        if (event.button !== 0 || !canAddComment) {
            return;
        }
        event.preventDefault();
        onRangeSelectionStart?.(lineIndex);
    }, [canAddComment, lineIndex, onRangeSelectionStart]);

    const handleRangeSelectionMove = useCallback((_event: MouseEvent<HTMLDivElement>) => {
        onRangeSelectionHover?.(lineIndex);
    }, [lineIndex, onRangeSelectionHover]);

    const handleAddClick = useCallback(() => {
        if (consumePendingAddClickSuppression?.(lineKey)) {
            return;
        }
        if (activeSessionKey === "") return;
        setSelectedRangeEndIndex(lineIndex);
        setActiveCommentLineKey(scopedLineKey);
    }, [activeSessionKey, consumePendingAddClickSuppression, lineIndex, lineKey, scopedLineKey, setActiveCommentLineKey]);

    const handleSave = useCallback(
        (text: string) => {
            if (activeSessionKey === "" || selectedRange == null) return;
            addComment({
                sessionKey: activeSessionKey,
                filePath,
                oldFilePath,
                startLineNum: selectedRange.startLineNum,
                startLineType: selectedRange.startLineType,
                endLineNum: selectedRange.endLineNum,
                endLineType: selectedRange.endLineType,
                lineContent: selectedRange.lineContent,
                commentText: text,
            });
            clearDraft(scopedLineKey);
            setActiveCommentLineKey(null);
        },
        [activeSessionKey, addComment, clearDraft, filePath, oldFilePath, scopedLineKey, selectedRange, setActiveCommentLineKey],
    );

    const handleCancel = useCallback(() => {
        setActiveCommentLineKey(null);
    }, [setActiveCommentLineKey]);

    const handleRangeChange = useCallback(
        (value: string) => {
            const nextEndIndex = Number.parseInt(value, 10);
            if (!Number.isInteger(nextEndIndex) || nextEndIndex < lineIndex || nextEndIndex >= hunkLines.length) {
                return;
            }
            setSelectedRangeEndIndex(nextEndIndex);
        },
        [hunkLines.length, lineIndex],
    );

    const oldLineLabel = line.type !== "added" ? `old line ${line.oldLineNum}` : "old line";
    const newLineLabel = line.type !== "removed" ? `new line ${line.newLineNum}` : "new line";
    const rowClassNames = ["diff-line", line.type, "diff-review-line-wrapper"];
    if (isInDragSelection) {
        rowClassNames.push("diff-review-line-wrapper--range-selected");
    }
    if (isDragSelectionAnchor) {
        rowClassNames.push("diff-review-line-wrapper--range-anchor");
    }
    if (hasPendingComment) {
        rowClassNames.push("diff-review-line-wrapper--pending-comment");
    }

    return (
        <>
            <div
                className={rowClassNames.join(" ")}
                onMouseMove={handleRangeSelectionMove}
                role="row"
                aria-selected={isInDragSelection}
            >
                <span className="diff-review-add-btn-cell">
                    <button
                        type="button"
                        className="diff-review-add-btn"
                        onMouseDown={handleAddMouseDown}
                        onClick={handleAddClick}
                        disabled={!canAddComment}
                        title="Add review comment"
                        aria-label="Add review comment"
                    >
                        +
                    </button>
                </span>
                <span className="diff-line-number" aria-label={oldLineLabel}>
                    {line.type !== "added" ? line.oldLineNum : ""}
                </span>
                <span className="diff-line-number" aria-label={newLineLabel}>
                    {line.type !== "removed" ? line.newLineNum : ""}
                </span>
                <span className="diff-line-content">{line.content}</span>
            </div>
            {isFormOpen && (
                <DiffCommentForm
                    draftKey={scopedLineKey}
                    onSave={handleSave}
                    onCancel={handleCancel}
                    rangeOptions={rangeOptions}
                    selectedRangeValue={String(selectedRangeEndIndex)}
                    onRangeChange={handleRangeChange}
                />
            )}
        </>
    );
});
