import {memo, useCallback, useEffect, useMemo, useRef} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {makeScrollStableOuter} from "../shared/TreeOuter";
import type {SearchFileResult} from "./fileTreeTypes";

/** Scroll-stable outer element with listbox role for search results. */
const SearchResultsOuter = makeScrollStableOuter({role: "listbox", ariaLabel: "Search results"});

interface FileSearchPanelProps {
    readonly query: string;
    readonly onQueryChange: (q: string) => void;
    readonly results: readonly SearchFileResult[];
    readonly isSearching: boolean;
    readonly searchError: string | null;
    readonly selectedPath: string | null;
    readonly onSelectFile: (path: string) => void;
    readonly onOpenFile: (path: string) => void;
    readonly onClose: () => void;
}

/** Fixed row height for search results. */
const ROW_HEIGHT = 52;

interface RowData {
    readonly results: readonly SearchFileResult[];
    readonly selectedPath: string | null;
    readonly onSelectFile: (path: string) => void;
    readonly onOpenFile: (path: string) => void;
}

const SearchResultRow = memo(function SearchResultRow(
    {index, style, data}: ListChildComponentProps<RowData>,
) {
    const result = data.results[index];
    if (!result) return null;

    const isSelected = data.selectedPath === result.path;

    const handleClick = useCallback(() => {
        data.onSelectFile(result.path);
    }, [data, result.path]);

    const handleDoubleClick = useCallback(() => {
        data.onOpenFile(result.path);
    }, [data, result.path]);

    const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
        if (e.key === "Enter") {
            e.preventDefault();
            data.onOpenFile(result.path);
        }
    }, [data, result.path]);

    // Extract directory from path (everything before the last slash).
    const dir = result.path.includes("/")
        ? result.path.substring(0, result.path.lastIndexOf("/") + 1)
        : "";

    // Show first content line preview if available.
    const firstLine = result.content_lines.length > 0 ? result.content_lines[0] : null;

    return (
        <div
            className={`file-tree-search-result-row${isSelected ? " selected" : ""}`}
            style={style}
            onClick={handleClick}
            onDoubleClick={handleDoubleClick}
            onKeyDown={handleKeyDown}
            tabIndex={0}
            role="option"
            aria-selected={isSelected}
        >
            <div className="file-tree-search-result-main">
                <span className="file-tree-search-result-icon">📄</span>
                <span className="file-tree-search-result-name">{result.name}</span>
                {dir && <span className="file-tree-search-result-dir">{dir}</span>}
            </div>
            {firstLine && (
                <div className="file-tree-search-result-line">
                    <span className="file-tree-search-result-line-num">L{firstLine.line}</span>
                    <span className="file-tree-search-result-line-text">{firstLine.content}</span>
                </div>
            )}
        </div>
    );
}, (prev, next) => {
    if (prev.index !== next.index || prev.style !== next.style) return false;
    const prevData = prev.data;
    const nextData = next.data;
    const prevResult = prevData.results[prev.index];
    const nextResult = nextData.results[next.index];
    if (prevResult !== nextResult) return false;
    const wasSelected = prevResult && prevData.selectedPath === prevResult.path;
    const isSelected = nextResult && nextData.selectedPath === nextResult.path;
    if (wasSelected !== isSelected) return false;
    return prevData.onSelectFile === nextData.onSelectFile
        && prevData.onOpenFile === nextData.onOpenFile;
});

export function FileSearchPanel({
    query,
    onQueryChange,
    results,
    isSearching,
    searchError,
    selectedPath,
    onSelectFile,
    onOpenFile,
    onClose,
}: FileSearchPanelProps) {
    const inputRef = useRef<HTMLInputElement>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    // noiseThresholdPx: 1 suppresses ±1px ResizeObserver churn.
    const height = useContainerHeight(containerRef, ROW_HEIGHT, {noiseThresholdPx: 1});

    // Autofocus input on mount.
    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        onQueryChange(e.target.value);
    }, [onQueryChange]);

    const handleInputKeyDown = useCallback((e: React.KeyboardEvent) => {
        if (e.key === "Escape") {
            e.preventDefault();
            onClose();
        }
    }, [onClose]);

    const itemData = useMemo<RowData>(() => ({
        results,
        selectedPath,
        onSelectFile,
        onOpenFile,
    }), [results, selectedPath, onSelectFile, onOpenFile]);

    // Status text.
    let statusText: string;
    if (isSearching) {
        statusText = "検索中...";
    } else if (searchError) {
        statusText = searchError;
    } else if (query.trim() === "") {
        statusText = "";
    } else {
        statusText = `${results.length}件`;
    }

    return (
        <div className="file-tree-search-panel">
            <div className="file-tree-search-input-row">
                <input
                    ref={inputRef}
                    className="file-tree-search-input"
                    type="text"
                    value={query}
                    onChange={handleInputChange}
                    onKeyDown={handleInputKeyDown}
                    placeholder="Search files..."
                    spellCheck={false}
                    autoComplete="off"
                />
                <button
                    className="file-tree-search-close"
                    onClick={onClose}
                    title="Close search (Escape)"
                    type="button"
                >
                    ✕
                </button>
            </div>
            {statusText && (
                <div className={`file-tree-search-status${searchError ? " error" : ""}`}>
                    {statusText}
                </div>
            )}
            <div className="file-tree-search-results" ref={containerRef}>
                {results.length > 0 && height > 0 ? (
                    <FixedSizeList
                        height={height}
                        itemCount={results.length}
                        itemSize={ROW_HEIGHT}
                        width="100%"
                        itemData={itemData}
                        overscanCount={10}
                        outerElementType={SearchResultsOuter}
                    >
                        {SearchResultRow}
                    </FixedSizeList>
                ) : query.trim() !== "" && !isSearching && results.length === 0 && !searchError ? (
                    <div className="file-tree-search-empty">一致するファイルはありません</div>
                ) : null}
            </div>
        </div>
    );
}
