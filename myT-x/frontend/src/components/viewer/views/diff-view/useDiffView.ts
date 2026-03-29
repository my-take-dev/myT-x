import {useState} from "react";
import type {DiffTreeNode, WorkingDiffFile, WorkingDiffResult} from "./diffViewTypes";
import type {BranchInfo, DiffSidebarMode, OperationType, StagingListItem} from "./sourceControlTypes";
import {useDiffData} from "./useDiffData";
import {useGitOperations} from "./useGitOperations";

export interface UseDiffViewResult {
    // --- Existing (tree view) ---
    readonly flatNodes: readonly DiffTreeNode[];
    readonly selectedPath: string | null;
    readonly selectedFile: WorkingDiffFile | null;
    readonly diffResult: WorkingDiffResult | null;
    readonly isLoading: boolean;
    readonly error: string | null;
    readonly toggleDir: (path: string) => void;
    readonly selectFile: (path: string) => void;
    readonly loadDiff: (sessionName?: string, silent?: boolean) => void;
    readonly activeSession: string | null;

    // --- New: sidebar mode ---
    readonly sidebarMode: DiffSidebarMode;
    readonly setSidebarMode: (mode: DiffSidebarMode) => void;

    // --- New: flat view staging data ---
    readonly stagingItems: readonly StagingListItem[];
    readonly stagedCount: number;
    readonly unstagedCount: number;
    readonly branchInfo: BranchInfo | null;
    readonly toggleStagingGroup: (group: "staged" | "unstaged") => void;

    // --- New: git operations ---
    readonly operationInFlight: OperationType;
    readonly stageFile: (path: string) => Promise<void>;
    readonly unstageFile: (path: string) => Promise<void>;
    readonly discardFile: (path: string) => Promise<void>;
    readonly stageAll: () => Promise<void>;
    readonly unstageAll: () => Promise<void>;
    readonly commit: (message: string) => Promise<boolean>;
    readonly commitAndPush: (message: string) => Promise<boolean>;
    readonly push: () => Promise<void>;
    readonly pull: () => Promise<void>;
    readonly fetch: () => Promise<void>;

    // --- New: commit message ---
    readonly commitMessage: string;
    readonly setCommitMessage: (msg: string) => void;
}

export function useDiffView(): UseDiffViewResult {
    const [sidebarMode, setSidebarMode] = useState<DiffSidebarMode>("tree");

    const data = useDiffData();
    const git = useGitOperations({
        activeSession: data.activeSession,
        sessionRef: data.sessionRef,
        setError: data.setError,
        loadDiff: data.loadDiff,
        operationActiveRef: data.operationActiveRef,
        refreshStatus: data.refreshStatus,
        setStagedPaths: data.setStagedPaths,
        branchInfo: data.branchInfo,
    });

    return {
        // Existing (tree view)
        flatNodes: data.flatNodes,
        selectedPath: data.selectedPath,
        selectedFile: data.selectedFile,
        diffResult: data.diffResult,
        isLoading: data.isLoading,
        error: data.error,
        toggleDir: data.toggleDir,
        selectFile: data.selectFile,
        loadDiff: data.loadDiff,
        activeSession: data.activeSession,
        // Sidebar mode
        sidebarMode,
        setSidebarMode,
        // Flat view staging
        stagingItems: data.stagingItems,
        stagedCount: data.stagedCount,
        unstagedCount: data.unstagedCount,
        branchInfo: data.branchInfo,
        toggleStagingGroup: data.toggleStagingGroup,
        // Git operations
        operationInFlight: git.operationInFlight,
        stageFile: git.stageFile,
        unstageFile: git.unstageFile,
        discardFile: git.discardFile,
        stageAll: git.stageAll,
        unstageAll: git.unstageAll,
        commit: git.commit,
        commitAndPush: git.commitAndPush,
        push: git.push,
        pull: git.pull,
        fetch: git.fetch,
        // Commit message
        commitMessage: git.commitMessage,
        setCommitMessage: git.setCommitMessage,
    };
}
