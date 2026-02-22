import { memo, useEffect, useMemo, useRef, useState } from "react";
import { FixedSizeList, type ListChildComponentProps } from "react-window";
import type { GitGraphCommit, LaneAssignment } from "./gitGraphTypes";
import { CommitRow } from "./CommitRow";

interface CommitGraphProps {
  commits: GitGraphCommit[];
  laneAssignments: LaneAssignment[];
  selectedCommit: GitGraphCommit | null;
  onSelectCommit: (commit: GitGraphCommit) => void;
  logCount: number;
  onLoadMore: () => void;
}

interface RowData {
  commits: GitGraphCommit[];
  laneAssignments: LaneAssignment[];
  selectedHash: string | null;
  onSelectCommit: (commit: GitGraphCommit) => void;
}

const ROW_HEIGHT = 32;

const Row = memo(function Row({ index, style, data }: ListChildComponentProps<RowData>) {
  const commit = data.commits[index];
  const laneAssignment = data.laneAssignments[index];
  return (
    <CommitRow
      commit={commit}
      laneAssignment={laneAssignment}
      style={style}
      isSelected={data.selectedHash === commit.full_hash}
      onSelect={data.onSelectCommit}
    />
  );
});

export function CommitGraph({
  commits,
  laneAssignments,
  selectedCommit,
  onSelectCommit,
  logCount,
  onLoadMore,
}: CommitGraphProps) {
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
    commits,
    laneAssignments,
    selectedHash: selectedCommit?.full_hash ?? null,
    onSelectCommit,
  }), [commits, laneAssignments, selectedCommit, onSelectCommit]);

  const canLoadMore = commits.length >= logCount && logCount < 1000;

  return (
    <div className="git-graph-commits" ref={containerRef}>
      <FixedSizeList
        height={height}
        itemCount={commits.length}
        itemSize={ROW_HEIGHT}
        width="100%"
        itemData={itemData}
        overscanCount={10}
      >
        {Row}
      </FixedSizeList>
      {canLoadMore && (
        <div className="git-load-more">
          <button onClick={onLoadMore}>Load more commits...</button>
        </div>
      )}
    </div>
  );
}
