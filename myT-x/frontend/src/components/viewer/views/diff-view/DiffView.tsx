import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {CommitPanel} from "./CommitPanel";
import {DiffContentViewer} from "./DiffContentViewer";
import {DiffFileSidebar} from "./DiffFileSidebar";
import {DiffViewModeToggle} from "./DiffViewModeToggle";
import {StagingFlatView} from "./StagingFlatView";
import {useDiffView} from "./useDiffView";

export function DiffView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        flatNodes,
        selectedPath,
        selectedFile,
        diffResult,
        isLoading,
        error,
        toggleDir,
        selectFile,
        loadDiff,
        activeSession,
        // Sidebar mode
        sidebarMode,
        setSidebarMode,
        // Flat view staging
        stagingItems,
        stagedCount,
        unstagedCount,
        branchInfo,
        toggleStagingGroup,
        // Git operations
        operationInFlight,
        stageFile,
        unstageFile,
        discardFile,
        stageAll,
        unstageAll,
        commit,
        commitAndPush,
        push,
        pull,
        fetch: fetch_,
        // Commit message
        commitMessage,
        setCommitMessage,
    } = useDiffView();
    const fileCount = diffResult?.files?.length ?? 0;
    const totalAdded = diffResult?.total_added ?? 0;
    const totalDeleted = diffResult?.total_deleted ?? 0;

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="diff-view"
                title="Diff"
                onClose={closeView}
                message="No active session"
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="diff-view"
                title="Diff"
                onClose={closeView}
                onRefresh={loadDiff}
                message={error}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="diff-view"
            title="Diff"
            onClose={closeView}
        >
            {/* Action bar: all operation buttons in one row */}
            <div className="diff-action-bar">
                <span className="diff-action-bar-title">Diff</span>
                <DiffViewModeToggle mode={sidebarMode} onModeChange={setSidebarMode} />
                <span className="diff-action-bar-spacer" />
                <button type="button" className="viewer-header-btn" onClick={() => loadDiff()}
                        title="Refresh" aria-label="Refresh">{"\u21BB"}</button>
                <button type="button" className="viewer-header-btn" onClick={closeView}
                        title="Close" aria-label="Close">{"\u2715"}</button>
            </div>

            {fileCount > 0 && (
                <div className="diff-subbar">
                    <span className="diff-header-stats">
                        <span className="diff-tree-additions">+{totalAdded}</span>
                        <span className="diff-tree-deletions"> -{totalDeleted}</span>
                    </span>
                    <span className="diff-header-file-count">
                        Files: {fileCount}
                    </span>
                    {diffResult?.truncated && (
                        <span className="diff-header-truncated">
                            (truncated)
                        </span>
                    )}
                </div>
            )}
            <div className="diff-view-body">
                {branchInfo?.statusFetchFailed && (
                    <div className="diff-status-warning">{"\u26A0"} Git status unavailable — file list may be stale</div>
                )}
                {isLoading && !diffResult ? (
                    <div className="viewer-message">Loading diff...</div>
                ) : fileCount === 0 ? (
                    <div className="viewer-message">No working changes</div>
                ) : (
                    <>
                        {sidebarMode === "tree" ? (
                            <div className="tree-sidebar-with-commit">
                                <DiffFileSidebar
                                    flatNodes={flatNodes}
                                    selectedPath={selectedPath}
                                    onToggleDir={toggleDir}
                                    onSelectFile={selectFile}
                                />
                                <CommitPanel
                                    branchInfo={branchInfo}
                                    commitMessage={commitMessage}
                                    onSetCommitMessage={setCommitMessage}
                                    onCommit={commit}
                                    onCommitAndPush={commitAndPush}
                                    onPush={push}
                                    onPull={pull}
                                    onFetch={fetch_}
                                    operationInFlight={operationInFlight}
                                    stagedCount={stagedCount}
                                />
                            </div>
                        ) : (
                            <StagingFlatView
                                stagingItems={stagingItems}
                                selectedPath={selectedPath}
                                stagedCount={stagedCount}
                                unstagedCount={unstagedCount}
                                branchInfo={branchInfo}
                                operationInFlight={operationInFlight}
                                commitMessage={commitMessage}
                                onSetCommitMessage={setCommitMessage}
                                onSelectFile={selectFile}
                                onStageFile={stageFile}
                                onUnstageFile={unstageFile}
                                onDiscardFile={discardFile}
                                onStageAll={stageAll}
                                onUnstageAll={unstageAll}
                                onToggleGroup={toggleStagingGroup}
                                onCommit={commit}
                                onCommitAndPush={commitAndPush}
                                onPush={push}
                                onPull={pull}
                                onFetch={fetch_}
                            />
                        )}
                        <div className="diff-view-content">
                            <DiffContentViewer file={selectedFile}/>
                        </div>
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
