import type { CSSProperties } from "react";
import type { FlatNode } from "./fileTreeTypes";
import { formatFileSize } from "./treeUtils";

interface TreeNodeRowProps {
  node: FlatNode;
  style: CSSProperties;
  isSelected: boolean;
  onToggleDir: (path: string) => void;
  onSelectFile: (path: string) => void;
  onContextMenu: (e: React.MouseEvent, node: FlatNode) => void;
}

export function TreeNodeRow({ node, style, isSelected, onToggleDir, onSelectFile, onContextMenu }: TreeNodeRowProps) {
  const handleClick = () => {
    if (node.isDir) {
      onToggleDir(node.path);
    } else {
      onSelectFile(node.path);
    }
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    onContextMenu(e, node);
  };

  return (
    <div
      className={`tree-node-row${isSelected ? " selected" : ""}`}
      style={{ ...style, paddingLeft: `${8 + node.depth * 16}px` }}
      onClick={handleClick}
      onContextMenu={handleContextMenu}
    >
      {/* Expand/collapse arrow for directories */}
      <span className={`tree-node-arrow${node.isExpanded ? " expanded" : ""}`}>
        {node.isDir ? "\u25B6" : ""}
      </span>

      {/* File/folder icon */}
      <span className="tree-node-icon">
        {node.isDir ? "\uD83D\uDCC1" : "\uD83D\uDCC4"}
      </span>

      {/* Name */}
      <span className="tree-node-name">{node.name}</span>

      {/* Loading indicator or file size */}
      {node.isLoading ? (
        <span className="tree-node-loading">...</span>
      ) : !node.isDir && node.size > 0 ? (
        <span className="tree-node-size">{formatFileSize(node.size)}</span>
      ) : null}
    </div>
  );
}
