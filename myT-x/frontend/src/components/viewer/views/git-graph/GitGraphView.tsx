import { useViewerStore } from "../../viewerStore";
import { BranchStatusBar } from "./BranchStatusBar";
import { CommitGraph } from "./CommitGraph";
import { DiffViewer } from "./DiffViewer";
import { useGitGraph } from "./useGitGraph";

export function GitGraphView() {
  const closeView = useViewerStore((s) => s.closeView);
  const {
    commits,
    laneAssignments,
    status,
    selectedCommit,
    diff,
    isLoadingDiff,
    allBranches,
    loading,
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
      <div className="git-graph-view">
        <div className="viewer-header">
          <h2 className="viewer-header-title">Git Graph</h2>
          <div className="viewer-header-spacer" />
          <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
        </div>
        <div className="viewer-message">No active session</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="git-graph-view">
        <div className="viewer-header">
          <h2 className="viewer-header-title">Git Graph</h2>
          <div className="viewer-header-spacer" />
          <button className="viewer-header-btn" onClick={loadData} title="Refresh">{"\u21BB"}</button>
          <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
        </div>
        <div className="viewer-message">{error}</div>
      </div>
    );
  }

  return (
    <div className="git-graph-view">
      <div className="viewer-header">
        <h2 className="viewer-header-title">Git Graph</h2>
        <label style={{ display: "flex", alignItems: "center", gap: 4, fontSize: "0.78rem", color: "var(--fg-dim)" }}>
          <input
            type="checkbox"
            checked={allBranches}
            onChange={(e) => toggleAllBranches(e.target.checked)}
          />
          All branches
        </label>
        <div className="viewer-header-spacer" />
        <button className="viewer-header-btn" onClick={loadData} title="Refresh">{"\u21BB"}</button>
        <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
      </div>

      <BranchStatusBar status={status} />

      <div className="git-graph-body">
        {loading ? (
          <div className="viewer-message">Loading git data...</div>
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
                {isLoadingDiff ? (
                  <div className="viewer-message">Loading diff...</div>
                ) : diff ? (
                  <DiffViewer diff={diff} />
                ) : null}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
