import {lazy, Suspense, useCallback, useEffect, useMemo, useState} from "react";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {FileContentViewer} from "./FileContentViewer";
import {FileSearchPanel} from "./FileSearchPanel";
import {FileTreeSidebar} from "./FileTreeSidebar";
import {DrawioRenderer} from "./renderers/DrawioRenderer";
import {MarkdownRenderer} from "./renderers/MarkdownRenderer";
import type {DocumentKind} from "./documentTypes";
import {classifyDocument, filterDocumentTree, isDocumentFile} from "./documentFilter";
import {flattenTree} from "./treeUtils";
import type {FileContentResult} from "./fileTreeTypes";
import {useFileSearch} from "./useFileSearch";
import {useFileTree} from "./useFileTree";

const LazyMermaidRenderer = lazy(async () => ({
    default: (await import("./renderers/MermaidRenderer")).MermaidRenderer,
}));

const LazySwaggerRenderer = lazy(async () => ({
    default: (await import("./renderers/SwaggerRenderer")).SwaggerRenderer,
}));

const LazySqliteRenderer = lazy(async () => ({
    default: (await import("./renderers/SqliteRenderer")).SqliteRenderer,
}));

export function FileTreeView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        tree,
        expandedPaths,
        loadingPaths,
        selectedPath,
        fileContent,
        isLoadingContent,
        isRootLoading,
        error,
        contentError,
        dirError,
        watcherError,
        toggleDir,
        selectFile,
        loadRoot,
        activeSession,
        activeSessionKey,
    } = useFileTree();

    const {query, setQuery, results, isSearching, searchError, clearSearch} = useFileSearch();
    const [isSearchMode, setIsSearchMode] = useState(false);

    const filteredTree = useMemo(
        () => filterDocumentTree(tree),
        [tree],
    );
    const filteredFlatNodes = useMemo(
        () => flattenTree(filteredTree, expandedPaths, loadingPaths),
        [expandedPaths, filteredTree, loadingPaths],
    );
    const filteredSearchResults = useMemo(
        () => results.filter((result) => isDocumentFile({
            name: result.name,
            path: result.path,
            isDir: false,
            hasChildren: false,
        })),
        [results],
    );

    const currentDocumentKind = useMemo<DocumentKind | null>(() => {
        if (!fileContent) {
            return null;
        }
        const segments = fileContent.path.split("/");
        const fileName = segments[segments.length - 1] ?? fileContent.path;
        return classifyDocument(fileName, fileContent.content);
    }, [fileContent]);

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

    const previewRenderer = useCallback((content: FileContentResult, kind: DocumentKind) => {
        switch (kind) {
            case "markdown":
                return (
                    <MarkdownRenderer
                        content={content.content}
                        filePath={content.path}
                        sessionKey={activeSessionKey}
                        sessionName={activeSession}
                    />
                );
            case "mermaid":
                return (
                    <Suspense fallback={<div className="file-content-empty">Loading Mermaid preview...</div>}>
                        <LazyMermaidRenderer code={content.content}/>
                    </Suspense>
                );
            case "swagger":
                return (
                    <Suspense fallback={<div className="file-content-empty">Loading Swagger preview...</div>}>
                        <LazySwaggerRenderer content={content.content} filePath={content.path}/>
                    </Suspense>
                );
            case "drawio-svg":
            case "drawio-xml":
                return (
                    <DrawioRenderer
                        kind={kind}
                        content={content.content}
                        filePath={content.path}
                        sessionKey={activeSessionKey}
                        sessionName={activeSession}
                    />
                );
            case "sqlite":
                return (
                    <Suspense fallback={<div className="file-content-empty">Loading SQLite preview...</div>}>
                        <LazySqliteRenderer
                            filePath={content.path}
                            sessionKey={activeSessionKey}
                            sessionName={activeSession}
                        />
                    </Suspense>
                );
            case "yaml-json-raw":
                return null;
        }
    }, [activeSession, activeSessionKey]);

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="file-tree-view"
                title="File View"
                onClose={closeView}
                message="No active session"
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="file-tree-view"
                title="File View"
                onClose={closeView}
                onRefresh={loadRoot}
                message={error}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="file-tree-view"
            title="File View"
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
                                results={filteredSearchResults}
                                isSearching={isSearching}
                                searchError={searchError}
                                selectedPath={selectedPath}
                                onSelectFile={selectFile}
                                onOpenFile={handleOpenFile}
                                onClose={handleSearchClose}
                            />
                        ) : (
                            <FileTreeSidebar
                                flatNodes={filteredFlatNodes}
                                selectedPath={selectedPath}
                                onToggleDir={toggleDir}
                                onSelectFile={selectFile}
                                onSearchOpen={handleSearchOpen}
                            />
                        )}
                        {/* Right panel: always visible (shared between both modes) */}
                        <div className="file-tree-content">
                            {watcherError ? (
                                <div className="file-tree-watcher-warning">{watcherError}</div>
                            ) : null}
                            {/* Priority: dirError > contentError > content.
                               watcherError is rendered separately because degraded auto-refresh
                               should stay visible until the watcher is restarted or the session changes. */}
                            {dirError ? (
                                <div className="file-tree-dir-error">{dirError}</div>
                            ) : contentError ? (
                                <div className="file-content-empty">{contentError}</div>
                            ) : (
                                <FileContentViewer
                                    key={fileContent?.path ?? "file-content-empty"}
                                    content={fileContent}
                                    isLoading={isLoadingContent}
                                    documentKind={currentDocumentKind}
                                    previewRenderer={previewRenderer}
                                />
                            )}
                        </div>
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
