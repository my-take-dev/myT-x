import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {useStore} from "zustand";
import {api} from "../../../../api";
import {useFileTreeActions} from "../../../../hooks/useFileTreeActions";
import {createFileTreeStore} from "../../../../stores/fileTreeStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {shouldIgnoreSessionMutation} from "../../../../utils/sessionGuard";
import type {FlatNode} from "../file-tree/fileTreeTypes";
import {flattenTree} from "../file-tree/treeUtils";
import {joinRelativePath, parentDirOf} from "./editorPathUtils";

export interface UseEditorResult {
    readonly activeSession: string | null;
    readonly activeSessionKey: string;
    readonly error: string | null;
    readonly flatNodes: readonly FlatNode[];
    readonly isRootLoading: boolean;
    readonly selectedPath: string | null;
    readonly clearSelection: () => void;
    readonly createDirectory: (parentDir: string, name: string) => Promise<EditorMutationResult<boolean>>;
    readonly createFile: (parentDir: string, name: string) => Promise<EditorMutationResult<string | null>>;
    readonly deleteItem: (path: string) => Promise<EditorMutationResult<boolean>>;
    readonly loadRoot: () => void;
    readonly renameItem: (path: string, newName: string) => Promise<EditorMutationResult<string | null>>;
    readonly selectFile: (path: string) => void;
    readonly toggleDir: (path: string) => void;
}

export interface EditorMutationResult<T> {
    readonly result: T;
    readonly refreshError: string | null;
}

export function useEditor(): UseEditorResult {
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSession = useTmuxStore((state) => state.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";
    const isMountedRef = useRef(true);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const [store] = useState(createFileTreeStore);

    const tree = useStore(store, (state) => state.tree);
    const expandedPaths = useStore(store, (state) => state.expandedPaths);
    const loadingPaths = useStore(store, (state) => state.loadingPaths);
    const selectedPath = useStore(store, (state) => state.selectedPath);
    const isRootLoading = useStore(store, (state) => state.isRootLoading);
    const error = useStore(store, (state) => state.error);

    const {loadRoot, refreshDirectory, selectFile, toggleDir} = useFileTreeActions(store, {
        activeSession,
        activeSessionKey,
        loadFileContent: false,
        autoRefreshExternalChanges: true,
    });

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    useEffect(() => {
        latestSessionKeyRef.current = activeSessionKey;
    }, [activeSessionKey]);

    const shouldIgnoreMutationResult = useCallback((capturedSessionKey: string): boolean => {
        return shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef);
    }, []);

    const refreshDirectoryAfterMutation = useCallback(
        async (
            capturedSessionKey: string,
            parentDir: string,
            actionName: string,
            refreshErrorPrefix: string,
            metadata: Record<string, unknown>,
        ) => {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return null;
            }
            try {
                await refreshDirectory(parentDir, {expandOnSuccess: parentDir !== ""});
                if (shouldIgnoreMutationResult(capturedSessionKey)) {
                    // A stale refresh after a session switch is intentionally ignored.
                    return null;
                }
                return null;
            } catch (err: unknown) {
                if (shouldIgnoreMutationResult(capturedSessionKey)) {
                    return null;
                }
                console.warn(`[editor] refreshDirectory after ${actionName} failed`, {...metadata, err});
                return `${refreshErrorPrefix}. ${toErrorMessage(err, "File tree refresh failed.")}`;
            }
        },
        [refreshDirectory, shouldIgnoreMutationResult],
    );

    const createFile = useCallback(async (parentDir: string, name: string) => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        if (!capturedSession) {
            throw new Error("No active session.");
        }
        if (!capturedSessionKey) {
            throw new Error("Active session key is unavailable.");
        }

        const path = joinRelativePath(parentDir, name);
        try {
            await api.DevPanelCreateFile(capturedSessionKey, path);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: null, refreshError: null};
            }
            const refreshError = await refreshDirectoryAfterMutation(
                capturedSessionKey,
                parentDir,
                "create file",
                "Created the item",
                {path, session: capturedSession},
            );
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: null, refreshError: null};
            }
            return {result: path, refreshError};
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: null, refreshError: null};
            }
            throw err;
        }
    }, [activeSession, activeSessionKey, refreshDirectoryAfterMutation, shouldIgnoreMutationResult]);

    const createDirectory = useCallback(async (parentDir: string, name: string) => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        if (!capturedSession) {
            throw new Error("No active session.");
        }
        if (!capturedSessionKey) {
            throw new Error("Active session key is unavailable.");
        }

        const path = joinRelativePath(parentDir, name);
        try {
            await api.DevPanelCreateDirectory(capturedSessionKey, path);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: false, refreshError: null};
            }
            const refreshError = await refreshDirectoryAfterMutation(
                capturedSessionKey,
                parentDir,
                "create directory",
                "Created the item",
                {path, session: capturedSession},
            );
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: false, refreshError: null};
            }
            return {result: true, refreshError};
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: false, refreshError: null};
            }
            throw err;
        }
    }, [activeSession, activeSessionKey, refreshDirectoryAfterMutation, shouldIgnoreMutationResult]);

    const renameItem = useCallback(async (path: string, newName: string) => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        if (!capturedSession) {
            throw new Error("No active session.");
        }
        if (!capturedSessionKey) {
            throw new Error("Active session key is unavailable.");
        }

        const parentDir = parentDirOf(path);
        const newPath = joinRelativePath(parentDir, newName);
        try {
            await api.DevPanelRenameFile(capturedSessionKey, path, newPath);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: null, refreshError: null};
            }
            const refreshError = await refreshDirectoryAfterMutation(
                capturedSessionKey,
                parentDir,
                "rename item",
                "Renamed the item",
                {
                    newPath,
                    path,
                    session: capturedSession,
                },
            );
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: null, refreshError: null};
            }
            return {result: newPath, refreshError};
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: null, refreshError: null};
            }
            throw err;
        }
    }, [activeSession, activeSessionKey, refreshDirectoryAfterMutation, shouldIgnoreMutationResult]);

    const deleteItem = useCallback(async (path: string) => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        if (!capturedSession) {
            throw new Error("No active session.");
        }
        if (!capturedSessionKey) {
            throw new Error("Active session key is unavailable.");
        }

        const parentDir = parentDirOf(path);
        try {
            await api.DevPanelDeleteFile(capturedSessionKey, path);
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: false, refreshError: null};
            }
            const refreshError = await refreshDirectoryAfterMutation(
                capturedSessionKey,
                parentDir,
                "delete item",
                "Deleted the item",
                {path, session: capturedSession},
            );
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: false, refreshError: null};
            }
            return {result: true, refreshError};
        } catch (err: unknown) {
            if (shouldIgnoreMutationResult(capturedSessionKey)) {
                return {result: false, refreshError: null};
            }
            throw err;
        }
    }, [activeSession, activeSessionKey, refreshDirectoryAfterMutation, shouldIgnoreMutationResult]);

    const flatNodes = useMemo(
        () => flattenTree(tree, expandedPaths, loadingPaths),
        [expandedPaths, loadingPaths, tree],
    );
    const clearSelection = useCallback(() => {
        store.getState().setSelectedPath(null);
    }, [store]);

    return {
        activeSession,
        activeSessionKey,
        error,
        flatNodes,
        isRootLoading,
        selectedPath,
        clearSelection,
        createDirectory,
        createFile,
        deleteItem,
        loadRoot,
        renameItem,
        selectFile,
        toggleDir,
    };
}
