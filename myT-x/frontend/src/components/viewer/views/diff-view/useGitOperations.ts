import type {MutableRefObject, Dispatch, SetStateAction} from "react";
import {useCallback, useEffect, useRef, useState} from "react";
import {api} from "../../../../api";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {notifyAndLog} from "../../../../utils/notifyUtils";
import type {BranchInfo, OperationType} from "./sourceControlTypes";

export interface UseGitOperationsParams {
    readonly activeSession: string | null;
    readonly sessionRef: MutableRefObject<string | null>;
    readonly setError: Dispatch<SetStateAction<string | null>>;
    readonly loadDiff: (sessionName?: string, silent?: boolean) => void;
    /** Shared ref — set to true while an operation is in flight so auto-refresh skips. */
    readonly operationActiveRef: MutableRefObject<boolean>;
    /** Lightweight status-only refresh (skips expensive WorkingDiff).
     *  Rejects on failure so callers can fall back to full loadDiff. */
    readonly refreshStatus: () => Promise<void>;
    /** Direct setter for optimistic staging updates. */
    readonly setStagedPaths: Dispatch<SetStateAction<string[]>>;
    /** Current branch info — used to detect fresh repos that need full refresh. */
    readonly branchInfo: BranchInfo | null;
}

export interface UseGitOperationsResult {
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
    readonly commitMessage: string;
    readonly setCommitMessage: (msg: string) => void;
}

