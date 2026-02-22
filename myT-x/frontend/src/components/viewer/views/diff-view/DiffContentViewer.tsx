import {useMemo} from "react";
import type {WorkingDiffFile} from "./diffViewTypes";

interface DiffContentViewerProps {
    file: WorkingDiffFile | null;
}

interface DiffHunk {
    header: string;
    lines: DiffLine[];
    startOldLine: number;
    startNewLine: number;
}

interface DiffLine {
    type: "context" | "added" | "removed";
    content: string;
    oldLineNum?: number;
    newLineNum?: number;
}

interface DiffGap {
    afterHunkIndex: number;
    hiddenLineCount: number;
}

interface ParsedDiff {
    hunks: DiffHunk[];
    gaps: Map<number, DiffGap>;
}

function parseFileDiff(raw: string): ParsedDiff {
    const hunks: DiffHunk[] = [];
    const lines = raw.split("\n");
    let currentHunk: DiffHunk | null = null;
    let oldLine = 0;
    let newLine = 0;

    for (const line of lines) {
        // Skip diff metadata lines.
        if (
            line.startsWith("diff --git") ||
            line.startsWith("index ") ||
            line.startsWith("---") ||
            line.startsWith("+++") ||
            line.startsWith("new file") ||
            line.startsWith("deleted file") ||
            line.startsWith("similarity") ||
            line.startsWith("rename")
        ) {
            continue;
        }

        // Hunk header.
        if (line.startsWith("@@")) {
            const match = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
            if (!match) {
                continue;
            }
            oldLine = parseInt(match[1], 10);
            newLine = parseInt(match[2], 10);
            currentHunk = {header: line, lines: [], startOldLine: oldLine, startNewLine: newLine};
            hunks.push(currentHunk);
            continue;
        }

        if (!currentHunk) continue;

        // Skip "\ No newline at end of file" marker.
        if (line.startsWith("\\")) continue;

        if (line.startsWith("+")) {
            currentHunk.lines.push({
                type: "added",
                content: line.substring(1),
                newLineNum: newLine++,
            });
        } else if (line.startsWith("-")) {
            currentHunk.lines.push({
                type: "removed",
                content: line.substring(1),
                oldLineNum: oldLine++,
            });
        } else {
            currentHunk.lines.push({
                type: "context",
                content: line.startsWith(" ") ? line.substring(1) : line,
                oldLineNum: oldLine++,
                newLineNum: newLine++,
            });
        }
    }

    // Compute gaps between consecutive hunks.
    const gaps = new Map<number, DiffGap>();
    for (let i = 0; i < hunks.length - 1; i++) {
        const current = hunks[i];
        const next = hunks[i + 1];
        // End old line of current hunk = start + number of old lines consumed.
        let endOldLine = current.startOldLine;
        for (const l of current.lines) {
            if (l.type === "context" || l.type === "removed") {
                endOldLine++;
            }
        }
        const hidden = next.startOldLine - endOldLine;
        if (hidden > 0) {
            gaps.set(i, {afterHunkIndex: i, hiddenLineCount: hidden});
        }
    }

    return {hunks, gaps};
}

export function DiffContentViewer({file}: DiffContentViewerProps) {
    const parsed = useMemo(
        () => (file ? parseFileDiff(file.diff) : {hunks: [], gaps: new Map()}),
        [file?.diff],
    );

    if (!file) {
        return <div className="diff-content-empty">Select a file to view diff</div>;
    }

    return (
        <div className="diff-content-viewer">
            <div className="diff-content-header">
                <span className="diff-content-path">
                    {file.status === "renamed" && file.old_path && file.old_path !== file.path
                        ? `${file.old_path} -> ${file.path}`
                        : file.path}
                </span>
                <span className="diff-header-stats">
          {file.additions > 0 && <span className="diff-tree-additions">+{file.additions}</span>}
                    {file.deletions > 0 && <span className="diff-tree-deletions"> -{file.deletions}</span>}
        </span>
            </div>
            <div className="diff-content-body">
                <div className="diff-viewer">
                    {parsed.hunks.map((hunk, hi) => (
                        <div key={hi}>
                            <div className="diff-hunk-header">{hunk.header}</div>
                            {hunk.lines.map((line, li) => (
                                <div key={`${hi}-${li}`} className={`diff-line ${line.type}`}>
                                    <span className="diff-line-number">{line.oldLineNum ?? ""}</span>
                                    <span className="diff-line-number">{line.newLineNum ?? ""}</span>
                                    <span className="diff-line-content">{line.content}</span>
                                </div>
                            ))}
                            {(() => {
                                const gap = parsed.gaps.get(hi);
                                return gap ? (
                                    <div className="diff-expand-bar" title="Hidden context expansion is not available.">
                                        Hidden context: {gap.hiddenLineCount} lines
                                    </div>
                                ) : null;
                            })()}
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}
