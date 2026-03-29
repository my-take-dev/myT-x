import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {createConsecutiveFailureCounter, notifyAndLog} from "../../../../utils/notifyUtils";
import {
    normalizeGitGraphCommits,
    type GitGraphCommit,
    type GitStatusResult,
    type LaneAssignment
} from "./gitGraphTypes";
import {computeLanes} from "./laneComputation";

const DEFAULT_LOG_COUNT = 100;

// Module-level consecutive failure counter for git graph data loading.
// loadData fires on session change, branch toggle, and focus events,
// so transient failures should be silently recovered. Only persistent
// failures (3+ consecutive) trigger a user notification.
const gitGraphFailureCounter = createConsecutiveFailureCounter(3);

/** Maximum number of commits to load. Matches the UI's "Load More" cap in CommitGraph. */
export const MAX_LOG_COUNT = 1000;

export interface UseGitGraphResult {
    readonly commits: readonly GitGraphCommit[];
    readonly laneAssignments: readonly LaneAssignment[];
    readonly status: GitStatusResult | null;
    readonly selectedCommit: GitGraphCommit | null;
    readonly diff: string | null;
    readonly diffError: string | null;
    readonly isLoadingDiff: boolean;
    readonly allBranches: boolean;
    readonly isLoading: boolean;
    readonly error: string | null;
    readonly logCount: number;
    readonly selectCommit: (commit: GitGraphCommit) => void;
    readonly loadMore: () => void;
    readonly loadData: () => void;
    readonly toggleAllBranches: (value: boolean) => void;
    readonly activeSession: string | null;
}

// Branch list fetching removed; not consumed by GitGraphView.
// Re-add when branch visualization UI is implemented.

