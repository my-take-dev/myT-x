import type {CopyNoticeState} from "../../../../utils/clipboardUtils";
import {CopyPathButton} from "../shared/CopyPathButton";
import {formatFileSize} from "./treeUtils";

interface FileContentHeaderProps {
    readonly path: string;
    readonly pathCopyState: CopyNoticeState;
    readonly onCopyPath: () => void;
    readonly isMarkdownFile: boolean;
    readonly isPreviewMode: boolean;
    readonly onTogglePreview: () => void;
    readonly size: number;
    readonly truncated: boolean;
    readonly headerNotice: string | null;
    readonly headerNoticeClass: string;
}

function SourceCodeIcon() {
    return (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
            <path d="M5.5 3L2 8l3.5 5" stroke="currentColor" strokeWidth="1.5"
                  strokeLinecap="round" strokeLinejoin="round"/>
            <path d="M10.5 3L14 8l-3.5 5" stroke="currentColor" strokeWidth="1.5"
                  strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
    );
}

function PreviewIcon() {
    return (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
            <path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z"
                  stroke="currentColor" strokeWidth="1.5"/>
            <circle cx="8" cy="8" r="2" stroke="currentColor" strokeWidth="1.5"/>
        </svg>
    );
}

export function FileContentHeader({
    path,
    pathCopyState,
    onCopyPath,
    isMarkdownFile,
    isPreviewMode,
    onTogglePreview,
    size,
    truncated,
    headerNotice,
    headerNoticeClass,
}: FileContentHeaderProps) {
    return (
        <div className="file-content-header">
            <span className="file-content-path">{path}</span>
            <CopyPathButton state={pathCopyState} onClick={onCopyPath}/>
            {isMarkdownFile && (
                <button
                    type="button"
                    className={`file-content-toggle-preview${isPreviewMode ? " active" : ""}`}
                    onClick={onTogglePreview}
                    title={isPreviewMode ? "Show source" : "Show preview"}
                    aria-label={isPreviewMode ? "Show source" : "Show preview"}
                    aria-pressed={isPreviewMode}
                >
                    {isPreviewMode ? <SourceCodeIcon/> : <PreviewIcon/>}
                </button>
            )}
            <span className="file-content-size">
                {formatFileSize(size)}
                {truncated ? " (truncated)" : ""}
            </span>
            {headerNotice && (
                <span className={headerNoticeClass} title={headerNotice}>{headerNotice}</span>
            )}
        </div>
    );
}
