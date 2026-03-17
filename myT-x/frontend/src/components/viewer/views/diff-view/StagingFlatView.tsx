import {useCallback, useMemo, useRef, useState} from "react";
import {FixedSizeList} from "react-window";
import {useContainerHeight} from "../../../../hooks/useContainerHeight";
import {makeTreeOuter} from "../shared/TreeOuter";
import {CommitPanel} from "./CommitPanel";
import {DiscardConfirmDialog} from "./DiscardConfirmDialog";
import {StagingRow} from "./StagingFileRow";
import type {StagingRowData} from "./StagingFileRow";
import type {BranchInfo, OperationType, StagingListItem} from "./sourceControlTypes";

interface StagingFlatViewProps {
    stagingItems: readonly StagingListItem[];
    selectedPath: string | null;
    stagedCount: number;
    unstagedCount: number;
    branchInfo: BranchInfo | null;
    operationInFlight: OperationType;
    commitMessage: string;
    onSetCommitMessage: (msg: string) => void;
    onSelectFile: (path: string) => void;
    onStageFile: (path: string) => Promise<void>;
    onUnstageFile: (path: string) => Promise<void>;
    onDiscardFile: (path: string) => Promise<void>;
    onStageAll: () => Promise<void>;
    onUnstageAll: () => Promise<void>;
    onToggleGroup: (group: "staged" | "unstaged") => void;
    onCommit: (message: string) => Promise<boolean>;
    onCommitAndPush: (message: string) => Promise<boolean>;
    onPush: () => Promise<void>;
    onPull: () => Promise<void>;
    onFetch: () => Promise<void>;
}

const ROW_HEIGHT = 28;
const StagingListOuter = makeTreeOuter("Staging files");

export function StagingFlatView({
    stagingItems,
    selectedPath,
    stagedCount,
    unstagedCount,
    branchInfo,
    operationInFlight,
    commitMessage,
    onSetCommitMessage,
    onSelectFile,
    onStageFile,
    onUnstageFile,
    onDiscardFile,
    onStageAll,
    onUnstageAll,
    onToggleGroup,
    onCommit,
    onCommitAndPush,
    onPush,
    onPull,
    onFetch,
}: StagingFlatViewProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const height = useContainerHeight(containerRef, ROW_HEIGHT, {noiseThresholdPx: 1});

    // Discard confirmation state.
    const [discardTarget, setDiscardTarget] = useState<string | null>(null);

    const handleDiscardFile = useCallback((path: string) => {
        setDiscardTarget(path);
    }, []);

    const confirmDiscard = useCallback(() => {
        if (discardTarget) {
            void onDiscardFile(discardTarget);
            setDiscardTarget(null);
        }
    }, [discardTarget, onDiscardFile]);

    const cancelDiscard = useCallback(() => {
        setDiscardTarget(null);
    }, []);

    const handleBatchAction = useCallback((group: "staged" | "unstaged") => {
        if (group === "staged") {
            void onUnstageAll();
        } else {
            void onStageAll();
        }
    }, [onStageAll, onUnstageAll]);

    const itemData = useMemo<StagingRowData>(() => ({
        items: stagingItems,
        selectedPath,
        operationInFlight,
        onSelectFile,
        onStageFile: (path: string) => { void onStageFile(path); },
        onUnstageFile: (path: string) => { void onUnstageFile(path); },
        onDiscardFile: handleDiscardFile,
        onToggleGroup: onToggleGroup,
        onBatchAction: handleBatchAction,
    }), [
        stagingItems, selectedPath, operationInFlight,
        onSelectFile, onStageFile, onUnstageFile, handleDiscardFile,
        onToggleGroup, handleBatchAction,
    ]);

    return (
        <div className="staging-flat-view">
            <div className="staging-file-list" ref={containerRef}>
                {height > 0 ? (
                    <FixedSizeList
                        height={height}
                        itemCount={stagingItems.length}
                        itemSize={ROW_HEIGHT}
                        width="100%"
                        itemData={itemData}
                        overscanCount={10}
                        outerElementType={StagingListOuter}
                    >
                        {StagingRow}
                    </FixedSizeList>
                ) : null}
            </div>
            <CommitPanel
                branchInfo={branchInfo}
                commitMessage={commitMessage}
                onSetCommitMessage={onSetCommitMessage}
                onCommit={onCommit}
                onCommitAndPush={onCommitAndPush}
                onPush={onPush}
                onPull={onPull}
                onFetch={onFetch}
                operationInFlight={operationInFlight}
                stagedCount={stagedCount}
            />
            {discardTarget !== null && (
                <DiscardConfirmDialog
                    filePath={discardTarget}
                    onConfirm={confirmDiscard}
                    onCancel={cancelDiscard}
                />
            )}
        </div>
    );
}