export function useGitGraph(): UseGitGraphResult {
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [commits, setCommits] = useState<readonly GitGraphCommit[]>([]);
    const [status, setStatus] = useState<GitStatusResult | null>(null);
    const [selectedCommit, setSelectedCommit] = useState<GitGraphCommit | null>(null);
    const [diff, setDiff] = useState<string | null>(null);
    const [diffError, setDiffError] = useState<string | null>(null);
    const [isLoadingDiff, setIsLoadingDiff] = useState(false);
    const [allBranches, setAllBranches] = useState(false);
    const [logCount, setLogCount] = useState(DEFAULT_LOG_COUNT);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const prevSessionRef = useRef<string | null>(null);
    const mountedRef = useRef(true);
    const sessionRef = useRef(activeSession);
    const loadRequestRef = useRef(0);
    const selectRequestRef = useRef(0);

    // Synchronize sessionRef during render. This is an idempotent mutation:
    // the same value is written on every render, so multiple invocations in
    // StrictMode or Concurrent Mode are safe.
    // Avoids the one-render delay and declaration-order fragility of useEffect.
    // NOTE: Add useEffect here if useTransition/Suspense is introduced, as
    // speculative renders that don't commit can leave stale ref values.
    sessionRef.current = activeSession;

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
            loadRequestRef.current += 1;
            selectRequestRef.current += 1;
        };
    }, []);

    useEffect(() => {
        if (prevSessionRef.current === activeSession) return;
        prevSessionRef.current = activeSession;
        loadRequestRef.current += 1;
        selectRequestRef.current += 1;
        setCommits([]);
        setStatus(null);
        // Session switch: clear commit selection and diff so stale details
        // from the previous session are never shown.
        setSelectedCommit(null);
        setDiff(null);
        setDiffError(null);
        setIsLoadingDiff(false);
        setAllBranches(false);
        setLogCount(DEFAULT_LOG_COUNT);
        setIsLoading(false);
        setError(null);
    }, [activeSession]);

    const loadData = useCallback(() => {
        const capturedSession = sessionRef.current?.trim();
        const requestId = loadRequestRef.current + 1;
        loadRequestRef.current = requestId;
        // Invalidate any in-flight selectCommit diff request — the commit list is being
        // reloaded, so stale diff results should be discarded.
        selectRequestRef.current += 1;

        if (!capturedSession) {
            setSelectedCommit(null);
            setDiff(null);
            setDiffError(null);
            setIsLoadingDiff(false);
            setIsLoading(false);
            setError("No active session");
            return;
        }

        setSelectedCommit(null);
        setDiff(null);
        setDiffError(null);
        setIsLoadingDiff(false);
        setIsLoading(true);
        setError(null);

        const isCurrentRequest = (): boolean => {
            return mountedRef.current
                && sessionRef.current?.trim() === capturedSession
                && loadRequestRef.current === requestId;
        };

        void (async () => {
            try {
                const [logResult, statusResult] = await Promise.allSettled([
                    api.DevPanelGitLog(capturedSession, logCount, allBranches),
                    api.DevPanelGitStatus(capturedSession),
                ]);

                if (!isCurrentRequest()) return;

                const loadFailures: string[] = [];
                if (logResult.status === "fulfilled") {
                    setCommits(normalizeGitGraphCommits(logResult.value));
                } else {
                    const reason = toErrorMessage(logResult.reason, "Unknown error");
                    console.error("[git-graph] DevPanelGitLog failed", reason);
                    // On refresh failure, keep existing commits visible so the user
                    // doesn't lose their context. The error banner will indicate the issue.
                    // On initial load, commits are already empty (session-change reset).
                    loadFailures.push(`git log: ${reason}`);
                }

                if (statusResult.status === "fulfilled") {
                    setStatus(statusResult.value);
                } else {
                    const reason = toErrorMessage(statusResult.reason, "Unknown error");
                    console.warn("[git-graph] DevPanelGitStatus failed", reason);
                    loadFailures.push(`git status: ${reason}`);
                }

                // Determine the primary error to display.
                // Log failure is fatal (no commits to show); status failure is
                // non-fatal but should still be surfaced so users know the data
                // may be stale (e.g., backend disconnect).
                if (logResult.status === "rejected") {
                    setError(toErrorMessage(logResult.reason, "Failed to load git log."));
                } else if (statusResult.status === "rejected") {
                    setError(toErrorMessage(statusResult.reason, "Failed to load git status."));
                } else {
                    setError(null);
                }

                // Log a combined summary only when both API calls (commits + status) fail.
                // Individual failures are already logged above in their respective catch blocks.
                if (loadFailures.length > 1) {
                    console.warn("[git-graph] partial failures:", loadFailures.join("; "));
                }

                // Track consecutive failures for toast escalation.
                if (loadFailures.length > 0) {
                    gitGraphFailureCounter.recordFailure(() => {
                        notifyAndLog(
                            "Git graph refresh",
                            "warn",
                            new Error(loadFailures.join("; ")),
                            "GitGraph",
                        );
                    });
                } else {
                    gitGraphFailureCounter.recordSuccess();
                }
            } catch (err: unknown) {
                if (!isCurrentRequest()) return;
                console.error("[git-graph] loadData unexpected error", err);
                // Unexpected errors (e.g., normalizeGitGraphCommits throws) may leave
                // state inconsistent. Clear commits to avoid showing corrupted data.
                setCommits([]);
                setError(toErrorMessage(err, "Failed to load git data."));
                gitGraphFailureCounter.recordFailure(() => {
                    notifyAndLog("Git graph refresh", "warn", err, "GitGraph");
                });
            } finally {
                if (isCurrentRequest()) {
                    setIsLoading(false);
                }
            }
        })();
    }, [allBranches, logCount]);

    // Triggers on: activeSession change, loadData identity change.
    // loadData changes when allBranches or logCount change (captured in useCallback deps),
    // so this effect also fires indirectly on branch toggle or "load more" actions.
    useEffect(() => {
        if (!activeSession) return;
        loadData();
    }, [activeSession, loadData]);

    const selectCommit = useCallback((commit: GitGraphCommit) => {
        const capturedSession = sessionRef.current?.trim();
        if (!capturedSession) return;

        const requestId = selectRequestRef.current + 1;
        selectRequestRef.current = requestId;

        const isCurrentRequest = (): boolean => {
            return mountedRef.current
                && sessionRef.current?.trim() === capturedSession
                && selectRequestRef.current === requestId;
        };

        setSelectedCommit(commit);
        setDiff(null);
        setDiffError(null);
        setIsLoadingDiff(true);

        void api.DevPanelCommitDiff(capturedSession, commit.full_hash)
            .then((result) => {
                if (!isCurrentRequest()) return;
                setDiff(result);
                setDiffError(null);
            })
            .catch((err: unknown) => {
                if (!isCurrentRequest()) return;
                console.warn("[git-graph] DevPanelCommitDiff failed", err);
                setDiff(null);
                setDiffError(toErrorMessage(err, "Failed to load commit diff."));
            })
            .finally(() => {
                if (isCurrentRequest()) {
                    setIsLoadingDiff(false);
                }
            });
    }, []);

    const loadMore = useCallback(() => {
        // Incrementing logCount triggers loadData via the useEffect dependency chain.
        setLogCount((prev) => Math.min(prev + 100, MAX_LOG_COUNT));
    }, []);

    const toggleAllBranches = useCallback((value: boolean) => {
        setAllBranches(value);
    }, []);

    const laneAssignments = useMemo(() => computeLanes(commits), [commits]);

    return {
        commits,
        laneAssignments,
        status,
        selectedCommit,
        diff,
        diffError,
        isLoadingDiff,
        allBranches,
        isLoading,
        error,
        logCount,
        selectCommit,
        loadMore,
        loadData,
        toggleAllBranches,
        activeSession,
    };
}
