import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FixedSizeList, type ListChildComponentProps } from "react-window";
import type { FlatNode } from "./fileTreeTypes";
import { FileTreeContextMenu } from "./FileTreeContextMenu";
import { TreeNodeRow } from "./TreeNodeRow";

interface FileTreeSidebarProps {
  flatNodes: FlatNode[];
  selectedPath: string | null;
  onToggleDir: (path: string) => void;
  onSelectFile: (path: string) => void;
}

interface RowData {
  flatNodes: FlatNode[];
  selectedPath: string | null;
  onToggleDir: (path: string) => void;
  onSelectFile: (path: string) => void;
  onContextMenu: (e: React.MouseEvent, node: FlatNode) => void;
}

interface ContextMenuState {
  x: number;
  y: number;
  node: FlatNode;
}

const ROW_HEIGHT = 28;

const Row = memo(function Row({ index, style, data }: ListChildComponentProps<RowData>) {
  const node = data.flatNodes[index];
  return (
    <TreeNodeRow
      node={node}
      style={style}
      isSelected={data.selectedPath === node.path}
      onToggleDir={data.onToggleDir}
      onSelectFile={data.onSelectFile}
      onContextMenu={data.onContextMenu}
    />
  );
});

export function FileTreeSidebar({ flatNodes, selectedPath, onToggleDir, onSelectFile }: FileTreeSidebarProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState(400);
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);

  // Track container height with ResizeObserver for react-window.
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setHeight(entry.contentRect.height);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const handleContextMenu = useCallback((e: React.MouseEvent, node: FlatNode) => {
    setContextMenu({ x: e.clientX, y: e.clientY, node });
  }, []);

  const handleCloseContextMenu = useCallback(() => {
    setContextMenu(null);
  }, []);

  const itemData = useMemo<RowData>(() => ({
    flatNodes,
    selectedPath,
    onToggleDir,
    onSelectFile,
    onContextMenu: handleContextMenu,
  }), [flatNodes, selectedPath, onToggleDir, onSelectFile, handleContextMenu]);

  return (
    <div className="file-tree-sidebar" ref={containerRef}>
      <FixedSizeList
        height={height}
        itemCount={flatNodes.length}
        itemSize={ROW_HEIGHT}
        width="100%"
        itemData={itemData}
        overscanCount={10}
      >
        {Row}
      </FixedSizeList>
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
