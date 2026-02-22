import {useViewerStore} from "../../viewerStore";
import {DiffContentViewer} from "./DiffContentViewer";
import {DiffFileSidebar} from "./DiffFileSidebar";
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
    } = useDiffView();

    if (!activeSession) {
        return (
            <div className="diff-view">
                <div className="viewer-header">
                    <h2 className="viewer-header-title">Diff</h2>
                    <div className="viewer-header-spacer"/>
                    <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
                </div>
                <div className="viewer-message">No active session</div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="diff-view">
                <div className="viewer-header">
                    <h2 className="viewer-header-title">Diff</h2>
                    <div className="viewer-header-spacer"/>
                    <button className="viewer-header-btn" onClick={() => loadDiff()}
                            title="Refresh">{"\u21BB"}</button>
                    <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
                </div>
                <div className="viewer-message">{error}</div>
            </div>
        );
    }

    const fileCount = diffResult?.files?.length ?? 0;
    const totalAdded = diffResult?.total_added ?? 0;
    const totalDeleted = diffResult?.total_deleted ?? 0;

    return (
        <div className="diff-view">
            <div className="viewer-header">
                <h2 className="viewer-header-title">Diff</h2>
                {fileCount > 0 && (
                    <>
                        <span className="diff-header-stats">
                            <span className="diff-tree-additions">+{totalAdded}</span>
                            <span className="diff-tree-deletions"> -{totalDeleted}</span>
                        </span>
                        <span className="diff-header-file-count">
                            Files Changed: {fileCount}
                        </span>
                    </>
                )}
                {diffResult?.truncated && (
                    <span className="diff-header-truncated">
                        (truncated)
                    </span>
                )}
                <div className="viewer-header-spacer"/>
                <button className="viewer-header-btn" onClick={() => loadDiff()}
                        title="Refresh">{"\u21BB"}</button>
                <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
            </div>
            <div className="file-tree-body">
                {isLoading ? (
                    <div className="viewer-message">Loading diff...</div>
                ) : fileCount === 0 ? (
                    <div className="viewer-message">No working changes</div>
                ) : (
                    <>
                        <DiffFileSidebar
                            flatNodes={flatNodes}
                            selectedPath={selectedPath}
                            onToggleDir={toggleDir}
                            onSelectFile={selectFile}
                        />
                        <div className="file-tree-content">
                            <DiffContentViewer file={selectedFile}/>
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
