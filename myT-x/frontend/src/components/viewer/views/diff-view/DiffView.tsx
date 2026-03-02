import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
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
            onRefresh={loadDiff}
            headerChildren={(
                <>
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
                </>
            )}
        >
            <div className="diff-view-body">
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
                        <div className="diff-view-content">
                            <DiffContentViewer file={selectedFile}/>
                        </div>
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
