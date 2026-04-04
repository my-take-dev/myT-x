import {useMemo, useState} from "react";
import {useStore} from "zustand";
import {useFileTreeActions} from "../../../../hooks/useFileTreeActions";
import {createFileTreeStore} from "../../../../stores/fileTreeStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {FileContentResult, FlatNode} from "./fileTreeTypes";
import {flattenTree} from "./treeUtils";

export interface UseFileTreeResult {
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
}

export function useFileTree(): UseFileTreeResult {
    const activeSession = useTmuxStore((state) => state.activeSession);
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
        loadFileContent: true,
    });

    const flatNodes = useMemo(
        () => flattenTree(tree, expandedPaths, loadingPaths),
        [expandedPaths, loadingPaths, tree],
    );

    return {
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
    };
}
