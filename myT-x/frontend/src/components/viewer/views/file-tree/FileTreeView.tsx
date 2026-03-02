import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {FileContentViewer} from "./FileContentViewer";
import {FileTreeSidebar} from "./FileTreeSidebar";
import {useFileTree} from "./useFileTree";

export function FileTreeView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        flatNodes,
        selectedPath,
        fileContent,
        isLoadingContent,
        isRootLoading,
        error,
        contentError,
        dirError,
        toggleDir,
        selectFile,
        loadRoot,
        activeSession,
    } = useFileTree();

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="file-tree-view"
                title="File Tree"
                onClose={closeView}
                message="No active session"
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="file-tree-view"
                title="File Tree"
                onClose={closeView}
                onRefresh={loadRoot}
                message={error}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="file-tree-view"
            title="File Tree"
            onClose={closeView}
            onRefresh={loadRoot}
        >
            <div className="file-tree-body">
                {isRootLoading ? (
                    <div className="viewer-message">Loading file tree...</div>
                ) : (
                    <>
                        <FileTreeSidebar
                            flatNodes={flatNodes}
                            selectedPath={selectedPath}
                            onToggleDir={toggleDir}
                            onSelectFile={selectFile}
                        />
                        <div className="file-tree-content">
                            {/* Priority: dirError > contentError > content.
                               dirError is cleared by selectFile, loadRoot, and any directory
                               toggle (toggleDir collapse/expand start, and expand success),
                               so it only persists between the failed expand and the user's next action. */}
                            {dirError ? (
                                <div className="file-tree-dir-error">{dirError}</div>
                            ) : contentError ? (
                                <div className="file-content-empty">{contentError}</div>
                            ) : (
                                <FileContentViewer content={fileContent} isLoading={isLoadingContent}/>
                            )}
                        </div>
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
