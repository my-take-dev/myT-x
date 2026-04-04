import {memo, type MouseEvent as ReactMouseEvent, useCallback, useMemo, useRef, useState} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {TreeOuter} from "../shared/TreeOuter";
import {useVirtualizedTreeFocus} from "../shared/useVirtualizedTreeFocus";
import type {FlatNode} from "../file-tree/fileTreeTypes";
import {TreeNodeRow} from "../file-tree/TreeNodeRow";
import {EditorContextMenu} from "./EditorContextMenu";
import {parentDirOf} from "./editorPathUtils";

interface EditorFileTreeProps {
    readonly flatNodes: readonly FlatNode[];
    readonly isRefreshing: boolean;
    readonly selectedPath: string | null;
    readonly onRefresh: () => void;
    readonly onRequestCreateDirectory: (parentDir: string) => void;
    readonly onRequestCreateFile: (parentDir: string) => void;
    readonly onRequestDelete: (node: FlatNode) => void;
    readonly onRequestRename: (node: FlatNode) => void;
    readonly onSearchOpen: () => void;
    readonly onSelectFile: (path: string) => void;
    readonly onToggleDir: (path: string) => void;
}

interface RowData {
    readonly findParentIndex: (index: number) => number;
    readonly flatNodes: readonly FlatNode[];
    readonly focusIndex: (index: number) => void;
    readonly focusedIndex: number;
    readonly onContextMenu: (event: ReactMouseEvent, node: FlatNode) => void;
    readonly onSelectFile: (path: string) => void;
    readonly onToggleDir: (path: string) => void;
    readonly selectedPath: string | null;
}

interface ContextMenuState {
    readonly node: FlatNode;
    readonly x: number;
    readonly y: number;
}

const ROW_HEIGHT = 28;

const Row = memo(function Row({data, index, style}: ListChildComponentProps<RowData>) {
    const node = data.flatNodes[index];
    if (!node) {
        return null;
    }

    return (
        <TreeNodeRow
            node={node}
            style={style}
            isSelected={data.selectedPath === node.path}
            isFocusable={data.focusedIndex === index}
            onToggleDir={data.onToggleDir}
            onSelectFile={data.onSelectFile}
            onContextMenu={data.onContextMenu}
            onFocusIndex={data.focusIndex}
            index={index}
            findParentIndex={data.findParentIndex}
        />
    );
}, (prev, next) => {
    if (prev.index !== next.index || prev.style !== next.style) {
        return false;
    }

    const prevData = prev.data;
    const nextData = next.data;
    const prevNode = prevData.flatNodes[prev.index];
    const nextNode = nextData.flatNodes[next.index];
    if (prevNode !== nextNode) {
        return false;
    }

    const wasSelected = prevNode && prevData.selectedPath === prevNode.path;
    const isSelected = nextNode && nextData.selectedPath === nextNode.path;
    if (wasSelected !== isSelected) {
        return false;
    }

    const wasFocused = prevData.focusedIndex === prev.index;
    const isFocused = nextData.focusedIndex === next.index;
    if (wasFocused !== isFocused) {
        return false;
    }

    return prevData.onToggleDir === nextData.onToggleDir
        && prevData.onSelectFile === nextData.onSelectFile
        && prevData.onContextMenu === nextData.onContextMenu
        && prevData.focusIndex === nextData.focusIndex
        && prevData.findParentIndex === nextData.findParentIndex;
});

export function EditorFileTree({
                                   flatNodes,
                                   isRefreshing,
                                   selectedPath,
                                   onRefresh,
                                   onRequestCreateDirectory,
                                   onRequestCreateFile,
                                   onRequestDelete,
                                   onRequestRename,
                                   onSearchOpen,
                                   onSelectFile,
                                   onToggleDir,
                               }: EditorFileTreeProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<FixedSizeList<RowData> | null>(null);
    const height = useContainerHeight(containerRef, ROW_HEIGHT, {noiseThresholdPx: 1});
    const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
    const {focusedIndex, focusIndex, findParentIndex} = useVirtualizedTreeFocus(flatNodes, selectedPath, listRef);

    const handleContextMenu = useCallback((event: ReactMouseEvent, node: FlatNode) => {
        setContextMenu({node, x: event.clientX, y: event.clientY});
    }, []);

    const closeContextMenu = useCallback(() => {
        setContextMenu(null);
    }, []);

    const itemData = useMemo<RowData>(() => ({
        findParentIndex,
        flatNodes,
        focusIndex,
        focusedIndex,
        onContextMenu: handleContextMenu,
        onSelectFile,
        onToggleDir,
        selectedPath,
    }), [findParentIndex, flatNodes, focusIndex, focusedIndex, handleContextMenu, onSelectFile, onToggleDir, selectedPath]);

    const contextTargetDir = contextMenu?.node.isDir ? contextMenu.node.path : parentDirOf(contextMenu?.node.path ?? "");

    return (
        <div className="editor-file-tree">
            <div className="editor-file-tree-header">
                <button
                    type="button"
                    className="editor-file-tree-action"
                    title={isRefreshing ? "Refreshing tree" : "Refresh tree"}
                    aria-label={isRefreshing ? "Refreshing tree" : "Refresh tree"}
                    aria-busy={isRefreshing}
                    disabled={isRefreshing}
                    onClick={onRefresh}
                >
                    ↻
                </button>
                <button
                    type="button"
                    className="editor-file-tree-action"
                    title="Create file at root"
                    aria-label="Create file at root"
                    onClick={() => onRequestCreateFile("")}
                >
                    +F
                </button>
                <button
                    type="button"
                    className="editor-file-tree-action"
                    title="Create folder at root"
                    aria-label="Create folder at root"
                    onClick={() => onRequestCreateDirectory("")}
                >
                    +D
                </button>
                <button
                    type="button"
                    className="editor-file-tree-action"
                    title="Search files"
                    aria-label="Search files"
                    onClick={onSearchOpen}
                >
                    🔍
                </button>
                {isRefreshing && (
                    <span className="editor-file-tree-status" aria-live="polite">
                        Refreshing...
                    </span>
                )}
            </div>
            <div className="editor-file-tree-list" ref={containerRef}>
                {height > 0 ? (
                    <FixedSizeList
                        ref={listRef}
                        height={height}
                        itemCount={flatNodes.length}
                        itemSize={ROW_HEIGHT}
                        width="100%"
                        itemData={itemData}
                        overscanCount={10}
                        outerElementType={TreeOuter}
                    >
                        {Row}
                    </FixedSizeList>
                ) : null}
            </div>
            {contextMenu && (
                <EditorContextMenu
                    node={contextMenu.node}
                    x={contextMenu.x}
                    y={contextMenu.y}
                    onClose={closeContextMenu}
                    onCreateFile={() => onRequestCreateFile(contextTargetDir)}
                    onCreateDirectory={() => onRequestCreateDirectory(contextTargetDir)}
                    onRename={() => onRequestRename(contextMenu.node)}
                    onDelete={() => onRequestDelete(contextMenu.node)}
                />
            )}
        </div>
    );
}
