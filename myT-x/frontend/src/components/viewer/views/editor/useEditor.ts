import {useCallback, useMemo, useState} from "react";
import {useStore} from "zustand";
import {api} from "../../../../api";
import {useFileTreeActions} from "../../../../hooks/useFileTreeActions";
import {createFileTreeStore} from "../../../../stores/fileTreeStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {FlatNode} from "../file-tree/fileTreeTypes";
import {flattenTree} from "../file-tree/treeUtils";
import {joinRelativePath, parentDirOf} from "./editorPathUtils";

export interface UseEditorResult {
    readonly activeSession: string | null;
    readonly error: string | null;
    readonly flatNodes: readonly FlatNode[];
    readonly isRootLoading: boolean;
    readonly selectedPath: string | null;
    readonly clearSelection: () => void;
    readonly createDirectory: (parentDir: string, name: string) => Promise<string>;
    readonly createFile: (parentDir: string, name: string) => Promise<string>;
    readonly deleteItem: (path: string) => Promise<void>;
    readonly loadRoot: () => void;
    readonly renameItem: (path: string, newName: string) => Promise<string>;
    readonly selectFile: (path: string) => void;
    readonly toggleDir: (path: string) => void;
}

export function useEditor(): UseEditorResult {
    const activeSession = useTmuxStore((state) => state.activeSession);
    const [store] = useState(createFileTreeStore);

    const tree = useStore(store, (state) => state.tree);
    const expandedPaths = useStore(store, (state) => state.expandedPaths);
    const loadingPaths = useStore(store, (state) => state.loadingPaths);
    const selectedPath = useStore(store, (state) => state.selectedPath);
    const isRootLoading = useStore(store, (state) => state.isRootLoading);
    const error = useStore(store, (state) => state.error);

    const {loadRoot, refreshDirectory, selectFile, toggleDir} = useFileTreeActions(store, {
        activeSession,
        loadFileContent: false,
    });

    const createFile = useCallback(async (parentDir: string, name: string) => {
        const capturedSession = activeSession?.trim();
        if (!capturedSession) {
            throw new Error("No active session.");
        }

        const path = joinRelativePath(parentDir, name);
        await api.DevPanelCreateFile(capturedSession, path);
        await refreshDirectory(parentDir, {expandOnSuccess: parentDir !== ""});
        return path;
    }, [activeSession, refreshDirectory]);

    const createDirectory = useCallback(async (parentDir: string, name: string) => {
        const capturedSession = activeSession?.trim();
        if (!capturedSession) {
            throw new Error("No active session.");
        }

        const path = joinRelativePath(parentDir, name);
        await api.DevPanelCreateDirectory(capturedSession, path);
        await refreshDirectory(parentDir, {expandOnSuccess: parentDir !== ""});
        return path;
    }, [activeSession, refreshDirectory]);

    const renameItem = useCallback(async (path: string, newName: string) => {
        const capturedSession = activeSession?.trim();
        if (!capturedSession) {
            throw new Error("No active session.");
        }

        const parentDir = parentDirOf(path);
        const newPath = joinRelativePath(parentDir, newName);
        await api.DevPanelRenameFile(capturedSession, path, newPath);

        store.getState().renamePath(path, newPath);
        await refreshDirectory(parentDir, {expandOnSuccess: parentDir !== ""});
        return newPath;
    }, [activeSession, refreshDirectory, store]);

    const deleteItem = useCallback(async (path: string) => {
        const capturedSession = activeSession?.trim();
        if (!capturedSession) {
            throw new Error("No active session.");
        }

        const parentDir = parentDirOf(path);
        await api.DevPanelDeleteFile(capturedSession, path);

        store.getState().removePath(path);
        await refreshDirectory(parentDir, {expandOnSuccess: parentDir !== ""});
    }, [activeSession, refreshDirectory, store]);

    const flatNodes = useMemo(
        () => flattenTree(tree, expandedPaths, loadingPaths),
        [expandedPaths, loadingPaths, tree],
    );

    return {
        activeSession,
        error,
        flatNodes,
        isRootLoading,
        selectedPath,
        clearSelection: () => store.getState().setSelectedPath(null),
        createDirectory,
        createFile,
        deleteItem,
        loadRoot,
        renameItem,
        selectFile,
        toggleDir,
    };
}
