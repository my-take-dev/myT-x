import {forwardRef, memo, useMemo, useRef, type HTMLAttributes} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import type {GitGraphCommit, LaneAssignment} from "./gitGraphTypes";
import {CommitRow} from "./CommitRow";
import {MAX_LOG_COUNT} from "./useGitGraph";

interface CommitGraphProps {
    commits: readonly GitGraphCommit[];
    laneAssignments: readonly LaneAssignment[];
    selectedCommit: GitGraphCommit | null;
    onSelectCommit: (commit: GitGraphCommit) => void;
    logCount: number;
    onLoadMore: () => void;
}

interface RowData {
    commits: readonly GitGraphCommit[];
    laneAssignments: readonly LaneAssignment[];
    selectedHash: string | null;
    onSelectCommit: (commit: GitGraphCommit) => void;
}

const ROW_HEIGHT = 32;

/** Outer container for FixedSizeList providing list semantics for accessibility. */
const CommitListOuter = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
    function CommitListOuter(props, ref) {
        return <div {...props} ref={ref} role="list" aria-label="Commit history"/>;
    },
);

const Row = memo(function Row({index, style, data}: ListChildComponentProps<RowData>) {
    const commit = data.commits[index];
    const laneAssignment = data.laneAssignments[index];
    if (!commit || !laneAssignment) return null;
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
    const height = useContainerHeight(containerRef);

    const itemData = useMemo<RowData>(() => ({
        commits,
        laneAssignments,
        selectedHash: selectedCommit?.full_hash ?? null,
        onSelectCommit,
    }), [commits, laneAssignments, selectedCommit, onSelectCommit]);

    // commits.length can temporarily exceed logCount when filters change and old rows remain visible
    // until the next fetch settles. Using >= keeps the "Load more" action available in that state.
    const canLoadMore = commits.length > 0 && commits.length >= logCount && logCount < MAX_LOG_COUNT;

    return (
        <div className="git-graph-commits" ref={containerRef}>
            {/* NOTE: height starts at 0 until ResizeObserver reports; guard prevents empty FixedSizeList render. */}
            {height > 0 && (
                <FixedSizeList
                    height={height}
                    itemCount={commits.length}
                    itemSize={ROW_HEIGHT}
                    width="100%"
                    itemData={itemData}
                    overscanCount={10}
                    outerElementType={CommitListOuter}
                >
                    {Row}
                </FixedSizeList>
            )}
            {canLoadMore && (
                <div className="git-load-more">
                    <button type="button" onClick={onLoadMore}>Load more commits...</button>
                </div>
            )}
        </div>
    );
}
