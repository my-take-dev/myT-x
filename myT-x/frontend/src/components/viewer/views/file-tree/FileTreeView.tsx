import {lazy, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {useViewerStore} from "../../viewerStore";
import {findViewerViewForShortcut} from "../../viewerShortcutDefinitions";
import {isImeTransitionalEvent} from "../../../../utils/ime";
import {useI18n} from "../../../../i18n";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {FileContentViewer} from "./FileContentViewer";
import {FileSearchPanel} from "./FileSearchPanel";
import {FileTreeSidebar} from "./FileTreeSidebar";
import {buildShortcutFromKeyboardEvent} from "../../viewerShortcutUtils";
import {DrawioRenderer} from "./renderers/DrawioRenderer";
import {MarkdownRenderer} from "./renderers/MarkdownRenderer";
import {RendererSurface} from "./renderers/RendererSurface";
import {
    canPreviewDocumentKind,
    getDefaultRenderModeForDocumentKind,
    type DocumentKind,
    type RenderMode,
} from "./documentTypes";
import {FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT} from "./fileContentShortcuts";
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

const LazyGraphvizRenderer = lazy(async () => ({
    default: (await import("./renderers/GraphvizRenderer")).GraphvizRenderer,
}));

const LazyMarkmapRenderer = lazy(async () => ({
    default: (await import("./renderers/MarkmapRenderer")).MarkmapRenderer,
}));

const LazyWavedromRenderer = lazy(async () => ({
    default: (await import("./renderers/WavedromRenderer")).WavedromRenderer,
}));

const LazyVegaLiteRenderer = lazy(async () => ({
    default: (await import("./renderers/VegaLiteRenderer")).VegaLiteRenderer,
}));

interface FileContentRenderState {
    readonly path: string | null;
    readonly mode: RenderMode;
}

function isSearchShortcutBlocked(activeElement: Element | null): boolean {
    return activeElement instanceof HTMLElement && activeElement.closest(".xterm") !== null;
}

function isPreviewShortcutBlocked(activeElement: Element | null): boolean {
    if (!(activeElement instanceof HTMLElement)) {
        return false;
    }

    return activeElement instanceof HTMLInputElement
        || activeElement instanceof HTMLTextAreaElement
        || activeElement instanceof HTMLSelectElement
        || activeElement.isContentEditable
        || activeElement.closest("[contenteditable]") !== null
        || activeElement.closest(".monaco-editor") !== null
        || activeElement.closest(".xterm") !== null;
}

export function FileTreeView() {
    const {t} = useI18n();
    const closeView = useViewerStore((s) => s.closeView);
    const viewerShortcutsConfig = useTmuxStore((state) => state.config?.viewer_shortcuts ?? null);
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
    const currentFilePath = fileContent?.path ?? null;
    const defaultRenderMode = getDefaultRenderModeForDocumentKind(currentDocumentKind);
    const canPreviewCurrentDocument = canPreviewDocumentKind(currentDocumentKind);
    const [fileContentRenderState, setFileContentRenderState] = useState<FileContentRenderState>(() => ({
        path: currentFilePath,
        mode: defaultRenderMode,
    }));
    const currentRenderMode = fileContentRenderState.path === currentFilePath
        ? fileContentRenderState.mode
        : defaultRenderMode;
    const previewShortcutOwner = useMemo(
        () => findViewerViewForShortcut(viewerShortcutsConfig, FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT),
        [viewerShortcutsConfig],
    );
    const previewShortcutStateRef = useRef({
        canPreviewCurrentDocument,
        currentFilePath,
        defaultRenderMode,
        previewShortcutOwner,
    });

    const handleSearchOpen = useCallback(() => setIsSearchMode(true), []);
    const handleSearchClose = useCallback(() => {
        setIsSearchMode(false);
        clearSearch();
    }, [clearSearch]);
    const handleRenderModeChange = useCallback((mode: RenderMode) => {
        setFileContentRenderState({
            path: currentFilePath,
            mode,
        });
    }, [currentFilePath]);

    // Double-click handler: select file + close search.
    const handleOpenFile = useCallback((path: string) => {
        selectFile(path);
        setIsSearchMode(false);
    }, [selectFile]);

    useEffect(() => {
        previewShortcutStateRef.current = {
            canPreviewCurrentDocument,
            currentFilePath,
            defaultRenderMode,
            previewShortcutOwner,
        };
    }, [canPreviewCurrentDocument, currentFilePath, defaultRenderMode, previewShortcutOwner]);

    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if (e.defaultPrevented || isImeTransitionalEvent(e)) {
                return;
            }
            const {
                canPreviewCurrentDocument: latestCanPreviewCurrentDocument,
                currentFilePath: latestCurrentFilePath,
                defaultRenderMode: latestDefaultRenderMode,
                previewShortcutOwner: latestPreviewShortcutOwner,
            } = previewShortcutStateRef.current;

            const shortcut = buildShortcutFromKeyboardEvent(e);
            if (shortcut === "ctrl+f") {
                if (isSearchShortcutBlocked(document.activeElement)) {
                    return;
                }
                e.preventDefault();
                setIsSearchMode(true);
                return;
            }

            if (shortcut !== FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT) {
                return;
            }

            if (latestPreviewShortcutOwner !== null) {
                return;
            }

            if (!latestCanPreviewCurrentDocument || isPreviewShortcutBlocked(document.activeElement)) {
                return;
            }

            e.preventDefault();
            setFileContentRenderState((previous) => {
                const effectiveMode = previous.path === latestCurrentFilePath ? previous.mode : latestDefaultRenderMode;
                return {
                    path: latestCurrentFilePath,
                    mode: effectiveMode === "preview" ? "raw" : "preview",
                };
            });
        };
        document.addEventListener("keydown", handler);
        return () => document.removeEventListener("keydown", handler);
    }, []);

    // Reset search mode on session change.
    useEffect(() => {
        setIsSearchMode(false);
        clearSearch();
    }, [activeSession, clearSearch]);

    useEffect(() => {
        setFileContentRenderState((previous) => {
            if (previous.path !== currentFilePath) {
                return {
                    path: currentFilePath,
                    mode: defaultRenderMode,
                };
            }

            if (defaultRenderMode === "raw" && previous.mode !== "raw") {
                return {
                    path: currentFilePath,
                    mode: "raw",
                };
            }

            return previous;
        });
    }, [currentFilePath, defaultRenderMode]);

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
                        <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.mermaid", "Mermaid プレビューを読み込み中...")}
                        rendererName="Mermaid"
                    >
                        <LazyMermaidRenderer code={content.content}/>
                    </RendererSurface>
                );
            case "swagger":
                return (
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.swagger", "Swagger プレビューを読み込み中...")}
                        rendererName="Swagger"
                    >
                        <LazySwaggerRenderer content={content.content} filePath={content.path}/>
                    </RendererSurface>
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
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.sqlite", "SQLite プレビューを読み込み中...")}
                        rendererName="SQLite"
                    >
                        <LazySqliteRenderer
                            filePath={content.path}
                            sessionKey={activeSessionKey}
                            sessionName={activeSession}
                        />
                    </RendererSurface>
                );
            case "graphviz":
                return (
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.graphviz", "Graphviz プレビューを読み込み中...")}
                        rendererName="Graphviz"
                    >
                        <LazyGraphvizRenderer code={content.content}/>
                    </RendererSurface>
                );
            case "markmap":
                return (
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.markmap", "Markmap プレビューを読み込み中...")}
                        rendererName="Markmap"
                    >
                        <LazyMarkmapRenderer code={content.content}/>
                    </RendererSurface>
                );
            case "wavedrom":
                return (
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.wavedrom", "WaveDrom プレビューを読み込み中...")}
                        rendererName="WaveDrom"
                    >
                        <LazyWavedromRenderer code={content.content}/>
                    </RendererSurface>
                );
            case "vega-lite":
                return (
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.vegaLite", "Vega-Lite プレビューを読み込み中...")}
                        rendererName="Vega-Lite"
                    >
                        <LazyVegaLiteRenderer code={content.content} kind="vega-lite"/>
                    </RendererSurface>
                );
            case "vega":
                return (
                    <RendererSurface
                        filePath={content.path}
                        loadingMessage={t("viewer.preview.loading.vega", "Vega プレビューを読み込み中...")}
                        rendererName="Vega"
                    >
                        <LazyVegaLiteRenderer code={content.content} kind="vega"/>
                    </RendererSurface>
                );
            case "yaml-json-raw":
                return null;
        }
    }, [activeSession, activeSessionKey, t]);

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="file-tree-view"
                title={t("viewer.fileTree.title", "ファイルツリー")}
                onClose={closeView}
                message={t("viewer.fileTree.noActiveSession", "アクティブなセッションがありません")}
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="file-tree-view"
                title={t("viewer.fileTree.title", "ファイルツリー")}
                onClose={closeView}
                onRefresh={loadRoot}
                message={error}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="file-tree-view"
            title={t("viewer.fileTree.title", "ファイルツリー")}
            onClose={closeView}
            onRefresh={loadRoot}
        >
            <div className="file-tree-body">
                {isRootLoading ? (
                    <div className="viewer-message">{t("viewer.fileTree.loading", "ファイルツリーを読み込み中...")}</div>
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
                                    renderMode={currentRenderMode}
                                    canPreview={canPreviewCurrentDocument}
                                    onRenderModeChange={handleRenderModeChange}
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
