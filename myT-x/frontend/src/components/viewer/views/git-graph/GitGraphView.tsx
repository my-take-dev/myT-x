import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {BranchStatusBar} from "./BranchStatusBar";
import {CommitGraph} from "./CommitGraph";
import {DiffViewer} from "./DiffViewer";
import {useGitGraph} from "./useGitGraph";

export function GitGraphView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        commits,
        laneAssignments,
        status,
        selectedCommit,
        diff,
        diffError,
        isLoadingDiff,
        allBranches,
        isLoading,
        error,
        logCount,
        selectCommit,
        loadMore,
        loadData,
        toggleAllBranches,
        activeSession,
    } = useGitGraph();

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="git-graph-view"
                title="Git Graph"
                onClose={closeView}
                message="No active session"
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="git-graph-view"
                title="Git Graph"
                onClose={closeView}
                onRefresh={loadData}
                message={error}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="git-graph-view"
            title="Git Graph"
            onClose={closeView}
            onRefresh={loadData}
            headerChildren={(
                <label className="git-graph-branches-label">
                    <input
                        type="checkbox"
                        checked={allBranches}
                        onChange={(e) => toggleAllBranches(e.target.checked)}
                    />
                    All branches
                </label>
            )}
        >

            <BranchStatusBar status={status}/>

            <div className="git-graph-body">
                {isLoading ? (
                    <div className="viewer-message">Loading git data...</div>
                ) : commits.length === 0 ? (
                    <div className="viewer-message">No commits found.</div>
                ) : (
                    <>
                        <CommitGraph
                            commits={commits}
                            laneAssignments={laneAssignments}
                            selectedCommit={selectedCommit}
                            onSelectCommit={selectCommit}
                            logCount={logCount}
                            onLoadMore={loadMore}
                        />

                        {selectedCommit && (
                            <div className="git-detail-panel">
                                <div className="git-detail-header">
                                    <div>
                                        <span className="git-detail-hash">{selectedCommit.hash}</span>
                                        <span className="git-detail-author"> by {selectedCommit.author_name}</span>
                                    </div>
                                    <div className="git-detail-message">{selectedCommit.subject}</div>
                                </div>
                                {isLoadingDiff && <div className="viewer-message">Loading diff...</div>}
                                {!isLoadingDiff && diffError && <div className="viewer-message">{diffError}</div>}
                                {!isLoadingDiff && !diffError && diff != null && (
                                    <DiffViewer key={selectedCommit.full_hash} diff={diff}/>
                                )}
                            </div>
                        )}
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
