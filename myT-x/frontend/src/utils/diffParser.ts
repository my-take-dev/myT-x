import {splitLines} from "./textLines";

export type ParsedDiffLine =
    | {
    type: "context";
    content: string;
    oldLineNum: number;
    newLineNum: number;
}
    | {
    type: "added";
    content: string;
    oldLineNum?: never;
    newLineNum: number;
}
    | {
    type: "removed";
    content: string;
    oldLineNum: number;
    newLineNum?: never;
};

export type DiffLineType = ParsedDiffLine["type"];

export interface ParsedDiffHunk {
    readonly header: string;
    readonly lines: readonly ParsedDiffLine[];
    readonly startOldLine: number;
    readonly startNewLine: number;
}

export interface ParsedDiffFile {
    readonly header: string | null;
    readonly hunks: readonly ParsedDiffHunk[];
}

export interface ParsedDiffGap {
    readonly afterHunkIndex: number;
    readonly hiddenLineCount: number;
}

export type ParseSingleFileDiffResult =
    | {
    readonly status: "success";
    readonly hunks: readonly ParsedDiffHunk[];
    readonly gaps: ReadonlyMap<number, ParsedDiffGap>;
    readonly fileCount: number;
}
    | {
    readonly status: "error";
    /** @invariant Always a non-empty, user-facing error description. */
    readonly message: string;
};

/** Build an error result with a non-empty message guarantee. */
function makeParseError(message: string): ParseSingleFileDiffResult {
    const safeMessage = message || "Failed to parse diff.";
    if (!message) {
        // Bug: callers should always provide a non-empty message.
        // Logged in all environments so production issues are traceable.
        console.error("[diffParser] BUG: empty error message passed to makeParseError");
    }
    return {status: "error" as const, message: safeMessage};
}

/**
 * Line prefixes that identify diff metadata rather than file content.
 * Used as a fallback filter inside active hunks: when a line lacks normal diff prefixes
 * (+, -, space), matching these patterns causes it to be skipped rather than treated as context.
 *
 * NOTE: "---" and "+++" can appear as file-level metadata between "diff --git" and the first
 * "@@" header. Inside a hunk, prefixed content lines ("---foo", "+++bar") are parsed by the
 * +/- branches before this fallback, so this metadata filter applies only to malformed/bare
 * metadata lines that reach the fallback path.
 */
const METADATA_PREFIXES = [
    "index ",
    "---",
    "+++",
    "new file",
    "deleted file",
    "similarity",
    "rename",
];

function isMetadataLine(line: string): boolean {
    return METADATA_PREFIXES.some((prefix) => line.startsWith(prefix));
}

function parseHunkHeader(line: string): { startOldLine: number; startNewLine: number } | null {
    const match = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    if (!match) {
        console.warn("[diff-parser] Invalid hunk header skipped:", line);
        return null;
    }

    return {
        startOldLine: Number.parseInt(match[1], 10),
        startNewLine: Number.parseInt(match[2], 10),
    };
}

function oldLineCursorAfterHunk(hunk: ParsedDiffHunk): number {
    const oldLinesConsumed = hunk.lines.reduce((count, line) => {
        return line.type === "added" ? count : count + 1;
    }, 0);
    return hunk.startOldLine + oldLinesConsumed;
}

/**
 * Compute hidden line-count gaps between adjacent hunks, keyed by previous hunk index.
 */
export function computeHunkGaps(hunks: readonly ParsedDiffHunk[]): Map<number, ParsedDiffGap> {
    const gaps = new Map<number, ParsedDiffGap>();
    for (let i = 0; i < hunks.length - 1; i++) {
        const current = hunks[i];
        const next = hunks[i + 1];
        const hiddenLineCount = next.startOldLine - oldLineCursorAfterHunk(current);
        if (hiddenLineCount > 0) {
            gaps.set(i, {
                afterHunkIndex: i,
                hiddenLineCount,
            });
        }
    }
    return gaps;
}

// Mutable builder types used internally by parseDiffFiles during construction.
// The exported interfaces have readonly fields to prevent external mutation,
// but the parser needs to push() into arrays during construction.
// TypeScript allows assigning mutable arrays to readonly arrays, so the
// return type coercion from MutableDiffHunk → ParsedDiffHunk is implicit.
interface MutableDiffHunk {
    header: string;
    lines: ParsedDiffLine[];
    startOldLine: number;
    startNewLine: number;
}