export function useGitOperations({
    activeSession,
    sessionRef,
    setError,
    loadDiff,
    operationActiveRef,
    refreshStatus,
    setStagedPaths,
    branchInfo,
}: UseGitOperationsParams): UseGitOperationsResult {
    const [operationInFlight, setOperationInFlight] = useState<OperationType>(null);
    const operationRef = useRef<OperationType>(null);
    const [commitMessage, setCommitMessage] = useState("");

    // Ref avoids recreating withStagingOperation on every branchInfo change.
    const branchInfoRef = useRef(branchInfo);
    branchInfoRef.current = branchInfo;

    // Synchronously update both state and ref to eliminate the 1-render-cycle
    // delay that could allow double-submission on rapid clicks.
    // Also updates operationActiveRef so auto-refresh (focus/polling) skips.
    const setOperation = useCallback((op: OperationType) => {
        operationRef.current = op;
        operationActiveRef.current = op !== null;
        setOperationInFlight(op);
    }, [operationActiveRef]);

    // Reset when session changes.
    // ORDER DEPENDENCY: useDiffData also resets on activeSession change
    // (clearing diff results, staged paths, etc.). React does not guarantee
    // cross-hook useEffect ordering, but requestIDRef-based stale response
    // rejection in useDiffData ensures no stale data surfaces.
    useEffect(() => {
        setOperation(null);
        setCommitMessage("");
    }, [activeSession, setOperation]);

    // Wraps a git operation with: double-submission guard (via operationRef),
    // session resolution, operationActive flag for auto-refresh suppression,
    // silent diff reload on success, and error notification via Toast + log.
    const withOperation = useCallback(
        (op: NonNullable<OperationType>, fn: (session: string) => Promise<void>) => {
            return async () => {
                if (operationRef.current) return;
                const session = sessionRef.current;
                if (!session) {
                    setError("No active session");
                    return;
                }
                setOperation(op);
                try {
                    await fn(session);
                    // Silent refresh after mutation to avoid screen flash.
                    loadDiff(undefined, true);
                } catch (err: unknown) {
                    console.error(`[viewer/diff] ${op} failed`, err);
                    setError(toErrorMessage(err, `${op} failed.`));
                    notifyAndLog(op, "error", err, "GitOperations");
                } finally {
                    setOperation(null);
                }
            };
        },
        [loadDiff, setError, setOperation, sessionRef],
    );

    // Wraps a staging operation (stage/unstage) with optimistic UI update
    // and lightweight status-only refresh. For non-fresh repos, git diff HEAD
    // output is unchanged by staging — only stagedPaths needs updating.
    // Fresh repos (no HEAD commit, branchInfo.branch === "") fall back to full
    // loadDiff because the backend uses git diff --cached which changes on staging.
    const withStagingOperation = useCallback(
        (
            op: NonNullable<OperationType>,
            fn: (session: string) => Promise<void>,
            optimisticUpdate: (prev: string[]) => string[],
        ) => {
            return async () => {
                if (operationRef.current) return;
                const session = sessionRef.current;
                if (!session) {
                    setError("No active session");
                    return;
                }
                setOperation(op);
                try {
                    await fn(session);
                    // Fresh repo: diff content changes on stage/unstage → full refresh.
                    const bi = branchInfoRef.current;
                    if (!bi || bi.branch === "") {
                        loadDiff(undefined, true);
                    } else {
                        // Optimistic update: move file between staged/unstaged groups instantly.
                        setStagedPaths(optimisticUpdate);
                        // Confirm with lightweight status-only refresh.
                        // If refreshStatus fails, fall back to full loadDiff so the
                        // optimistic stagedPaths is reconciled with server truth.
                        try {
                            await refreshStatus();
                        } catch {
                            loadDiff(undefined, true);
                        }
                    }
                } catch (err: unknown) {
                    console.error(`[viewer/diff] ${op} failed`, err);
                    setError(toErrorMessage(err, `${op} failed.`));
                    notifyAndLog(op, "error", err, "GitOperations");
                } finally {
                    setOperation(null);
                }
            };
        },
        [loadDiff, refreshStatus, setStagedPaths, setError, setOperation, sessionRef],
    );

    // --- Staging operations ---
    // Optimistic UI update + status-only refresh for instant perceived response.

    const stageFile = useCallback(
        (path: string) => withStagingOperation(
            "stage",
            (session) => api.DevPanelGitStage(session, path),
            (prev) => prev.includes(path) ? prev : [...prev, path],
        )(),
        [withStagingOperation],
    );

    const unstageFile = useCallback(
        (path: string) => withStagingOperation(
            "unstage",
            (session) => api.DevPanelGitUnstage(session, path),
            (prev) => prev.filter((p) => p !== path),
        )(),
        [withStagingOperation],
    );

    const discardFile = useCallback(
        (path: string) => withOperation("discard", (session) => api.DevPanelGitDiscard(session, path))(),
        [withOperation],
    );

    // stageAll/unstageAll: use withOperation (full loadDiff) because the
    // optimistic update cannot enumerate all file paths client-side.
    // Still benefits from the git operation itself being fast; the full
    // refresh is the same cost as before but only for bulk operations.
    const stageAll = useCallback(
        () => withOperation("stageAll", (session) => api.DevPanelGitStageAll(session))(),
        [withOperation],
    );

    const unstageAll = useCallback(
        () => withOperation("unstageAll", (session) => api.DevPanelGitUnstageAll(session))(),
        [withOperation],
    );

    // --- Commit / push / pull / fetch ---

    const commit = useCallback(
        async (message: string): Promise<boolean> => {
            if (operationRef.current) return false;
            const session = sessionRef.current;
            if (!session) {
                setError("No active session");
                return false;
            }
            setOperation("commit");
            try {
                await api.DevPanelGitCommit(session, message);
                setCommitMessage("");
                loadDiff(undefined, true);
                return true;
            } catch (err: unknown) {
                console.error("[viewer/diff] commit failed", err);
                setError(toErrorMessage(err, "Commit failed."));
                notifyAndLog("Commit", "error", err, "GitOperations");
                return false;
            } finally {
                setOperation(null);
            }
        },
        [loadDiff, setError, sessionRef, setOperation],
    );

    const commitAndPush = useCallback(
        async (message: string): Promise<boolean> => {
            if (operationRef.current) return false;
            const session = sessionRef.current;
            if (!session) {
                setError("No active session");
                return false;
            }
            setOperation("commit");
            try {
                await api.DevPanelGitCommit(session, message);
                setCommitMessage("");
            } catch (err: unknown) {
                console.error("[viewer/diff] commit failed", err);
                setError(toErrorMessage(err, "Commit failed."));
                notifyAndLog("Commit", "error", err, "GitOperations");
                setOperation(null);
                return false;
            }
            setOperation("push");
            try {
                await api.DevPanelGitPush(session);
                loadDiff(undefined, true);
                return true;
            } catch (err: unknown) {
                console.error("[viewer/diff] push failed (commit succeeded)", err);
                setError(toErrorMessage(err, "Push failed (commit was successful)."));
                notifyAndLog("Push", "error", err, "GitOperations");
                loadDiff(undefined, true);
                return false;
            } finally {
                setOperation(null);
            }
        },
        [loadDiff, setError, sessionRef, setOperation],
    );

    const push = useCallback(
        () => withOperation("push", async (session) => { await api.DevPanelGitPush(session); })(),
        [withOperation],
    );

    const pull = useCallback(
        () => withOperation("pull", async (session) => { await api.DevPanelGitPull(session); })(),
        [withOperation],
    );

    const fetch_ = useCallback(
        () => withOperation("fetch", (session) => api.DevPanelGitFetch(session))(),
        [withOperation],
    );

    return {
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
        commitMessage,
        setCommitMessage,
    };
}
