import {useCallback, useEffect, useRef, useState} from "react";
import {useViewerStore} from "../../viewerStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import type {FlatNode} from "../file-tree/fileTreeTypes";
import {FileSearchPanel} from "../file-tree/FileSearchPanel";
import {useFileSearch} from "../file-tree/useFileSearch";
import {DeleteDialog, CreateDialog, DiscardChangesDialog, RenameDialog} from "./EditorDialogs";
import {EditorFileTree} from "./EditorFileTree";
import {EditorPane} from "./EditorPane";
import {useEditor} from "./useEditor";
import {useEditorFile} from "./useEditorFile";

interface CreateDialogState {
    readonly parentDir: string;
    readonly type: "directory" | "file";
}

interface RenameDialogState {
    readonly name: string;
    readonly path: string;
}

interface DeleteDialogState {
    readonly isDir: boolean;
    readonly name: string;
    readonly path: string;
}

export function EditorView() {
    const closeView = useViewerStore((state) => state.closeView);
    const {
        activeSession,
        error,
        flatNodes,
        isRootLoading,
        selectedPath,
        createDirectory,
        createFile,
        deleteItem,
        loadRoot,
        renameItem,
        selectFile,
        toggleDir,
    } = useEditor();
    const {
        clearFile,
        currentPath,
        detectedLanguage,
        error: editorError,
        fileSize,
        handleChange,
        handleEditorMount,
        isModified,
        loadFile,
        loadingState,
        readOnly,
        saveFile,
        truncated,
    } = useEditorFile();

    const {query, setQuery, results, isSearching, searchError, clearSearch} = useFileSearch();
    const [isSearchMode, setIsSearchMode] = useState(false);

    const [createDialog, setCreateDialog] = useState<CreateDialogState | null>(null);
    const [renameDialog, setRenameDialog] = useState<RenameDialogState | null>(null);
    const [deleteDialog, setDeleteDialog] = useState<DeleteDialogState | null>(null);
    const [showDiscardDialog, setShowDiscardDialog] = useState(false);
    const [dialogError, setDialogError] = useState<string | null>(null);
    const pendingDiscardActionRef = useRef<(() => void | Promise<void>) | null>(null);
    const isInitialRootLoad = isRootLoading && flatNodes.length === 0;
    const isTreeRefreshing = isRootLoading && flatNodes.length > 0;

    useEffect(() => {
        if (selectedPath) {
            void loadFile(selectedPath);
            return;
        }
        clearFile();
    }, [clearFile, loadFile, selectedPath]);

    useEffect(() => {
        setCreateDialog(null);
        setRenameDialog(null);
        setDeleteDialog(null);
        setShowDiscardDialog(false);
        pendingDiscardActionRef.current = null;
        setDialogError(null);
        setIsSearchMode(false);
        clearSearch();
    }, [activeSession, clearSearch]);

    const confirmDiscardUnsavedChanges = useCallback((action: () => void | Promise<void>) => {
        if (!isModified) {
            return true;
        }
        pendingDiscardActionRef.current = action;
        setShowDiscardDialog(true);
        return false;
    }, [isModified]);

    const handleCancelDiscard = useCallback(() => {
        pendingDiscardActionRef.current = null;
        setShowDiscardDialog(false);
    }, []);

    const handleConfirmDiscard = useCallback(async () => {
        const action = pendingDiscardActionRef.current;
        pendingDiscardActionRef.current = null;
        setShowDiscardDialog(false);
        if (!action) {
            return;
        }
        try {
            await action();
        } catch (err: unknown) {
            setDialogError(toErrorMessage(err, "Failed to continue after discarding changes."));
        }
    }, []);

    const handleSelectFile = useCallback((path: string) => {
        if (path === currentPath) return;
        if (!confirmDiscardUnsavedChanges(() => {
            selectFile(path);
        })) {
            return;
        }
        selectFile(path);
    }, [confirmDiscardUnsavedChanges, currentPath, selectFile]);

    // ── Search mode handlers ──
    // NOTE: Ctrl+F shortcut is intentionally omitted — Monaco Editor uses it for find-in-file.

    const handleSearchOpen = useCallback(() => setIsSearchMode(true), []);

    const handleSearchClose = useCallback(() => {
        setIsSearchMode(false);
        clearSearch();
    }, [clearSearch]);

    const handleOpenFileFromSearch = useCallback((path: string) => {
        if (path === currentPath) {
            setIsSearchMode(false);
            return;
        }
        if (!confirmDiscardUnsavedChanges(() => {
            selectFile(path);
            setIsSearchMode(false);
        })) {
            return;
        }
        selectFile(path);
        setIsSearchMode(false);
    }, [confirmDiscardUnsavedChanges, currentPath, selectFile]);

    const handleRequestCreateFile = useCallback((parentDir: string) => {
        setDialogError(null);
        setCreateDialog({parentDir, type: "file"});
    }, []);

    const handleRequestCreateDirectory = useCallback((parentDir: string) => {
        setDialogError(null);
        setCreateDialog({parentDir, type: "directory"});
    }, []);

    const handleRequestRename = useCallback((node: FlatNode) => {
        setDialogError(null);
        setRenameDialog({name: node.name, path: node.path});
    }, []);

    const handleRequestDelete = useCallback((node: FlatNode) => {
        setDialogError(null);
        setDeleteDialog({isDir: node.isDir, name: node.name, path: node.path});
    }, []);

    const handleConfirmCreate = useCallback(async (name: string) => {
        if (!createDialog) {
            return;
        }

        try {
            setDialogError(null);
            if (createDialog.type === "file") {
                const createdPath = await createFile(createDialog.parentDir, name);
                setCreateDialog(null);
                if (confirmDiscardUnsavedChanges(() => {
                    selectFile(createdPath);
                })) {
                    selectFile(createdPath);
                }
                return;
            }

            await createDirectory(createDialog.parentDir, name);
            setCreateDialog(null);
        } catch (err: unknown) {
            setDialogError(toErrorMessage(err, `Failed to create ${createDialog.type}.`));
        }
    }, [confirmDiscardUnsavedChanges, createDialog, createDirectory, createFile, selectFile]);

    const handleConfirmRename = useCallback(async (name: string) => {
        if (!renameDialog) {
            return;
        }

        try {
            setDialogError(null);
            if (renameDialog.path === currentPath && !confirmDiscardUnsavedChanges(async () => {
                try {
                    await renameItem(renameDialog.path, name);
                    setRenameDialog(null);
                } catch (err: unknown) {
                    setDialogError(toErrorMessage(err, "Failed to rename item."));
                }
            })) {
                return;
            }
            await renameItem(renameDialog.path, name);
            setRenameDialog(null);
        } catch (err: unknown) {
            setDialogError(toErrorMessage(err, "Failed to rename item."));
        }
    }, [confirmDiscardUnsavedChanges, currentPath, renameDialog, renameItem]);

    const handleConfirmDelete = useCallback(async () => {
        if (!deleteDialog) {
            return;
        }

        try {
            setDialogError(null);
            if (deleteDialog.path === currentPath && !confirmDiscardUnsavedChanges(async () => {
                try {
                    await deleteItem(deleteDialog.path);
                    setDeleteDialog(null);
                } catch (err: unknown) {
                    setDialogError(toErrorMessage(err, "Failed to delete item."));
                }
            })) {
                return;
            }
            await deleteItem(deleteDialog.path);
            setDeleteDialog(null);
        } catch (err: unknown) {
            setDialogError(toErrorMessage(err, "Failed to delete item."));
        }
    }, [confirmDiscardUnsavedChanges, currentPath, deleteDialog, deleteItem]);

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="editor-view"
                title="Editor"
                onClose={closeView}
                message="No active session"
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="editor-view"
                title="Editor"
                onClose={closeView}
                onRefresh={loadRoot}
                message={error}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="editor-view"
            title="Editor"
            onClose={closeView}
            onRefresh={loadRoot}
        >
            <div className="editor-body">
                {isInitialRootLoad ? (
                    <div className="viewer-message">Loading editor tree...</div>
                ) : (
                    <>
                        {isSearchMode ? (
                            <FileSearchPanel
                                query={query}
                                onQueryChange={setQuery}
                                results={results}
                                isSearching={isSearching}
                                searchError={searchError}
                                selectedPath={selectedPath}
                                onSelectFile={handleSelectFile}
                                onOpenFile={handleOpenFileFromSearch}
                                onClose={handleSearchClose}
                            />
                        ) : (
                            <EditorFileTree
                                flatNodes={flatNodes}
                                isRefreshing={isTreeRefreshing}
                                selectedPath={selectedPath}
                                onRefresh={loadRoot}
                                onRequestCreateDirectory={handleRequestCreateDirectory}
                                onRequestCreateFile={handleRequestCreateFile}
                                onRequestDelete={handleRequestDelete}
                                onRequestRename={handleRequestRename}
                                onSearchOpen={handleSearchOpen}
                                onSelectFile={handleSelectFile}
                                onToggleDir={toggleDir}
                            />
                        )}
                        <EditorPane
                            currentPath={currentPath}
                            detectedLanguage={detectedLanguage}
                            error={editorError}
                            fileSize={fileSize}
                            isModified={isModified}
                            loadingState={loadingState}
                            readOnly={readOnly}
                            truncated={truncated}
                            onChange={handleChange}
                            onEditorMount={handleEditorMount}
                            onSave={saveFile}
                        />
                    </>
                )}
            </div>
            {createDialog && (
                <CreateDialog
                    errorMessage={dialogError}
                    parentPath={createDialog.parentDir}
                    type={createDialog.type}
                    onCancel={() => {
                        setCreateDialog(null);
                        setDialogError(null);
                    }}
                    onConfirm={handleConfirmCreate}
                />
            )}
            {renameDialog && (
                <RenameDialog
                    currentName={renameDialog.name}
                    errorMessage={dialogError}
                    onCancel={() => {
                        setRenameDialog(null);
                        setDialogError(null);
                    }}
                    onConfirm={handleConfirmRename}
                />
            )}
            {deleteDialog && (
                <DeleteDialog
                    errorMessage={dialogError}
                    isDir={deleteDialog.isDir}
                    name={deleteDialog.name}
                    onCancel={() => {
                        setDeleteDialog(null);
                        setDialogError(null);
                    }}
                    onConfirm={handleConfirmDelete}
                />
            )}
            {showDiscardDialog && (
                <DiscardChangesDialog
                    onCancel={handleCancelDiscard}
                    onConfirm={handleConfirmDiscard}
                />
            )}
        </ViewerPanelShell>
    );
}
