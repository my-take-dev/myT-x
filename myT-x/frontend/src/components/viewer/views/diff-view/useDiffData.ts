import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import type {Dispatch, MutableRefObject, SetStateAction} from "react";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {createConsecutiveFailureCounter, notifyAndLog} from "../../../../utils/notifyUtils";
import type {devpanel} from "../../../../../wailsjs/go/models";
import type {DiffTreeNode, WorkingDiffFile, WorkingDiffResult} from "./diffViewTypes";
import type {BranchInfo, StagingListItem} from "./sourceControlTypes";

/** Background polling interval (ms) as a safety net for changes made in other tools.
 *  Primary refresh sources are window focus and post-operation silent reloads;
 *  this interval is kept long to minimize unnecessary API calls. */
const BACKGROUND_POLL_INTERVAL_MS = 15_000;

// Module-level consecutive failure counter for GitStatus polling.
// Threshold 3: transient failures are silently recovered; persistent failures
// (e.g., git process hang, disk error) surface a user-visible toast + error log.
const gitStatusFailureCounter = createConsecutiveFailureCounter(3);

/** Compile-time field guard: satisfies ensures a compile error when Go adds fields to GitStatusResult.
 *  Used as the initial fallback in setBranchInfo when no previous state exists. */
const EMPTY_GIT_STATUS: devpanel.GitStatusResult = {
    modified: [],
    staged: [],
    untracked: [],
    conflicted: [],
    branch: "",
    ahead: 0,
    behind: 0,
    upstream_configured: false,
} satisfies devpanel.GitStatusResult;

// --- Pure utility functions for tree/staging data transformation ---

function createDiffDirNode(name: string, path: string, depth: number, isExpanded: boolean): DiffTreeNode {
    return {name, path, isDir: true, depth, isExpanded};
}

