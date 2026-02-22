import { memo, useEffect, useMemo, useRef, useState } from "react";
import { FixedSizeList, type ListChildComponentProps } from "react-window";
import type { FlatNode } from "./fileTreeTypes";
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
    />
  );
});

export function FileTreeSidebar({ flatNodes, selectedPath, onToggleDir, onSelectFile }: FileTreeSidebarProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState(400);

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

  const itemData = useMemo<RowData>(() => ({
    flatNodes,
    selectedPath,
    onToggleDir,
    onSelectFile,
  }), [flatNodes, selectedPath, onToggleDir, onSelectFile]);

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
    </div>
  );
}
