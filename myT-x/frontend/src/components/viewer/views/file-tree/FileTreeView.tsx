import { useViewerStore } from "../../viewerStore";
import { FileContentViewer } from "./FileContentViewer";
import { FileTreeSidebar } from "./FileTreeSidebar";
import { useFileTree } from "./useFileTree";

export function FileTreeView() {
  const closeView = useViewerStore((s) => s.closeView);
  const {
    flatNodes,
    selectedPath,
    fileContent,
    isLoadingContent,
    rootLoading,
    error,
    toggleDir,
    selectFile,
    loadRoot,
    activeSession,
  } = useFileTree();

  if (!activeSession) {
    return (
      <div className="file-tree-view">
        <div className="viewer-header">
          <h2 className="viewer-header-title">File Tree</h2>
          <div className="viewer-header-spacer" />
          <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
        </div>
        <div className="viewer-message">No active session</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="file-tree-view">
        <div className="viewer-header">
          <h2 className="viewer-header-title">File Tree</h2>
          <div className="viewer-header-spacer" />
          <button className="viewer-header-btn" onClick={loadRoot} title="Refresh">{"\u21BB"}</button>
          <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
        </div>
        <div className="viewer-message">{error}</div>
      </div>
    );
  }

  return (
    <div className="file-tree-view">
      <div className="viewer-header">
        <h2 className="viewer-header-title">File Tree</h2>
        <div className="viewer-header-spacer" />
        <button className="viewer-header-btn" onClick={loadRoot} title="Refresh">{"\u21BB"}</button>
        <button className="viewer-header-btn" onClick={closeView} title="Close">{"\u2715"}</button>
      </div>
      <div className="file-tree-body">
        {rootLoading ? (
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
              <FileContentViewer content={fileContent} isLoading={isLoadingContent} />
            </div>
          </>
        )}
      </div>
    </div>
  );
}
