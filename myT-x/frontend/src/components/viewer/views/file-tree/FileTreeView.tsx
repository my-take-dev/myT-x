import {useCallback, useEffect, useState} from "react";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {FileContentViewer} from "./FileContentViewer";
import {FileSearchPanel} from "./FileSearchPanel";
import {FileTreeSidebar} from "./FileTreeSidebar";
import {useFileSearch} from "./useFileSearch";
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

    const {query, setQuery, results, isSearching, searchError, clearSearch} = useFileSearch();
    const [isSearchMode, setIsSearchMode] = useState(false);

    const handleSearchOpen = useCallback(() => setIsSearchMode(true), []);
    const handleSearchClose = useCallback(() => {
        setIsSearchMode(false);
        clearSearch();
    }, [clearSearch]);

    // Double-click handler: select file + close search.
    const handleOpenFile = useCallback((path: string) => {
        selectFile(path);
        setIsSearchMode(false);
    }, [selectFile]);

    // Ctrl+F handler — skip when focus is inside a terminal pane to avoid
    // conflicting with the terminal's own Ctrl+F search bar.
    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if (e.ctrlKey && e.key === "f") {
                if (document.activeElement?.closest(".xterm")) return;
                e.preventDefault();
                setIsSearchMode(true);
            }
        };
        document.addEventListener("keydown", handler);
        return () => document.removeEventListener("keydown", handler);
    }, []);

    // Reset search mode on session change.
    useEffect(() => {
        setIsSearchMode(false);
        clearSearch();
    }, [activeSession, clearSearch]);

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
                        {/* Left panel: search mode or tree mode */}
                        {isSearchMode ? (
                            <FileSearchPanel
                                query={query}
                                onQueryChange={setQuery}
                                results={results}
                                isSearching={isSearching}
                                searchError={searchError}
                                selectedPath={selectedPath}
                                onSelectFile={selectFile}
                                onOpenFile={handleOpenFile}
                                onClose={handleSearchClose}
                            />
                        ) : (
                            <FileTreeSidebar
                                flatNodes={flatNodes}
                                selectedPath={selectedPath}
                                onToggleDir={toggleDir}
                                onSelectFile={selectFile}
                                onSearchOpen={handleSearchOpen}
                            />
                        )}
                        {/* Right panel: always visible (shared between both modes) */}
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
