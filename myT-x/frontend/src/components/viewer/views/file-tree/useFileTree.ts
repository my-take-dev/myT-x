import {useMemo, useState} from "react";
import {useStore} from "zustand";
import {useFileTreeActions} from "../../../../hooks/useFileTreeActions";
import {createFileTreeStore} from "../../../../stores/fileTreeStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {FileContentResult, FileNode, FlatNode} from "./fileTreeTypes";
import {flattenTree} from "./treeUtils";

export interface UseFileTreeResult {
    readonly tree: readonly FileNode[];
    readonly expandedPaths: ReadonlySet<string>;
    readonly loadingPaths: ReadonlySet<string>;
    readonly flatNodes: readonly FlatNode[];
    readonly selectedPath: string | null;
    readonly fileContent: FileContentResult | null;
    readonly isLoadingContent: boolean;
    readonly isRootLoading: boolean;
    readonly error: string | null;
    readonly contentError: string | null;
    readonly dirError: string | null;
    readonly toggleDir: (path: string) => void;
    readonly selectFile: (path: string) => void;
    readonly loadRoot: () => void;
    readonly activeSession: string | null;
    readonly activeSessionKey: string;
}

export function useFileTree(): UseFileTreeResult {
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSession = useTmuxStore((state) => state.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";
    const [store] = useState(createFileTreeStore);

    const tree = useStore(store, (state) => state.tree);
    const expandedPaths = useStore(store, (state) => state.expandedPaths);
    const loadingPaths = useStore(store, (state) => state.loadingPaths);
    const selectedPath = useStore(store, (state) => state.selectedPath);
    const fileContent = useStore(store, (state) => state.fileContent);
    const isLoadingContent = useStore(store, (state) => state.isLoadingContent);
    const isRootLoading = useStore(store, (state) => state.isRootLoading);
    const error = useStore(store, (state) => state.error);
    const contentError = useStore(store, (state) => state.contentError);
    const dirError = useStore(store, (state) => state.dirError);

    const {loadRoot, selectFile, toggleDir} = useFileTreeActions(store, {
        activeSession,
        activeSessionKey,
        loadFileContent: true,
        autoRefreshExternalChanges: false,
    });

    const flatNodes = useMemo(
        () => flattenTree(tree, expandedPaths, loadingPaths),
        [expandedPaths, loadingPaths, tree],
    );

    return {
        tree,
        expandedPaths,
        loadingPaths,
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
        activeSessionKey,
    };
}
