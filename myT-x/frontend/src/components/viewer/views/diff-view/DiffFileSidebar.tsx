import {memo, useMemo, useRef} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {makeTreeOuter} from "../shared/TreeOuter";
import {useVirtualizedTreeFocus} from "../shared/useVirtualizedTreeFocus";
import {TreeNodeRow} from "../file-tree/TreeNodeRow";
import type {FlatNode} from "../file-tree/fileTreeTypes";
import type {DiffTreeNode} from "./diffViewTypes";

interface DiffFileSidebarProps {
    flatNodes: readonly DiffTreeNode[];
    selectedPath: string | null;
    onToggleDir: (path: string) => void;
    onSelectFile: (path: string) => void;
}

interface RowData {
    flatNodes: readonly DiffTreeNode[];
    selectedPath: string | null;
    focusedIndex: number;
    focusIndex: (index: number) => void;
    findParentIndex: (index: number) => number;
    onToggleDir: (path: string) => void;
    onSelectFile: (path: string) => void;
}

const ROW_HEIGHT = 28;
const DiffTreeOuter = makeTreeOuter("Changed files");

// NOTE: --warning uses a CSS fallback (var(--fg-main)) because it is a supplementary
// variable not guaranteed in all themes. --git-staged, --danger, and --fg-main are
// core theme variables that are always defined.
function getStatusMeta(status: string): { label: string; color: string } {
    switch (status) {
        case "added":
        case "untracked":
            return {
                label: status,
                color: "var(--git-staged)",
            };
        case "deleted":
            return {
                label: "deleted",
                color: "var(--danger)",
            };
        case "renamed":
            return {
                label: "renamed",
                color: "var(--warning, var(--fg-main))",
            };
        default:
            return {
                label: "modified",
                color: "var(--fg-main)",
            };
    }
}

// Custom areEqual: focusedIndex is in itemData, so when it changes ALL rows
// re-render with default memo. This comparator only re-renders a row if its own
// selection state, focus state, or backing data actually changed.
const Row = memo(function Row({index, style, data}: ListChildComponentProps<RowData>) {
    const node = data.flatNodes[index];
    if (!node) return null;
    const isSelected = data.selectedPath === node.path;
    const isFocusable = data.focusedIndex === index;
    const statusMeta = !node.isDir ? getStatusMeta(node.file.status) : null;
    const treeRowNode: FlatNode = node.isDir
        ? {
            path: node.path,
            name: node.name,
            isDir: true,
            depth: node.depth,
            hasChildren: true,
            isExpanded: node.isExpanded,
            isLoading: false,
        }
        : {
            path: node.path,
            name: node.name,
            isDir: false,
            depth: node.depth,
            size: 0,
        };
    const nodeAriaLabel = node.isDir
        ? `Directory ${node.name}`
        : `File ${node.name}, ${statusMeta?.label ?? "modified"}, +${node.file.additions}, -${node.file.deletions}`;

    return (
        <TreeNodeRow
            node={treeRowNode}
            style={style}
            isSelected={isSelected}
            isFocusable={isFocusable}
            onToggleDir={data.onToggleDir}
            onSelectFile={data.onSelectFile}
            onFocusIndex={data.focusIndex}
            index={index}
            findParentIndex={data.findParentIndex}
            ariaLabel={nodeAriaLabel}
            nameStyle={!node.isDir ? {color: statusMeta?.color ?? "var(--fg-main)"} : undefined}
            renderIcon={() => null}
            renderExtra={() => node.isDir ? null : (
                <span className="diff-tree-stats">
                    {node.file.additions > 0 && (
                        <span className="diff-tree-additions">+{node.file.additions}</span>
                    )}
                    {node.file.deletions > 0 && (
                        <span className="diff-tree-deletions"> -{node.file.deletions}</span>
                    )}
                </span>
            )}
        />
    );
}, (prev, next) => {
    if (prev.index !== next.index || prev.style !== next.style) return false;
    const pd = prev.data;
    const nd = next.data;
    // Only re-render if THIS row's backing node changed.
    const prevNode = pd.flatNodes[prev.index];
    const nextNode = nd.flatNodes[next.index];
    if (prevNode !== nextNode) return false;
    // Only re-render if THIS row's selection state changed
    const wasSelected = prevNode && pd.selectedPath === prevNode.path;
    const isSelected = nextNode && nd.selectedPath === nextNode.path;
    if (wasSelected !== isSelected) return false;
    // Only re-render if THIS row's focus state changed (was focused <-> not focused)
    const wasFocused = pd.focusedIndex === prev.index;
    const isFocused = nd.focusedIndex === next.index;
    if (wasFocused !== isFocused) return false;
    return pd.onToggleDir === nd.onToggleDir &&
        pd.onSelectFile === nd.onSelectFile &&
        pd.focusIndex === nd.focusIndex &&
        pd.findParentIndex === nd.findParentIndex;
});

export function DiffFileSidebar({flatNodes, selectedPath, onToggleDir, onSelectFile}: DiffFileSidebarProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<FixedSizeList<RowData> | null>(null);
    // noiseThresholdPx: 1 suppresses ±1px ResizeObserver churn that causes scroll jitter.
    const height = useContainerHeight(containerRef, ROW_HEIGHT, {noiseThresholdPx: 1});
    const {focusedIndex, focusIndex, findParentIndex} = useVirtualizedTreeFocus(flatNodes, selectedPath, listRef);

    const itemData = useMemo<RowData>(() => ({
        flatNodes,
        selectedPath,
        focusedIndex,
        focusIndex,
        findParentIndex,
        onToggleDir,
        onSelectFile,
    }), [flatNodes, selectedPath, focusedIndex, focusIndex, findParentIndex, onToggleDir, onSelectFile]);

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
                    outerElementType={DiffTreeOuter}
                >
                    {Row}
                </FixedSizeList>
            ) : null}
        </div>
    );
}