function createDiffFileNode(file: WorkingDiffFile, name: string, depth: number): DiffTreeNode {
    return {name, path: file.path, isDir: false, depth, file};
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

// --- Hook return types ---

/** Public API consumed by UI components (e.g. DiffView). */
export interface UseDiffDataResult {
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
    readonly branchInfo: BranchInfo | null;
    readonly stagingItems: readonly StagingListItem[];
    readonly stagedCount: number;
    readonly unstagedCount: number;
    readonly toggleStagingGroup: (group: "staged" | "unstaged") => void;
}

/**
 * Internal fields exposed only for useGitOperations coupling.
 * Do not consume these in UI components — use UseGitOperationsParams instead.
 */
export interface DiffDataInternals {
    readonly sessionRef: MutableRefObject<string | null>;
    readonly setError: Dispatch<SetStateAction<string | null>>;
    /** Shared ref so useDiffData can skip auto-refresh while an operation is in flight. */
    readonly operationActiveRef: MutableRefObject<boolean>;
    /** Lightweight status-only refresh: updates stagedPaths and branchInfo
     *  without re-fetching the expensive WorkingDiff payload.
     *  Does NOT increment requestIDRef — a concurrent loadDiff always wins.
     *  Returns a Promise that rejects on failure so callers can fall back to loadDiff. */
    readonly refreshStatus: () => Promise<void>;
    /** Direct setter for optimistic staging updates (e.g. move file between groups instantly). */
    readonly setStagedPaths: Dispatch<SetStateAction<string[]>>;
}

// --- Hook ---

export function useDiffData(): UseDiffDataResult & DiffDataInternals {
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [diffResult, setDiffResult] = useState<WorkingDiffResult | null>(null);
    const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
    const [selectedPath, setSelectedPath] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [branchInfo, setBranchInfo] = useState<BranchInfo | null>(null);
    const [stagedPaths, setStagedPaths] = useState<string[]>([]);
    const [expandStagedGroup, setExpandStagedGroup] = useState(true);
    const [expandUnstagedGroup, setExpandUnstagedGroup] = useState(true);

    const mountedRef = useRef(true);
    const sessionRef = useRef<string | null>(activeSession);
    const requestIDRef = useRef(0);
    const operationActiveRef = useRef(false);

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

    // Reset state when session changes.
    // ORDER DEPENDENCY: useGitOperations also resets on activeSession change.
    // React does not guarantee inter-hook useEffect ordering, but
    // requestIDRef-based stale response rejection ensures correctness
    // regardless of which reset runs first.
    useEffect(() => {
        setDiffResult(null);
        setExpandedDirs(new Set());
        setSelectedPath(null);
        setError(null);
        setIsLoading(false);
        setBranchInfo(null);
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
        const statusPromise = api.DevPanelGitStatus(targetSession).then((status) => {
            gitStatusFailureCounter.recordSuccess();
            return {status, failed: false};
        }).catch((statusErr: unknown) => {
            console.warn("[viewer/diff] DevPanelGitStatus failed (non-fatal)", statusErr);
            gitStatusFailureCounter.recordFailure(() => {
                notifyAndLog("Load git status", "warn", statusErr, "DiffView");
            });
            // Return null to signal failure — the caller preserves previous branch info
            // (stale data > empty data) and sets statusFetchFailed flag.
            return {status: null as devpanel.GitStatusResult | null, failed: true};
        });

        void Promise.all([diffPromise, statusPromise])
            .then(([result, {status, failed: statusFetchFailed}]) => {
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

                // Map GitStatusResult (snake_case) to BranchInfo (camelCase).
                // When adding fields to GitStatusResult, update this mapping
                // AND the BranchInfo interface in sourceControlTypes.ts.
                if (status != null) {
                    setStagedPaths(status.staged ?? []);
                    setBranchInfo({
                        branch: status.branch ?? "",
                        ahead: status.ahead ?? 0,
                        behind: status.behind ?? 0,
                        upstreamConfigured: status.upstream_configured ?? false,
                        conflicted: status.conflicted ?? [],
                        statusFetchFailed: false,
                    });
                } else {
                    // Status fetch failed — preserve previous branch info (stale data > empty data)
                    // and set the statusFetchFailed flag so the UI can show a warning.
                    setBranchInfo((prev) => prev
                        ? {...prev, statusFetchFailed: true}
                        : {branch: "", ahead: 0, behind: 0, upstreamConfigured: false, conflicted: [], statusFetchFailed: true},
                    );
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

    // Lightweight status-only refresh for stage/unstage operations.
    // For non-fresh repos, git diff HEAD output is unchanged by staging —
    // only git status (staged paths) changes, so we skip the expensive WorkingDiff fetch.
    //
    // Does NOT increment requestIDRef — only reads it for stale-response rejection.
    // This ensures a concurrent loadDiff (which carries more data) is never invalidated
    // by a lightweight refreshStatus call, and isLoading is never left stuck.
    // Conversely, if loadDiff fires while refreshStatus is pending, the loadDiff
    // increments requestIDRef, causing refreshStatus to discard its stale response.
    //
    // Returns a Promise so the caller can fall back to full loadDiff on failure
    // (important when optimistic UI updates were applied before this call).
    const refreshStatus = useCallback((): Promise<void> => {
        const targetSession = sessionRef.current?.trim() ?? "";
        if (targetSession === "") return Promise.resolve();

        // Snapshot current requestID without incrementing — loadDiff wins on conflict.
        const requestID = requestIDRef.current;

        return api.DevPanelGitStatus(targetSession)
            .then((status) => {
                gitStatusFailureCounter.recordSuccess();
                if (!mountedRef.current || requestIDRef.current !== requestID) return;
                setStagedPaths(status.staged ?? []);
                setBranchInfo({
                    branch: status.branch ?? "",
                    ahead: status.ahead ?? 0,
                    behind: status.behind ?? 0,
                    upstreamConfigured: status.upstream_configured ?? false,
                    conflicted: status.conflicted ?? [],
                    statusFetchFailed: false,
                });
            })
            .catch((err: unknown) => {
                console.warn("[viewer/diff] refreshStatus failed (non-fatal)", {session: targetSession, err});
                gitStatusFailureCounter.recordFailure(() => {
                    notifyAndLog("Refresh git status", "warn", err, "DiffView");
                });
                if (!mountedRef.current || requestIDRef.current !== requestID) return;
                setBranchInfo((prev) => prev
                    ? {...prev, statusFetchFailed: true}
                    : {branch: "", ahead: 0, behind: 0, upstreamConfigured: false, conflicted: [], statusFetchFailed: true},
                );
                // Re-throw so the caller can detect failure and fall back to full loadDiff.
                throw err;
            });
    }, []);

    // Load diff when active session changes.
    // React 18 Strict Mode re-runs effects in development to surface cleanup bugs.
    // The double invocation is harmless here: requestIDRef invalidates the first stale response.
    useEffect(() => {
        if (!activeSession) return;
        loadDiff(activeSession);
    }, [activeSession, loadDiff]);

    // Auto-refresh on window focus — covers the common case of switching
    // back from an external editor or terminal.
    // Skipped when an operation is in flight to avoid UI flicker from stale data.
    useEffect(() => {
        const handleFocus = () => {
            if (sessionRef.current && !operationActiveRef.current) {
                loadDiff(undefined, true);
            }
        };
        window.addEventListener("focus", handleFocus);
        return () => window.removeEventListener("focus", handleFocus);
    }, [loadDiff]);

    // Background polling as a safety net for changes made in other tools.
    // Skipped when an operation is in flight to avoid UI flicker.
    useEffect(() => {
        if (!activeSession) return;
        const timer = setInterval(() => {
            if (!operationActiveRef.current) {
                loadDiff(undefined, true);
            }
        }, BACKGROUND_POLL_INTERVAL_MS);
        return () => clearInterval(timer);
    }, [activeSession, loadDiff]);

    const toggleDir = useCallback((path: string) => {
        setExpandedDirs((prev) => {
            const next = new Set(prev);
            if (next.has(path)) {
                // Remove this directory and all descendants so re-expanding
                // shows children collapsed (not in their previous open state).
                const prefix = path + "/";
                next.delete(path);
                for (const p of prev) {
                    if (p.startsWith(prefix)) {
                        next.delete(p);
                    }
                }
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

    return {
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
        branchInfo,
        stagingItems,
        stagedCount,
        unstagedCount,
        toggleStagingGroup,
        sessionRef,
        setError,
        operationActiveRef,
        refreshStatus,
        setStagedPaths,
    };
}