interface MutableDiffFile {
    header: string | null;
    hunks: MutableDiffHunk[];
}

/**
 * Parse unified diff text into per-file/per-hunk structures.
 */
export function parseDiffFiles(raw: string): ParsedDiffFile[] {
    const files: MutableDiffFile[] = [];
    const lines = splitLines(raw);
    let currentFile: MutableDiffFile | null = null;
    let currentHunk: MutableDiffHunk | null = null;
    let oldLine = 0;
    let newLine = 0;

    for (const line of lines) {
        if (line.startsWith("diff --git")) {
            currentFile = {header: line, hunks: []};
            files.push(currentFile);
            currentHunk = null;
            continue;
        }

        if (line.startsWith("@@")) {
            if (!currentFile) {
                currentFile = {header: null, hunks: []};
                files.push(currentFile);
            }

            const parsedHeader = parseHunkHeader(line);
            if (!parsedHeader) {
                currentHunk = null;
                continue;
            }

            oldLine = parsedHeader.startOldLine;
            newLine = parsedHeader.startNewLine;
            currentHunk = {
                header: line,
                lines: [],
                startOldLine: oldLine,
                startNewLine: newLine,
            };
            currentFile.hunks.push(currentHunk);
            continue;
        }

        // "\ No newline at end of file" is informational metadata, not a diff content row.
        // Skip it globally. Also skip any lines outside an active hunk (metadata/binary lines).
        if (line.startsWith("\\") || !currentHunk) {
            continue;
        }

        if (line.startsWith("+")) {
            currentHunk.lines.push({
                type: "added",
                content: line.substring(1),
                newLineNum: newLine++,
            });
            continue;
        }

        if (line.startsWith("-")) {
            currentHunk.lines.push({
                type: "removed",
                content: line.substring(1),
                oldLineNum: oldLine++,
            });
            continue;
        }

        // Defensive fallback for malformed diffs where metadata-like lines appear inside a hunk
        // without diff prefixes. Prefixed content lines (+/-/ ) are already handled above.
        if (isMetadataLine(line)) {
            continue;
        }

        currentHunk.lines.push({
            type: "context",
            content: line.startsWith(" ") ? line.substring(1) : line,
            oldLineNum: oldLine++,
            newLineNum: newLine++,
        });
    }

    return files;
}

/**
 * Parse diff content assumed to be for a single file.
 * If the input contains multiple file diffs, only the first file with hunks is extracted.
 * Use parseDiffFiles() for multi-file diff content.
 *
 * Returns `fileCount` so callers (e.g., DEV warnings) can detect multi-file input
 * without re-parsing.
 */
export function parseSingleFileDiff(raw: string): ParseSingleFileDiffResult {
    try {
        const allFiles = parseDiffFiles(raw);
        const firstFileWithHunks = allFiles.find((file) => file.hunks.length > 0);
        // Use a multiline regex instead of splitLines() to detect hunk headers
        // in one pass: avoids an O(n) line-array allocation since parseDiffFiles
        // already performs its own splitLines() internally.
        const hasHunkHeader = /^@@ -\d+/m.test(raw);
        if (!firstFileWithHunks && hasHunkHeader) {
            return makeParseError("Failed to parse diff: hunk headers found but not recognized.");
        }
        const hunks = firstFileWithHunks?.hunks ?? [];
        return {
            status: "success",
            hunks,
            gaps: computeHunkGaps(hunks),
            fileCount: allFiles.length,
        };
    } catch (err: unknown) {
        console.error("[diffParser] parseSingleFileDiff failed", err);
        // Always use a generic message for UI display.
        // Technical details are logged via the console.error call above.
        return makeParseError("Failed to parse diff.");
    }
}

/**
 * Extract the file path from a `diff --git a/... b/...` header line.
 * Returns the `b/` side (new filename) so renames show the current name.
 * Returns `"(untitled)"` when `header` is `null`.
 * Returns the raw header string for headers that don't match the expected `diff --git` format.
 */
export function diffHeaderToFilePath(header: string | null): string {
    if (header === null) {
        return "(untitled)";
    }
    // Extract from b/ side (new filename) so renames show the current name.
    const match = header.match(/^diff --git a\/.+? b\/(.+)$/);
    return match?.[1] ?? header;
}
