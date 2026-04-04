import {memo} from "react";
import type {CSSProperties, KeyboardEvent, MouseEvent, ReactNode} from "react";
import {ChevronIcon} from "../../icons/ChevronIcon";
import type {FlatNode} from "./fileTreeTypes";
import {formatFileSize} from "./treeUtils";

interface TreeNodeRowProps {
    node: FlatNode;
    style: CSSProperties;
    isSelected: boolean;
    isFocusable: boolean;
    onToggleDir: (path: string) => void;
    onSelectFile: (path: string) => void;
    onContextMenu?: (e: MouseEvent, node: FlatNode) => void;
    onFocusIndex: (index: number) => void;
    index: number;
    findParentIndex: (index: number) => number;
    ariaLabel?: string;
    nameStyle?: CSSProperties;
    renderExtra?: (node: FlatNode) => ReactNode;
    renderIcon?: (node: FlatNode) => ReactNode;
}

export const TreeNodeRow = memo(function TreeNodeRow({
                                                         node,
                                                         style,
                                                         isSelected,
                                                         isFocusable,
                                                         onToggleDir,
                                                         onSelectFile,
                                                         onContextMenu,
                                                         onFocusIndex,
                                                         index,
                                                         findParentIndex,
                                                         ariaLabel,
                                                         nameStyle,
                                                         renderExtra,
                                                         renderIcon,
                                                     }: TreeNodeRowProps) {
    const hasChildren = node.isDir ? (node.hasChildren ?? true) : false;

    const handleClick = () => {
        onFocusIndex(index);
        if (node.isDir) {
            if (!hasChildren) {
                return;
            }
            onToggleDir(node.path);
        } else {
            onSelectFile(node.path);
        }
    };

    const handleContextMenu = (e: MouseEvent) => {
        if (!onContextMenu) {
            return;
        }
        e.preventDefault();
        onContextMenu(e, node);
    };

    const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
        switch (e.key) {
            case "Enter":
            case " ": {
                e.preventDefault();
                handleClick();
                return;
            }
            case "ArrowDown": {
                e.preventDefault();
                onFocusIndex(index + 1);
                return;
            }
            case "ArrowUp": {
                e.preventDefault();
                onFocusIndex(index - 1);
                return;
            }
            case "ArrowRight": {
                // Always prevent default to claim ArrowRight as a tree-navigation key,
                // avoiding unwanted horizontal scroll even when no action fires.
                e.preventDefault();
                if (node.isDir && hasChildren && !node.isExpanded) {
                    onToggleDir(node.path);
                }
                return;
            }
            case "ArrowLeft": {
                if (node.isDir && node.isExpanded) {
                    e.preventDefault();
                    onToggleDir(node.path);
                    return;
                }
                const parentIndex = findParentIndex(index);
                if (parentIndex >= 0) {
                    e.preventDefault();
                    onFocusIndex(parentIndex);
                }
                return;
            }
        }
    };

    return (
        <div
            role="treeitem"
            tabIndex={isFocusable ? 0 : -1}
            aria-selected={isSelected}
            aria-expanded={node.isDir ? node.isExpanded : undefined}
            aria-label={ariaLabel}
            className={`tree-node-row${isSelected ? " selected" : ""}`}
            style={{...style, paddingLeft: 8 + node.depth * 16}}
            onClick={handleClick}
            onFocus={() => onFocusIndex(index)}
            onKeyDown={handleKeyDown}
            onContextMenu={onContextMenu ? handleContextMenu : undefined}
        >
            <span className={`tree-node-arrow${node.isDir && node.isExpanded ? " expanded" : ""}`}>
                {node.isDir && hasChildren ? <ChevronIcon /> : null}
            </span>

            {renderIcon !== undefined ? (
                renderIcon(node)
            ) : (
                <span className="tree-node-icon">
                    {node.isDir ? "\uD83D\uDCC1" : "\uD83D\uDCC4"}
                </span>
            )}

            <span className="tree-node-name" style={nameStyle}>{node.name}</span>

            {renderExtra !== undefined
                ? renderExtra(node)
                : node.isDir && node.isLoading ? (
                    <span className="tree-node-loading">...</span>
                ) : !node.isDir && (node.size ?? 0) > 0 ? (
                    <span className="tree-node-size">{formatFileSize(node.size ?? 0)}</span>
                ) : null}
        </div>
    );
});
