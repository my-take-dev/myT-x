import {createStore} from "zustand/vanilla";
import type {StoreApi} from "zustand/vanilla";
import type {FileContentResult, FileNode} from "../components/viewer/views/file-tree/fileTreeTypes";
import {
    isSameOrDescendantPath,
    mergeChildrenIntoTree,
    mergeRootNodes,
    removePathFromTree,
    renamePathInTree
} from "../components/viewer/views/file-tree/treeUtils";

interface FileTreeState {
    readonly tree: readonly FileNode[];
    readonly expandedPaths: ReadonlySet<string>;
    readonly loadingPaths: ReadonlySet<string>;
    readonly selectedPath: string | null;
    readonly fileContent: FileContentResult | null;
    readonly isLoadingContent: boolean;
    readonly isRootLoading: boolean;
    readonly error: string | null;
    readonly contentError: string | null;
    readonly dirError: string | null;
}

interface FileTreeActions {
    readonly reset: () => void;
    readonly setRootNodes: (nodes: readonly FileNode[]) => void;
    readonly setChildrenForPath: (path: string, children: readonly FileNode[]) => void;
    readonly removePath: (path: string) => void;
    readonly renamePath: (oldPath: string, newPath: string) => void;
    readonly setExpanded: (path: string, isExpanded: boolean) => void;
    readonly setLoadingPath: (path: string, isLoading: boolean) => void;
    readonly setSelectedPath: (path: string | null) => void;
    readonly setFileContent: (content: FileContentResult | null) => void;
    readonly setIsLoadingContent: (isLoading: boolean) => void;
    readonly setIsRootLoading: (isLoading: boolean) => void;
    readonly setError: (error: string | null) => void;
    readonly setContentError: (error: string | null) => void;
    readonly setDirError: (error: string | null) => void;
}

export type FileTreeStore = StoreApi<FileTreeState & FileTreeActions>;

function createInitialState(): FileTreeState {
    return {
        tree: [],
        expandedPaths: new Set(),
        loadingPaths: new Set(),
        selectedPath: null,
        fileContent: null,
        isLoadingContent: false,
        isRootLoading: false,
        error: null,
        contentError: null,
        dirError: null,
    };
}

function prunePathSet(paths: ReadonlySet<string>, targetPath: string): ReadonlySet<string> {
    const next = new Set<string>();

    for (const path of paths) {
        if (!isSameOrDescendantPath(path, targetPath)) {
            next.add(path);
        }
    }

    return next;
}

function remapPathValue(value: string | null, oldPath: string, newPath: string): string | null {
    if (!value) {
        return value;
    }
    if (value === oldPath) {
        return newPath;
    }
    const prefix = `${oldPath}/`;
    if (value.startsWith(prefix)) {
        return `${newPath}${value.slice(oldPath.length)}`;
    }
    return value;
}

function remapPathSet(paths: ReadonlySet<string>, oldPath: string, newPath: string): ReadonlySet<string> {
    const next = new Set<string>();

    for (const path of paths) {
        next.add(remapPathValue(path, oldPath, newPath) ?? path);
    }

    return next;
}

export function createFileTreeStore(): FileTreeStore {
    return createStore<FileTreeState & FileTreeActions>((set) => ({
        ...createInitialState(),
        reset: () => set(() => createInitialState()),
        setRootNodes: (nodes) =>
            set((state) => ({
                tree: mergeRootNodes(nodes, state.tree),
            })),
        setChildrenForPath: (path, children) =>
            set((state) => ({
                tree: mergeChildrenIntoTree(state.tree, path, children),
            })),
        removePath: (path) =>
            set((state) => ({
                tree: removePathFromTree(state.tree, path),
                expandedPaths: prunePathSet(state.expandedPaths, path),
                loadingPaths: prunePathSet(state.loadingPaths, path),
                selectedPath: state.selectedPath && isSameOrDescendantPath(state.selectedPath, path)
                    ? null
                    : state.selectedPath,
                fileContent: state.fileContent && isSameOrDescendantPath(state.fileContent.path, path)
                    ? null
                    : state.fileContent,
            })),
        renamePath: (oldPath, newPath) =>
            set((state) => ({
                tree: renamePathInTree(state.tree, oldPath, newPath),
                expandedPaths: remapPathSet(state.expandedPaths, oldPath, newPath),
                loadingPaths: remapPathSet(state.loadingPaths, oldPath, newPath),
                selectedPath: remapPathValue(state.selectedPath, oldPath, newPath),
                fileContent: state.fileContent
                    ? {
                        ...state.fileContent,
                        path: remapPathValue(state.fileContent.path, oldPath, newPath) ?? state.fileContent.path,
                    }
                    : null,
            })),
        setExpanded: (path, isExpanded) =>
            set((state) => {
                const next = new Set(state.expandedPaths);
                if (isExpanded) {
                    next.add(path);
                } else {
                    next.delete(path);
                }
                return {expandedPaths: next};
            }),
        setLoadingPath: (path, isLoading) =>
            set((state) => {
                const next = new Set(state.loadingPaths);
                if (isLoading) {
                    next.add(path);
                } else {
                    next.delete(path);
                }
                return {loadingPaths: next};
            }),
        setSelectedPath: (selectedPath) => set({selectedPath}),
        setFileContent: (fileContent) => set({fileContent}),
        setIsLoadingContent: (isLoadingContent) => set({isLoadingContent}),
        setIsRootLoading: (isRootLoading) => set({isRootLoading}),
        setError: (error) => set({error}),
        setContentError: (contentError) => set({contentError}),
        setDirError: (dirError) => set({dirError}),
    }));
}
