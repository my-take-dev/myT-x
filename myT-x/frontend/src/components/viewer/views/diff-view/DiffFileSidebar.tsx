import {memo, useMemo, useRef} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {makeTreeOuter} from "../shared/TreeOuter";
import {useVirtualizedTreeFocus} from "../shared/useVirtualizedTreeFocus";
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
    const nodeAriaLabel = node.isDir
        ? `Directory ${node.name}`
        : `File ${node.name}, ${statusMeta?.label ?? "modified"}, +${node.file.additions}, -${node.file.deletions}`;

    const handleClick = () => {
        data.focusIndex(index);
        if (node.isDir) {
            data.onToggleDir(node.path);
        } else {
            data.onSelectFile(node.path);
        }
    };

    return (
        <div
            role="treeitem"
            tabIndex={isFocusable ? 0 : -1}
            aria-selected={isSelected}
            aria-expanded={node.isDir ? node.isExpanded : undefined}
            aria-label={nodeAriaLabel}
            className={`tree-node-row${isSelected ? " selected" : ""}`}
            style={{...style, paddingLeft: 8 + node.depth * 16}}
            onClick={handleClick}
            onFocus={() => data.focusIndex(index)}
            onKeyDown={(e) => {
                switch (e.key) {
                    case "Enter":
                    case " ": {
                        e.preventDefault();
                        handleClick();
                        return;
                    }
                    case "ArrowDown": {
                        e.preventDefault();
                        data.focusIndex(index + 1);
                        return;
                    }
                    case "ArrowUp": {
                        e.preventDefault();
                        data.focusIndex(index - 1);
                        return;
                    }
                    case "ArrowRight": {
                        // Always prevent default to claim ArrowRight as a tree-navigation key,
                        // avoiding unwanted horizontal scroll even when no action fires.
                        e.preventDefault();
                        if (node.isDir && !node.isExpanded) {
                            data.onToggleDir(node.path);
                        }
                        return;
                    }
                    case "ArrowLeft": {
                        if (node.isDir && node.isExpanded) {
                            e.preventDefault();
                            data.onToggleDir(node.path);
                            return;
                        }
                        const parentIndex = data.findParentIndex(index);
                        if (parentIndex >= 0) {
                            e.preventDefault();
                            data.focusIndex(parentIndex);
                        }
                        return;
                    }
                }
            }}
        >
            <span className={`tree-node-arrow${node.isDir && node.isExpanded ? " expanded" : ""}`}>
                {node.isDir ? "\u25B6" : ""}
            </span>
            <span
                className="tree-node-name"
                style={!node.isDir ? {color: statusMeta?.color ?? "var(--fg-main)"} : undefined}
            >
                {node.name}
            </span>
            {!node.isDir && (
                <span className="diff-tree-stats">
                    {node.file.additions > 0 && (
                        <span className="diff-tree-additions">+{node.file.additions}</span>
                    )}
                    {node.file.deletions > 0 && (
                        <span className="diff-tree-deletions"> -{node.file.deletions}</span>
                    )}
                </span>
            )}
        </div>
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
    const height = useContainerHeight(containerRef);
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
