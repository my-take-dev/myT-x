import {forwardRef, memo, useEffect, useMemo, useRef, useState, type HTMLAttributes} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import type {DiffTreeNode} from "./diffViewTypes";

// Custom outer element to apply ARIA tree role to the FixedSizeList container.
const TreeOuter = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
    function TreeOuter(props, ref) {
        return <div ref={ref} role="tree" aria-label="Changed files" {...props} />;
    },
);

interface DiffFileSidebarProps {
    flatNodes: DiffTreeNode[];
    selectedPath: string | null;
    onToggleDir: (path: string) => void;
    onSelectFile: (path: string) => void;
}

interface RowData {
    flatNodes: DiffTreeNode[];
    selectedPath: string | null;
    onToggleDir: (path: string) => void;
    onSelectFile: (path: string) => void;
}

const ROW_HEIGHT = 28;

function statusColor(status: string): string {
    switch (status) {
        case "added":
        case "untracked":
            return "var(--git-staged)";
        case "deleted":
            return "var(--danger)";
        case "renamed":
            return "var(--warning, var(--fg-main))";
        default:
            return "var(--fg-main)";
    }
}

const Row = memo(function Row({index, style, data}: ListChildComponentProps<RowData>) {
    const node = data.flatNodes[index];
    const isSelected = data.selectedPath === node.path;

    const handleClick = () => {
        if (node.isDir) {
            data.onToggleDir(node.path);
        } else {
            data.onSelectFile(node.path);
        }
    };

    return (
        <div
            role="treeitem"
            tabIndex={0}
            aria-selected={isSelected}
            aria-expanded={node.isDir ? node.isExpanded : undefined}
            className={`tree-node-row${isSelected ? " selected" : ""}`}
            style={{...style, paddingLeft: 8 + node.depth * 16}}
            onClick={handleClick}
            onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleClick();
                }
            }}
        >
            <span className={`tree-node-arrow${node.isDir && node.isExpanded ? " expanded" : ""}`}>
                {node.isDir ? "\u25B6" : ""}
            </span>
            <span
                className="tree-node-name"
                style={!node.isDir ? {color: statusColor(node.file?.status ?? "modified")} : undefined}
            >
                {node.name}
            </span>
            {!node.isDir && node.file && (
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
});

export function DiffFileSidebar({flatNodes, selectedPath, onToggleDir, onSelectFile}: DiffFileSidebarProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [height, setHeight] = useState(0);

    useEffect(() => {
        const el = containerRef.current;
        if (!el) return;
        setHeight(Math.max(0, Math.floor(el.getBoundingClientRect().height)));
        const ro = new ResizeObserver((entries) => {
            for (const entry of entries) {
                setHeight(Math.max(0, Math.floor(entry.contentRect.height)));
            }
        });
        ro.observe(el);
        return () => ro.disconnect();
    }, []);

    const itemData = useMemo<RowData>(() => ({
        flatNodes,
        selectedPath,
        onToggleDir,
        onSelectFile,
    }), [flatNodes, selectedPath, onToggleDir, onSelectFile]);

    return (
        <div className="file-tree-sidebar" ref={containerRef}>
            {height > 0 ? (
                <FixedSizeList
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
    );
}
