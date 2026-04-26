import type {DiffLineType, ParsedDiffLine} from "../../../../utils/diffParser";

export interface DiffReviewLineRef {
    readonly lineNum: number;
    readonly lineType: DiffLineType;
}

export interface DiffReviewRange {
    readonly startLineNum: number;
    readonly startLineType: DiffLineType;
    readonly endLineNum: number;
    readonly endLineType: DiffLineType;
    readonly lineContent: string;
}

export function toDiffReviewLineRef(line: ParsedDiffLine): DiffReviewLineRef {
    return {
        lineNum: line.type === "removed" ? line.oldLineNum : line.newLineNum,
        lineType: line.type,
    };
}

export function formatDiffReviewLineLabel(line: DiffReviewLineRef): string {
    const prefix = line.lineType === "added" ? "+" : line.lineType === "removed" ? "-" : "";
    return `L${prefix}${line.lineNum}`;
}

function formatDiffReviewBoundaryLabel(line: DiffReviewLineRef): string {
    if (line.lineType === "added") {
        return `new L${line.lineNum}`;
    }
    if (line.lineType === "removed") {
        return `old L${line.lineNum}`;
    }
    return formatDiffReviewLineLabel(line);
}

export function formatDiffReviewRangeLabel(start: DiffReviewLineRef, end: DiffReviewLineRef): string {
    if (start.lineType !== end.lineType) {
        const startLabel = formatDiffReviewBoundaryLabel(start);
        const endLabel = formatDiffReviewBoundaryLabel(end);
        return startLabel === endLabel ? startLabel : `${startLabel} to ${endLabel}`;
    }
    const startLabel = formatDiffReviewLineLabel(start);
    const endLabel = formatDiffReviewLineLabel(end);
    return startLabel === endLabel ? startLabel : `${startLabel} to ${endLabel}`;
}

export function buildDiffReviewRange(
    lines: readonly ParsedDiffLine[],
    startIndex: number,
    endIndex: number,
): DiffReviewRange | null {
    if (startIndex < 0 || endIndex < startIndex || endIndex >= lines.length) {
        return null;
    }

    const startLine = lines[startIndex];
    const endLine = lines[endIndex];
    if (startLine == null || endLine == null) {
        return null;
    }

    const startRef = toDiffReviewLineRef(startLine);
    const endRef = toDiffReviewLineRef(endLine);
    const lineContent = lines
        .slice(startIndex, endIndex + 1)
        .map((line) => line.content)
        .join("\n");

    return {
        startLineNum: startRef.lineNum,
        startLineType: startRef.lineType,
        endLineNum: endRef.lineNum,
        endLineType: endRef.lineType,
        lineContent,
    };
}
