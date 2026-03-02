import {useEffect, useMemo, useRef} from "react";
import {useCopyPathNotice} from "../../../../hooks/useCopyPathNotice";
import {parseSingleFileDiff, type ParsedDiffGap} from "../../../../utils/diffParser";
import {DiffHunkSection} from "../shared/DiffHunkSection";
import {CopyPathButton} from "../shared/CopyPathButton";
import type {WorkingDiffFile} from "./diffViewTypes";

interface DiffContentViewerProps {
    file: WorkingDiffFile | null;
}

function buildDiffPathLabel(file: WorkingDiffFile): string {
    const isRename = file.status === "renamed" && file.old_path && file.old_path !== file.path;
    return isRename ? `${file.old_path} -> ${file.path}` : file.path;
}

export function DiffContentViewer({file}: DiffContentViewerProps) {
    const {copyState: pathCopyState, copyPath} = useCopyPathNotice(file?.path, {
        logPrefix: "[diff-content]",
    });

    const parsed = useMemo(() => {
        if (!file) return {status: "success" as const, hunks: [], gaps: new Map<number, ParsedDiffGap>(), fileCount: 0};
        return parseSingleFileDiff(file.diff);
    }, [file?.diff]);
    // Primitive dependency for the multi-file warning useEffect below.
    const fileCountForDep = parsed.status === "success" ? parsed.fileCount : 0;

    // Warn if multi-file diff is passed to a single-file viewer — logged in all
    // environments so silent degradation ("first file only") is detectable.
    const warnedDiffRef = useRef<string | null>(null);
    useEffect(() => {
        if (!file?.diff) return;
        if (warnedDiffRef.current === file.diff) return;
        warnedDiffRef.current = file.diff;
        if (parsed.status === "success" && fileCountForDep > 1) {
            if (import.meta.env.DEV) {
                console.warn("[DEBUG-diff] DiffContentViewer received multi-file diff (%d files), showing first only", fileCountForDep);
            } else {
                console.warn("[diff-content] multi-file diff received (%d files), showing first only", fileCountForDep);
            }
        }
    }, [file?.diff, parsed.status, fileCountForDep]);

    if (!file) {
        return <div className="diff-content-empty">Select a file to view diff</div>;
    }

    return (
        <div className="diff-content-viewer">
            <div className="diff-content-header">
                <span className="diff-content-path">{buildDiffPathLabel(file)}</span>
                <CopyPathButton state={pathCopyState} onClick={copyPath}/>
                <span className="diff-header-stats">
                    {file.additions > 0 && <span className="diff-tree-additions">+{file.additions}</span>}
                    {file.deletions > 0 && <span className="diff-tree-deletions"> -{file.deletions}</span>}
                </span>
            </div>
            <div className="diff-content-body">
                <div className="diff-viewer">
                    {parsed.status === "error" && (
                        <div className="diff-content-empty">{parsed.message}</div>
                    )}
                    {parsed.status === "success" && parsed.hunks.length === 0 && (
                        <div className="diff-content-empty">
                            No changes in this file
                        </div>
                    )}
                    {parsed.status === "success" && parsed.hunks.map((hunk, hi) => (
                        <DiffHunkSection key={`${hunk.header}:${hi}`} hunk={hunk} gap={parsed.gaps.get(hi)}/>
                    ))}
                </div>
            </div>
        </div>
    );
}
