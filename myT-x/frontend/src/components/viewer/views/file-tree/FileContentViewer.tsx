import {useCallback, useEffect, useMemo, useRef, useState, type ReactNode} from "react";
import {FixedSizeList} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {useCopyPathNotice} from "../../../../hooks/useCopyPathNotice";
import {useShikiHighlight} from "../../../../hooks/useShikiHighlight";
import {detectLineEnding, splitLines} from "../../../../utils/textLines";
import {isMarkdownLang, pathToShikiLang} from "../../../../utils/shikiHighlighter";
import {makeScrollStableOuter} from "../shared/TreeOuter";
import type {FileContentResult} from "./fileTreeTypes";
import {MarkdownPreview} from "./MarkdownPreview";
import {formatFileSize} from "./treeUtils";
import {
    FILE_CONTENT_ROW_HEIGHT_FALLBACK,
    MAX_OVERSCAN_ROWS,
    MIN_BODY_VIEWPORT_ROWS,
    OVERSCAN_BUFFER,
} from "./fileContentConstants";
import {FileContentRow, type FileContentRowData} from "./FileContentRow";
import {FileContentHeader} from "./FileContentHeader";
import {useRowHeight} from "./useRowHeight";
import {useFileContentSelection} from "./useFileContentSelection";
import {
    canPreviewBinaryDocumentKind,
    canPreviewDocumentKind,
    getUncontrolledDefaultRenderModeForDocumentKind,
    type DocumentKind,
    type RenderMode,
} from "./documentTypes";

export interface FileContentViewerProps {
    readonly content: FileContentResult | null;
    readonly isLoading: boolean;
    readonly documentKind?: DocumentKind | null;
    readonly renderMode?: RenderMode;
    readonly canPreview?: boolean;
    readonly onRenderModeChange?: (mode: RenderMode) => void;
    readonly previewRenderer?: (content: FileContentResult, kind: DocumentKind) => ReactNode;
}

/**
 * Module-level factory call — must not be inside a render function (see makeScrollStableOuter).
 * overflowX: "scroll" keeps the horizontal scrollbar always present so viewport height
 * does not oscillate when near-bottom rows have wider content than current rows.
 */
const FileContentListOuter = makeScrollStableOuter({
    role: "list",
    ariaLabel: "File content",
    overflowX: "scroll",
});

