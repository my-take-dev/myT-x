import {memo} from "react";
import type {ParsedDiffGap, ParsedDiffHunk} from "../../../../utils/diffParser";
import {DiffLineRow} from "./DiffLineRow";

interface DiffHunkSectionProps {
    hunk: ParsedDiffHunk;
    gap?: ParsedDiffGap;
}

export const DiffHunkSection = memo(function DiffHunkSection({hunk, gap}: DiffHunkSectionProps) {
    return (
        <div>
            <div className="diff-hunk-header">{hunk.header}</div>
            {/* li index suffix breaks ties when duplicate line numbers appear in a hunk.
                The hunk line list is never reordered, so index-based keys are stable. */}
            {hunk.lines.map((line, li) => (
                <DiffLineRow key={`${hunk.header}:${line.oldLineNum ?? "n"}:${line.newLineNum ?? "n"}:${li}`} line={line}/>
            ))}
            {gap && (
                <div className="diff-expand-bar" title="Hidden context expansion is not available.">
                    Hidden context: {gap.hiddenLineCount} lines
                </div>
            )}
        </div>
    );
});
