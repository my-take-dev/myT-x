import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import type {DiffTreeNode, WorkingDiffFile, WorkingDiffResult} from "./diffViewTypes";
import type {BranchInfo, DiffSidebarMode, OperationType, StagingListItem} from "./sourceControlTypes";

function createDiffDirNode(name: string, path: string, depth: number, isExpanded: boolean): DiffTreeNode {
    return {
        name,
        path,
        isDir: true,
        depth,
        isExpanded,
    };
}

function createDiffFileNode(file: WorkingDiffFile, name: string, depth: number): DiffTreeNode {
    return {
        name,
        path: file.path,
        isDir: false,
        depth,
        file,
    };
}

function buildDiffTree(files: WorkingDiffFile[], expandedDirs: Set<string>): DiffTreeNode[] {
    const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path));

    const nodes: DiffTreeNode[] = [];
    const addedDirs = new Set<string>();

    for (const file of sortedFiles) {
        const parts = file.path.split("/");

        for (let i = 1; i < parts.length; i++) {
            const dirPath = parts.slice(0, i).join("/");
            if (addedDirs.has(dirPath)) {
                continue;
            }

            const parentPath = parts.slice(0, i - 1).join("/");
            if (i > 1 && !expandedDirs.has(parentPath)) {
                continue;
            }

            addedDirs.add(dirPath);
            nodes.push(createDiffDirNode(parts[i - 1], dirPath, i - 1, expandedDirs.has(dirPath)));
        }

        const parentDir = parts.length > 1 ? parts.slice(0, -1).join("/") : "";
        if (parentDir === "" || expandedDirs.has(parentDir)) {
            nodes.push(createDiffFileNode(file, parts[parts.length - 1], parts.length - 1));
        }
    }

    return nodes;
}

function collectDirectorySet(files: WorkingDiffFile[]): Set<string> {
    const allDirs = new Set<string>();
    for (const file of files) {
        const parts = file.path.split("/");
        for (let i = 1; i < parts.length; i++) {
            allDirs.add(parts.slice(0, i).join("/"));
        }
    }
    return allDirs;
}

