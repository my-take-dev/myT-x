import {memo} from "react";
import type {ParsedDiffLine} from "../../../../utils/diffParser";

interface DiffLineRowProps {
    line: ParsedDiffLine;
}

export const DiffLineRow = memo(function DiffLineRow({line}: DiffLineRowProps) {
    const oldLineLabel = line.type !== "added" ? `old line ${line.oldLineNum}` : "old line";
    const newLineLabel = line.type !== "removed" ? `new line ${line.newLineNum}` : "new line";
    return (
        <div className={`diff-line ${line.type}`}>
            <span className="diff-line-number" aria-label={oldLineLabel}>
                {line.type !== "added" ? line.oldLineNum : ""}
            </span>
            <span className="diff-line-number" aria-label={newLineLabel}>
                {line.type !== "removed" ? line.newLineNum : ""}
            </span>
            <span className="diff-line-content">{line.content}</span>
        </div>
    );
});
