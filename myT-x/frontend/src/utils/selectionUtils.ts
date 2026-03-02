import {codePointLength, sliceByCodePoints} from "./codePointUtils";
import type {LineEnding} from "./textLines";

/**
 * Represents a text selection span in a virtualized line list.
 * NOTE: startLineIndex may be greater than endLineIndex for reverse
 * (bottom-to-top) drag selections. extractSelectedText normalizes
 * the indices internally.
 */
export interface SelectionSpan {
    readonly startLineIndex: number;
    readonly endLineIndex: number;
    readonly startOffset: number;
    readonly endOffset: number;
}

/**
 * Extract text from virtualized lines based on a selection span.
 *
 * Handles both forward and reverse (bottom-to-top) multi-line drag
 * selections by normalizing line indices and offsets at the top.
 * Single-line reverse offsets are also handled via Math.min/Math.max.
 *
 * Line indices may exceed lines.length — clamped to bounds here.
 */
export function extractSelectedText(lines: string[], span: SelectionSpan, lineEnding: LineEnding): string {
    if (lines.length === 0) return "";

    // Normalize for reverse (bottom-to-top) multi-line drag selections.
    let startLineIndex = span.startLineIndex;
    let endLineIndex = span.endLineIndex;
    let startLineOffset = span.startOffset;
    let endLineOffset = span.endOffset;
    if (startLineIndex > endLineIndex) {
        [startLineIndex, endLineIndex] = [endLineIndex, startLineIndex];
        [startLineOffset, endLineOffset] = [endLineOffset, startLineOffset];
    }

    const startIndex = Math.max(0, Math.min(startLineIndex, lines.length - 1));
    const endIndex = Math.max(0, Math.min(endLineIndex, lines.length - 1));

    const firstLine = lines[startIndex] ?? "";
    const lastLine = lines[endIndex] ?? "";

    const startOffset = Math.max(0, Math.min(startLineOffset, codePointLength(firstLine)));
    const endOffset = Math.max(0, Math.min(endLineOffset, codePointLength(lastLine)));

    if (startIndex === endIndex) {
        // Normalize offsets for reverse (right-to-left) drag selections on the same line.
        // When startIndex === endIndex (same line), the multi-line swap above is a no-op,
        // so startOffset/endOffset may still be in reverse order (endOffset < startOffset).
        // The lo/hi pattern ensures correct extraction regardless of drag direction.
        const lo = Math.min(startOffset, endOffset);
        const hi = Math.max(startOffset, endOffset);
        return sliceByCodePoints(firstLine, lo, hi);
    }

    const chunk: string[] = [];
    chunk.push(sliceByCodePoints(firstLine, startOffset));
    for (let i = startIndex + 1; i < endIndex; i++) {
        chunk.push(lines[i] ?? "");
    }
    chunk.push(sliceByCodePoints(lastLine, 0, endOffset));
    return chunk.join(lineEnding);
}