/** Builds a flat staging list with group headers from git status + diff data. */
function buildStagingItems(
    files: WorkingDiffFile[],
    staged: string[],
    expandStagedGroup: boolean,
    expandUnstagedGroup: boolean,
): {items: StagingListItem[]; stagedCount: number; unstagedCount: number} {
    const stagedSet = new Set(staged);
    const stagedFiles: WorkingDiffFile[] = [];
    const unstagedFiles: WorkingDiffFile[] = [];

    for (const file of files) {
        if (stagedSet.has(file.path)) {
            stagedFiles.push(file);
        } else {
            unstagedFiles.push(file);
        }
    }

    const items: StagingListItem[] = [];

    // Staged group header.
    items.push({type: "group-header", group: "staged", count: stagedFiles.length, isExpanded: expandStagedGroup});
    if (expandStagedGroup) {
        for (const file of stagedFiles) {
            items.push({type: "file", file, group: "staged"});
        }
    }

    // Unstaged group header.
    items.push({type: "group-header", group: "unstaged", count: unstagedFiles.length, isExpanded: expandUnstagedGroup});
    if (expandUnstagedGroup) {
        for (const file of unstagedFiles) {
            items.push({type: "file", file, group: "unstaged"});
        }
    }

    return {items, stagedCount: stagedFiles.length, unstagedCount: unstagedFiles.length};
}

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
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [diffResult, setDiffResult] = useState<WorkingDiffResult | null>(null);
    const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
    const [selectedPath, setSelectedPath] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const mountedRef = useRef(true);
    const sessionRef = useRef<string | null>(activeSession);
    const requestIDRef = useRef(0);

    useEffect(() => {
        sessionRef.current = activeSession;
    }, [activeSession]);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
            requestIDRef.current += 1;
        };
    }, []);

    // --- New state: sidebar mode, git ops, commit message ---
    const [sidebarMode, setSidebarMode] = useState<DiffSidebarMode>("tree");
    const [branchInfo, setBranchInfo] = useState<BranchInfo | null>(null);
    const [operationInFlight, setOperationInFlight] = useState<OperationType>(null);
    const operationRef = useRef<OperationType>(null);
    useEffect(() => {
        operationRef.current = operationInFlight;
    }, [operationInFlight]);
    const [commitMessage, setCommitMessage] = useState("");
    const [stagedPaths, setStagedPaths] = useState<string[]>([]);
    const [expandStagedGroup, setExpandStagedGroup] = useState(true);
    const [expandUnstagedGroup, setExpandUnstagedGroup] = useState(true);

    // Reset local state when session changes.
    useEffect(() => {
        setDiffResult(null);
        setExpandedDirs(new Set());
        setSelectedPath(null);
        setError(null);
        setIsLoading(false);
        // Reset git-related state (but preserve sidebarMode across sessions).
        setBranchInfo(null);
        setOperationInFlight(null);
        setCommitMessage("");
        setStagedPaths([]);
        setExpandStagedGroup(true);
        setExpandUnstagedGroup(true);
    }, [activeSession]);

    const loadDiff = useCallback((sessionName?: string, silent?: boolean) => {
        const targetSession = (sessionName ?? sessionRef.current)?.trim() ?? "";
        if (targetSession === "") {
            if (!mountedRef.current) {
                return;
            }
            setDiffResult(null);
            setExpandedDirs(new Set());
            setSelectedPath(null);
            setError("No active session");
            setIsLoading(false);
            return;
        }

        const requestID = requestIDRef.current + 1;
        requestIDRef.current = requestID;

        if (!silent) {
            setIsLoading(true);
        }
        setError(null);

        // Fetch both diff and git status in parallel.
        const diffPromise = api.DevPanelWorkingDiff(targetSession);
        const statusPromise = api.DevPanelGitStatus(targetSession).catch((statusErr: unknown) => {
            console.warn("[viewer/diff] DevPanelGitStatus failed (non-fatal)", statusErr);
            return null;
        });

        void Promise.all([diffPromise, statusPromise])
            .then(([result, status]) => {
                if (!mountedRef.current || requestIDRef.current !== requestID) {
                    return;
                }

                const files = result.files ?? [];
                if (files.length > 0) {
                    setExpandedDirs(collectDirectorySet(files));
                    setSelectedPath((prev) => {
                        if (prev && files.some((file) => file.path === prev)) {
                            return prev;
                        }
                        return files[0]?.path ?? null;
                    });
                } else {
                    setExpandedDirs(new Set());
                    setSelectedPath(null);
                }

                setDiffResult(result);

                // Update git status for flat view.
                if (status) {
                    setStagedPaths(status.staged ?? []);
                    setBranchInfo({
                        branch: status.branch ?? "",
                        ahead: status.ahead ?? 0,
                        behind: status.behind ?? 0,
                    });
                }
            })
            .catch((err: unknown) => {
                if (!mountedRef.current || requestIDRef.current !== requestID) {
                    return;
                }
                console.error("[viewer/diff] DevPanelWorkingDiff failed", {
                    session: targetSession,
                    err,
                });
                setDiffResult(null);
                setExpandedDirs(new Set());
                setSelectedPath(null);
                setError(toErrorMessage(err, "Failed to load diff."));
            })
            .finally(() => {
                if (!mountedRef.current || requestIDRef.current !== requestID) {
                    return;
                }
                setIsLoading(false);
            });
    }, []);

    // Load diff when active session changes.
    // Strict Mode double-effect is harmless: requestIDRef invalidates the first stale response.
    useEffect(() => {
        if (!activeSession) return;
        loadDiff(activeSession);
    }, [activeSession, loadDiff]);

    const toggleDir = useCallback((path: string) => {
        setExpandedDirs((prev) => {
            const next = new Set(prev);
            if (next.has(path)) {
                next.delete(path);
            } else {
                next.add(path);
            }
            return next;
        });
    }, []);

    const selectFile = useCallback((path: string) => {
        setSelectedPath(path);
    }, []);

    const flatNodes = useMemo(
        () => buildDiffTree(diffResult?.files ?? [], expandedDirs),
        [diffResult, expandedDirs],
    );

    const selectedFile = useMemo(
        () => diffResult?.files?.find((file) => file.path === selectedPath) ?? null,
        [diffResult, selectedPath],
    );

    // --- Flat view staging items ---
    const {items: stagingItems, stagedCount, unstagedCount} = useMemo(
        () => buildStagingItems(diffResult?.files ?? [], stagedPaths, expandStagedGroup, expandUnstagedGroup),
        [diffResult, stagedPaths, expandStagedGroup, expandUnstagedGroup],
    );

    const toggleStagingGroup = useCallback((group: "staged" | "unstaged") => {
        if (group === "staged") {
            setExpandStagedGroup((prev) => !prev);
        } else {
            setExpandUnstagedGroup((prev) => !prev);
        }
    }, []);

    // --- Git operation helpers ---
    const withOperation = useCallback(
        (op: NonNullable<OperationType>, fn: () => Promise<void>) => {
            return async () => {
                if (operationRef.current) return;
                setOperationInFlight(op);
                try {
                    await fn();
                    // Silent refresh after mutation to avoid screen flash.
                    loadDiff(undefined, true);
                } catch (err: unknown) {
                    console.error(`[viewer/diff] ${op} failed`, err);
                    setError(toErrorMessage(err, `${op} failed.`));
                } finally {
                    setOperationInFlight(null);
                }
            };
        },
        [loadDiff],
    );

    const stageFile = useCallback(
        async (path: string) => {
            if (operationRef.current) return;
            setOperationInFlight("stage");
            try {
                await api.DevPanelGitStage(sessionRef.current ?? "", path);
                // Optimistic update: move file to staged group immediately.
                setStagedPaths((prev) => prev.includes(path) ? prev : [...prev, path]);
                loadDiff(undefined, true);
            } catch (err: unknown) {
                console.error("[viewer/diff] stage failed", err);
                setError(toErrorMessage(err, "stage failed."));
            } finally {
                setOperationInFlight(null);
            }
        },
        [loadDiff],
    );

    const unstageFile = useCallback(
        async (path: string) => {
            if (operationRef.current) return;
            setOperationInFlight("unstage");
            try {
                await api.DevPanelGitUnstage(sessionRef.current ?? "", path);
                // Optimistic update: remove file from staged group immediately.
                setStagedPaths((prev) => prev.filter((p) => p !== path));
                loadDiff(undefined, true);
            } catch (err: unknown) {
                console.error("[viewer/diff] unstage failed", err);
                setError(toErrorMessage(err, "unstage failed."));
            } finally {
                setOperationInFlight(null);
            }
        },
        [loadDiff],
    );

    const discardFile = useCallback(
        (path: string) => withOperation("discard", () => api.DevPanelGitDiscard(sessionRef.current ?? "", path))(),
        [withOperation],
    );

    const stageAll = useCallback(
        () => withOperation("stageAll", () => api.DevPanelGitStageAll(sessionRef.current ?? ""))(),
        [withOperation],
    );

    const unstageAll = useCallback(
        () => withOperation("unstageAll", () => api.DevPanelGitUnstageAll(sessionRef.current ?? ""))(),
        [withOperation],
    );

    const commit = useCallback(
        async (message: string): Promise<boolean> => {
            if (operationRef.current) return false;
            setOperationInFlight("commit");
            try {
                await api.DevPanelGitCommit(sessionRef.current ?? "", message);
                setCommitMessage("");
                loadDiff(undefined, true);
                return true;
            } catch (err: unknown) {
                console.error("[viewer/diff] commit failed", err);
                setError(toErrorMessage(err, "Commit failed."));
                return false;
            } finally {
                setOperationInFlight(null);
            }
        },
        [loadDiff],
    );

    const commitAndPush = useCallback(
        async (message: string): Promise<boolean> => {
            if (operationRef.current) return false;
            setOperationInFlight("commit");
            try {
                await api.DevPanelGitCommit(sessionRef.current ?? "", message);
                setCommitMessage("");
            } catch (err: unknown) {
                console.error("[viewer/diff] commit failed", err);
                setError(toErrorMessage(err, "Commit failed."));
                setOperationInFlight(null);
                return false;
            }
            setOperationInFlight("push");
            try {
                await api.DevPanelGitPush(sessionRef.current ?? "");
                loadDiff(undefined, true);
                return true;
            } catch (err: unknown) {
                console.error("[viewer/diff] push failed (commit succeeded)", err);
                setError(toErrorMessage(err, "Push failed (commit was successful)."));
                loadDiff(undefined, true);
                return false;
            } finally {
                setOperationInFlight(null);
            }
        },
        [loadDiff],
    );

    const push = useCallback(
        () => withOperation("push", async () => { await api.DevPanelGitPush(sessionRef.current ?? ""); })(),
        [withOperation],
    );

    const pull = useCallback(
        () => withOperation("pull", async () => { await api.DevPanelGitPull(sessionRef.current ?? ""); })(),
        [withOperation],
    );

    const fetch_ = useCallback(
        () => withOperation("fetch", () => api.DevPanelGitFetch(sessionRef.current ?? ""))(),
        [withOperation],
    );

    return {
        // Existing (tree view)
        flatNodes,
        selectedPath,
        selectedFile,
        diffResult,
        isLoading,
        error,
        toggleDir,
        selectFile,
        loadDiff,
        activeSession,
        // Sidebar mode
        sidebarMode,
        setSidebarMode,
        // Flat view staging
        stagingItems,
        stagedCount,
        unstagedCount,
        branchInfo,
        toggleStagingGroup,
        // Git operations
        operationInFlight,
        stageFile,
        unstageFile,
        discardFile,
        stageAll,
        unstageAll,
        commit,
        commitAndPush,
        push,
        pull,
        fetch: fetch_,
        // Commit message
        commitMessage,
        setCommitMessage,
    };
}
