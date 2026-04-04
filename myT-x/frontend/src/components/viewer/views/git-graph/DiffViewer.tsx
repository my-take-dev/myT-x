import {memo, useMemo, useRef, useState} from "react";
import {computeHunkGaps, diffHeaderToFilePath, parseDiffFiles, type ParsedDiffFile} from "../../../../utils/diffParser";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {ChevronIcon} from "../../icons/ChevronIcon";
import {DiffHunkSection} from "../shared/DiffHunkSection";

/** Collapse all diff sections when file count exceeds this threshold to reduce initial render cost. */
export const DIFF_COLLAPSE_THRESHOLD = 10;

interface DiffViewerProps {
    diff: string;
}

const DiffFileSection = memo(function DiffFileSection({
                                                          file,
                                                          initialCollapsed
                                                      }: {
    file: ParsedDiffFile;
    initialCollapsed: boolean;
}) {
    const prevInitialCollapsedRef = useRef(initialCollapsed);
    const [collapsed, setCollapsed] = useState(initialCollapsed);

    // Adjust collapsed during render when initialCollapsed changes (e.g., file count
    // crosses DIFF_COLLAPSE_THRESHOLD). React batches the setState into the current
    // render cycle, avoiding the double-render that useEffect would cause.
    // This intentionally overrides any manual toggle — when the parent changes
    // initialCollapsed, the new value takes precedence.
    if (prevInitialCollapsedRef.current !== initialCollapsed) {
        prevInitialCollapsedRef.current = initialCollapsed;
        setCollapsed(initialCollapsed);
    }

    const filePath = diffHeaderToFilePath(file.header);
    const gaps = useMemo(() => computeHunkGaps(file.hunks), [file.hunks]);

    return (
        <div>
            <button type="button" className="diff-file-header" onClick={() => setCollapsed(!collapsed)}
                    aria-expanded={!collapsed}>
                <span className={`diff-file-toggle${collapsed ? "" : " expanded"}`}>
                    <ChevronIcon size={8} />
                </span>
                <span>{filePath}</span>
            </button>
            {!collapsed && file.hunks.map((hunk, hi) => (
                <DiffHunkSection key={`${hunk.header}:${hi}`} hunk={hunk} gap={gaps.get(hi)}/>
            ))}
        </div>
    );
});

export function DiffViewer({diff}: DiffViewerProps) {
    const {files, parseError} = useMemo<{ files: ParsedDiffFile[]; parseError: string | null }>(() => {
        try {
            return {files: parseDiffFiles(diff), parseError: null};
        } catch (err: unknown) {
            console.error("[diff-viewer] parseDiffFiles failed:", err);
            return {files: [], parseError: toErrorMessage(err, "Failed to parse diff.")};
        }
    }, [diff]);

    if (parseError) {
        return <div className="viewer-message">{parseError}</div>;
    }

    if (files.length === 0) {
        return <div className="viewer-message">No diff available</div>;
    }

    return (
        <div className="diff-viewer">
            {/* NOTE: file.header is null only when the diff starts with a @@ hunk line
                without a preceding "diff --git" header (e.g., standalone hunk input).
                The index-based fallback key is acceptable here because the list is not
                reordered — it's rendered once per commit selection. */}
            {files.map((file, index) => (
                <DiffFileSection
                    key={file.header ?? `unnamed-${index}`}
                    file={file}
                    initialCollapsed={files.length > DIFF_COLLAPSE_THRESHOLD}
                />
            ))}
        </div>
    );
}