export function FileContentViewer({
    content,
    isLoading,
    documentKind,
    renderMode,
    canPreview,
    onRenderModeChange,
    previewRenderer,
}: FileContentViewerProps) {
    const [internalRenderMode, setInternalRenderMode] = useState<RenderMode>("raw");
    const listBodyRef = useRef<HTMLDivElement>(null);
    // 12 rows keeps the viewer usable before the first ResizeObserver callback.
    // noiseThresholdPx: 1 suppresses ±1px RO churn that causes scroll jitter.
    const bodyHeight = useContainerHeight(
        listBodyRef,
        FILE_CONTENT_ROW_HEIGHT_FALLBACK * MIN_BODY_VIEWPORT_ROWS,
        {noiseThresholdPx: 1},
    );

    const {copyState: pathCopyState, copyPath} = useCopyPathNotice(content?.path, {
        logPrefix: "[DEBUG-file-content]",
    });

    // Computed early (before content null checks / early returns) so it can be used by
    // effects and callbacks that need to know whether the virtualized text body will render.
    // When content is null, this evaluates to false, which is the correct safe-side fallback
    // - no selection handlers or resize observers will activate. (checklist #92 safe fallback)
    const isTextBodyVisible = Boolean(!isLoading && content && !content.binary);

    // Detect markdown file using the authoritative extension map in shikiHighlighter.
    const defaultDocumentKind = useMemo<DocumentKind | null>(() => {
        if (!content?.path) return null;
        return isMarkdownLang(pathToShikiLang(content.path)) ? "markdown" : null;
    }, [content?.path]);

    const effectiveDocumentKind = documentKind ?? defaultDocumentKind;
    const isControlledRenderMode = renderMode !== undefined && onRenderModeChange !== undefined;
    const effectiveCanPreview = canPreview ?? canPreviewDocumentKind(effectiveDocumentKind);
    const effectiveRenderMode = isControlledRenderMode ? renderMode : internalRenderMode;
    const isPreviewMode = effectiveCanPreview && effectiveRenderMode === "preview";
    const shouldShowVirtualizedBody = isTextBodyVisible && !isPreviewMode;

    useEffect(() => {
        if (!isControlledRenderMode) {
            // Preserve the standalone FileContentViewer contract for uncontrolled callers.
            // FileTreeView controls render mode separately and opts previewable text documents into preview mode.
            setInternalRenderMode(getUncontrolledDefaultRenderModeForDocumentKind(effectiveDocumentKind));
        }
    }, [content?.path, effectiveDocumentKind, isControlledRenderMode]);

    useEffect(() => {
        if (!isControlledRenderMode && !effectiveCanPreview) {
            setInternalRenderMode("raw");
        }
    }, [effectiveCanPreview, isControlledRenderMode]);

    // Syntax highlighting for all code files (including .md in raw mode).
    // Skip highlighting when in markdown preview mode to avoid wasting resources.
    const {tokens, skipInfo} = useShikiHighlight(
        isPreviewMode ? undefined : (content?.content || undefined),
        isPreviewMode ? undefined : content?.path,
    );

    const highlightWarning = useMemo(() => {
        if (!skipInfo) return null;
        // Switch with exhaustive check so TypeScript catches any future additions
        // to HighlightSkipReason at compile time (checklist #142).
        const reason = skipInfo.reason;
        switch (reason) {
            case "size-limit":
                return `Syntax highlighting is intentionally disabled. Showing plain text because file size ${formatFileSize(skipInfo.actual)} exceeds ${formatFileSize(skipInfo.limit)}.`;
            case "line-count-limit":
                return `Syntax highlighting is intentionally disabled. Showing plain text because line count ${skipInfo.actual} exceeds limit ${skipInfo.limit}.`;
            case "line-length-limit":
                return `Syntax highlighting is intentionally disabled. Showing plain text because max line length ${skipInfo.actual} exceeds limit ${skipInfo.limit}.`;
            default:
                return "Syntax highlighting is intentionally disabled. Showing plain text.";
        }
    }, [skipInfo]);

    // Memoize lines to avoid re-splitting on every render.
    const lines = useMemo(() => {
        if (content?.content == null || content.content === "") return [];
        return splitLines(content.content);
    }, [content?.content]);
    // Memoize line ending detection to avoid recalculating on every render.
    const sourceLineEnding = useMemo(
        () => detectLineEnding(content?.content ?? ""),
        [content?.content],
    );

    const rowData = useMemo<FileContentRowData>(() => ({
        lines,
        tokens,
    }), [lines, tokens]);

    const rowHeight = useRowHeight(listBodyRef, shouldShowVirtualizedBody, bodyHeight);

    const {
        copySelectionNotice,
        handleBodyKeyDown,
        handleBodyMouseDown,
        handleBodyBlur,
        handleBodyCopy,
    } = useFileContentSelection({
        rawContent: content?.content,
        lines,
        sourceLineEnding,
        listBodyRef,
        shouldShowVirtualizedBody,
        resetKey: content?.path,
    });

    // Exclusive display: copySelectionNotice (ephemeral) takes priority over
    // highlightWarning (persistent). The copy notice auto-resets after
    // COPY_SELECTION_NOTICE_MS, allowing the highlight warning to reappear.
    const headerNotice = copySelectionNotice ?? highlightWarning;
    const headerNoticeClass = copySelectionNotice
        ? "file-content-copy-warning"
        : "file-content-highlight-warning";

    const handleTogglePreview = useCallback(() => {
        if (!effectiveCanPreview) {
            return;
        }
        const nextMode: RenderMode = isPreviewMode ? "raw" : "preview";
        if (isControlledRenderMode) {
            onRenderModeChange(nextMode);
            return;
        }
        setInternalRenderMode(nextMode);
    }, [effectiveCanPreview, isControlledRenderMode, isPreviewMode, onRenderModeChange]);

    const listViewportHeight = Math.max(bodyHeight, rowHeight, FILE_CONTENT_ROW_HEIGHT_FALLBACK);

    // Dynamically compute overscan to adapt to varying screen sizes and font configurations.
    // Large screens / small fonts need more overscan rows to keep selection anchors mounted;
    // small screens need fewer to avoid over-rendering. (review item S-05)
    // NOTE: rowHeight starts at FILE_CONTENT_ROW_HEIGHT_FALLBACK (20px), so the initial
    // overscan may be slightly over-estimated for large font sizes. This is acceptable because
    // the effect re-evaluates once rowHeight is measured from the actual DOM.
    const dynamicOverscan = rowHeight > 0
        ? Math.ceil(bodyHeight / rowHeight) + OVERSCAN_BUFFER
        : OVERSCAN_BUFFER;
    const overscanCount = Math.min(MAX_OVERSCAN_ROWS, dynamicOverscan);

    if (isLoading) {
        return <div className="file-content-empty">Loading...</div>;
    }

    if (!content) {
        return <div className="file-content-empty">Select a file to preview</div>;
    }

    if (content.binary) {
        if (!(canPreviewBinaryDocumentKind(effectiveDocumentKind) && isPreviewMode && previewRenderer)) {
            return <div className="file-content-binary">Binary file ({formatFileSize(content.size)})</div>;
        }
    }

    const previewContent = effectiveDocumentKind && isPreviewMode
        ? previewRenderer
            ? previewRenderer(content, effectiveDocumentKind)
            : <MarkdownPreview content={content.content}/>
        : null;

    return (
        <div className="file-content-viewer">
            <FileContentHeader
                path={content.path}
                pathCopyState={pathCopyState}
                onCopyPath={copyPath}
                canPreview={effectiveCanPreview}
                isPreviewMode={isPreviewMode}
                onTogglePreview={handleTogglePreview}
                size={content.size}
                truncated={content.truncated}
                headerNotice={headerNotice}
                headerNoticeClass={headerNoticeClass}
            />
            {previewContent ? (
                <div className="file-content-body" tabIndex={0}>
                    {previewContent}
                </div>
            ) : (
                <div
                    className="file-content-body file-content-body-virtualized"
                    ref={listBodyRef}
                    tabIndex={0}
                    onKeyDown={handleBodyKeyDown}
                    onMouseDown={handleBodyMouseDown}
                    onBlur={handleBodyBlur}
                    onCopy={handleBodyCopy}
                >
                    <FixedSizeList
                        className="file-content-list"
                        height={listViewportHeight}
                        itemCount={lines.length}
                        itemSize={rowHeight}
                        width="100%"
                        itemData={rowData}
                        outerElementType={FileContentListOuter}
                        // Dynamically computed: viewport rows + OVERSCAN_BUFFER.
                        // Virtual scroll unmounts rows outside the viewport + overscan window.
                        // Selection anchors (start/end nodes) must remain in the DOM for copy to work;
                        // dynamic overscan adapts to screen size and font, preventing the
                        // "non-rendered lines" copy warning without over-rendering on small screens.
                        overscanCount={overscanCount}
                    >
                        {FileContentRow}
                    </FixedSizeList>
                </div>
            )}
        </div>
    );
}
