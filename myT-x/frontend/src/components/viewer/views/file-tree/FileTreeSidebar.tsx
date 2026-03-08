import {memo, useCallback, useMemo, useRef, useState} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {TreeOuter} from "../shared/TreeOuter";
import {useVirtualizedTreeFocus} from "../shared/useVirtualizedTreeFocus";
import type {FlatNode} from "./fileTreeTypes";
import {FileTreeContextMenu} from "./FileTreeContextMenu";
import {TreeNodeRow} from "./TreeNodeRow";

interface FileTreeSidebarProps {
    readonly flatNodes: readonly FlatNode[];
    readonly selectedPath: string | null;
    readonly onToggleDir: (path: string) => void;
    readonly onSelectFile: (path: string) => void;
}

interface RowData {
    readonly flatNodes: readonly FlatNode[];
    readonly selectedPath: string | null;
    readonly focusedIndex: number;
    readonly focusIndex: (index: number) => void;
    readonly findParentIndex: (index: number) => number;
    readonly onToggleDir: (path: string) => void;
    readonly onSelectFile: (path: string) => void;
    readonly onContextMenu: (e: React.MouseEvent, node: FlatNode) => void;
}

interface ContextMenuState {
    x: number;
    y: number;
    node: FlatNode;
}

const ROW_HEIGHT = 28;

const Row = memo(function Row({index, style, data}: ListChildComponentProps<RowData>) {
    const node = data.flatNodes[index];
    if (!node) return null;
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
    if (prev.index !== next.index || prev.style !== next.style) return false;
    const prevData = prev.data;
    const nextData = next.data;
    const prevNode = prevData.flatNodes[prev.index];
    const nextNode = nextData.flatNodes[next.index];
    if (prevNode !== nextNode) return false;

    const wasSelected = prevNode && prevData.selectedPath === prevNode.path;
    const isSelected = nextNode && nextData.selectedPath === nextNode.path;
    if (wasSelected !== isSelected) return false;

    const wasFocused = prevData.focusedIndex === prev.index;
    const isFocused = nextData.focusedIndex === next.index;
    if (wasFocused !== isFocused) return false;

    return prevData.onToggleDir === nextData.onToggleDir
        && prevData.onSelectFile === nextData.onSelectFile
        && prevData.onContextMenu === nextData.onContextMenu
        && prevData.focusIndex === nextData.focusIndex
        && prevData.findParentIndex === nextData.findParentIndex;
});

export function FileTreeSidebar({flatNodes, selectedPath, onToggleDir, onSelectFile}: FileTreeSidebarProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<FixedSizeList<RowData> | null>(null);
    // noiseThresholdPx: 1 suppresses ±1px ResizeObserver churn that causes scroll jitter.
    const height = useContainerHeight(containerRef, ROW_HEIGHT, {noiseThresholdPx: 1});
    const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
    const {focusedIndex, focusIndex, findParentIndex} = useVirtualizedTreeFocus(flatNodes, selectedPath, listRef);

    const handleContextMenu = useCallback((e: React.MouseEvent, node: FlatNode) => {
        setContextMenu({x: e.clientX, y: e.clientY, node});
    }, []);

    const handleCloseContextMenu = useCallback(() => {
        setContextMenu(null);
    }, []);

    const itemData = useMemo<RowData>(() => ({
        flatNodes,
        selectedPath,
        focusedIndex,
        focusIndex,
        findParentIndex,
        onToggleDir,
        onSelectFile,
        onContextMenu: handleContextMenu,
    }), [flatNodes, selectedPath, focusedIndex, focusIndex, findParentIndex, onToggleDir, onSelectFile, handleContextMenu]);

    return (
        <div className="file-tree-sidebar" ref={containerRef}>
            {/* NOTE: height starts at 0 until ResizeObserver reports; guard prevents empty FixedSizeList render. */}
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
            {contextMenu && (
                <FileTreeContextMenu
                    x={contextMenu.x}
                    y={contextMenu.y}
                    node={contextMenu.node}
                    onClose={handleCloseContextMenu}
                />
            )}
        </div>
    );
}
