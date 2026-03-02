import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import type {FileContentResult, FileEntry, FlatNode} from "./fileTreeTypes";
import {flattenTree} from "./treeUtils";

export interface UseFileTreeResult {
    readonly flatNodes: readonly FlatNode[];
    readonly selectedPath: string | null;
    readonly fileContent: FileContentResult | null;
    readonly isLoadingContent: boolean;
    readonly isRootLoading: boolean;
    /** Root listing failure (loadRoot). Null when no error. */
    readonly error: string | null;
    /** File read failure (selectFile). Null when no error. */
    readonly contentError: string | null;
    /** Directory expansion failure (toggleDir). Null when no error. */
    readonly dirError: string | null;
    readonly toggleDir: (path: string) => void;
    readonly selectFile: (path: string) => void;
    readonly loadRoot: () => void;
    readonly activeSession: string | null;
}

/**
 * Custom hook that manages file-tree state for the DevPanel viewer.
 *
 * Responsibilities:
 * - Loads the root directory listing when a tmux session becomes active.
 * - Expands / collapses directories on demand, caching their children.
 * - Reads the content of a selected file for preview.
 * - Resets all state when the active session changes.
 *
 * @returns {@link UseFileTreeResult} containing tree data, selection state,
 *          loading / error flags, and action callbacks.
 */
export function useFileTree(): UseFileTreeResult {
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [rootEntries, setRootEntries] = useState<FileEntry[]>([]);
    const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
    const [loadingPaths, setLoadingPaths] = useState<Set<string>>(new Set());
    const [childrenCache, setChildrenCache] = useState<Map<string, FileEntry[]>>(new Map());
    const [selectedPath, setSelectedPath] = useState<string | null>(null);
    const [fileContent, setFileContent] = useState<FileContentResult | null>(null);
    const [isLoadingContent, setIsLoadingContent] = useState(false);
    const [isRootLoading, setIsRootLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [contentError, setContentError] = useState<string | null>(null);
    /** Directory expansion failure message. null = no error; empty string is never set. */
    const [dirError, setDirError] = useState<string | null>(null);

    // ── Refs (stale-closure prevention) ──

    // Tracks the previous session value to detect session switches.
    // When a new session becomes active, all tree state is reset to a clean slate.
    // When the same session re-renders, the equality guard prevents redundant resets.
    const prevSessionRef = useRef<string | null>(null);

    const mountedRef = useRef(true);
    const sessionRef = useRef(activeSession);

    /**
     * Ref mirror of childrenCache state to read latest value inside callbacks
     * without adding childrenCache to dependency arrays (stale closure prevention).
     */
    const childrenCacheRef = useRef(childrenCache);

    const selectRequestRef = useRef(0);
    const loadRequestRef = useRef(0);
    /** Per-directory request ID map to guard against stale toggleDir responses. */
    const toggleRequestRef = useRef<Map<string, number>>(new Map());

    /**
     * Ref mirror of expandedPaths state to allow synchronous reads inside
     * toggleDir without adding expandedPaths to its dependency array.
     * WHY: avoids commit-phase lag between rapid interactions (e.g., double-click
     * expand/collapse) where the state updater hasn't committed yet.
     * Synced in two places: inside setExpandedPathsAndSyncRef's state updater
     * and directly in the session-change reset effect.
     */
    const expandedPathsRef = useRef<ReadonlySet<string>>(new Set());

    // Synchronize sessionRef during render. This is an idempotent mutation:
    // the same value is written on every render, so multiple invocations in
    // StrictMode or Concurrent Mode are safe.
    // Avoids the one-render delay and declaration-order fragility of useEffect.
    // NOTE: Add useEffect here if useTransition/Suspense is introduced, as
    // speculative renders that don't commit can leave stale ref values.
    sessionRef.current = activeSession;

    // childrenCacheRef is synced via useEffect (not during render like sessionRef)
    // because childrenCache is an object reference that changes on every state update.
    // Render-time sync would be redundant here since callbacks only read the ref
    // after commit, and the one-frame lag is harmless for cache lookups.
    useEffect(() => {
        childrenCacheRef.current = childrenCache;
    }, [childrenCache]);

    // ── Mount / unmount lifecycle ──

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

    // ── Session change reset ──

    useEffect(() => {
        if (prevSessionRef.current === activeSession) return;
        prevSessionRef.current = activeSession;
        // Invalidate in-flight requests so stale finally handlers are discarded
        // even when the reqId guard is the only protection (session guard omitted in finally).
        loadRequestRef.current += 1;
        selectRequestRef.current += 1;
        toggleRequestRef.current.clear();
        setRootEntries([]);
        setExpandedPaths(new Set());
        expandedPathsRef.current = new Set();
        setLoadingPaths(new Set());
        setChildrenCache(new Map());
        setSelectedPath(null);
        setFileContent(null);
        setIsLoadingContent(false);
        setIsRootLoading(false);
        setError(null);
        setContentError(null);
        setDirError(null);
    }, [activeSession]);

    // NOTE: The updater returns mutable Set<string> to match useState's generic
    // parameter. expandedPathsRef is typed as ReadonlySet to prevent mutation via
    // the ref; the mutable→ReadonlySet assignment is safe (covariant widening).
    const setExpandedPathsAndSyncRef = useCallback((updater: (prev: Set<string>) => Set<string>) => {
        setExpandedPaths((prev) => {
            const next = updater(prev);
            expandedPathsRef.current = next;
            return next;
        });
    }, []);

    // ── Root loading ──
    // Guard strategy per operation (all reqIds are invalidated on session change):
    //   loadRoot:   mountedRef + session (then/catch) + reqId (then/catch/finally)
    //   toggleDir:  mountedRef + session (then/catch) + per-directory reqId (then/catch/finally)
    //   selectFile: mountedRef + session (then/catch) + reqId (then/catch/finally)

    const loadRoot = useCallback(() => {
        const capturedSession = sessionRef.current?.trim();
        if (!capturedSession) {
            setError("No active session");
            setIsRootLoading(false);
            return;
        }
        const reqId = ++loadRequestRef.current;
        setIsRootLoading(true);
        setError(null);
        // Clear dirError on refresh so stale directory errors don't persist
        // after the user explicitly reloads the tree.
        setDirError(null);
        void api.DevPanelListDir(capturedSession, "")
            .then((entries) => {
                if (!mountedRef.current) return;
                if (sessionRef.current?.trim() !== capturedSession) return;
                if (loadRequestRef.current !== reqId) return;
                setRootEntries(entries);
            })
            .catch((err: unknown) => {
                if (!mountedRef.current) return;
                if (sessionRef.current?.trim() !== capturedSession) return;
                if (loadRequestRef.current !== reqId) return;
                console.error("[file-tree] DevPanelListDir root failed", {
                    session: capturedSession,
                    err,
                });
                setRootEntries([]);
                setError(toErrorMessage(err, "Failed to load file tree."));
            })
            .finally(() => {
                // Session guard omitted: reqId is invalidated on session change
                // (see session-change reset), so the reqId check alone is sufficient
                // and avoids loading-stuck when session changes before promise settles.
                if (!mountedRef.current) return;
                if (loadRequestRef.current !== reqId) return;
                setIsRootLoading(false);
            });
    }, []);

    useEffect(() => {
        if (activeSession) {
            loadRoot();
        }
    }, [activeSession, loadRoot]);

    // ── Directory toggle ──

    const toggleDir = useCallback((path: string) => {
        const isExpanding = !expandedPathsRef.current.has(path);

        if (!isExpanding) {
            // Collapse: remove from expandedPaths, no API call needed.
            // Only clear dirError; contentError belongs to selectFile's scope
            // and should persist until the next file selection or session change.
            setDirError(null);
            setExpandedPathsAndSyncRef((prev) => {
                const next = new Set(prev);
                next.delete(path);
                return next;
            });
            return;
        }

        // Expand: clear stale per-directory errors at operation start, then load children if not cached.
        setDirError(null);

        const currentCache = childrenCacheRef.current;
        if (currentCache.has(path)) {
            // Already cached: expand immediately without API call.
            setExpandedPathsAndSyncRef((prev) => new Set(prev).add(path));
            return;
        }

        const capturedSession = sessionRef.current?.trim();
        if (!capturedSession) {
            setDirError("No active session");
            return;
        }

        // Track per-directory request ID to discard stale responses when the same
        // directory is toggled rapidly (e.g., double-click expand).
        const reqId = (toggleRequestRef.current.get(path) ?? 0) + 1;
        toggleRequestRef.current.set(path, reqId);

        // Show loading indicator while fetching directory contents.
        setLoadingPaths((lp) => new Set(lp).add(path));
        void api.DevPanelListDir(capturedSession, path)
            .then((children) => {
                if (!mountedRef.current || sessionRef.current?.trim() !== capturedSession) return;
                if (toggleRequestRef.current.get(path) !== reqId) return;
                setChildrenCache((prev) => {
                    const next = new Map(prev);
                    next.set(path, children);
                    return next;
                });
                // Expand the directory only after children are successfully loaded.
                setExpandedPathsAndSyncRef((prev) => new Set(prev).add(path));
                // Clear any stale dirError from a previous failed expand of this
                // or another directory — the successful expand supersedes it.
                setDirError(null);
            })
            .catch((err: unknown) => {
                if (!mountedRef.current || sessionRef.current?.trim() !== capturedSession) return;
                if (toggleRequestRef.current.get(path) !== reqId) return;
                console.error("[file-tree] toggleDir failed:", {path, session: capturedSession, err});
                // On failure: show inline error; directory remains collapsed.
                setDirError(toErrorMessage(err, `Failed to load directory: ${path}`));
            })
            .finally(() => {
                // Session guard omitted: toggleRequestRef is cleared on session change
                // (see session-change reset), so stale requests are discarded by the
                // reqId check. This avoids loading-stuck when session changes mid-flight.
                if (!mountedRef.current) return;
                if (toggleRequestRef.current.get(path) !== reqId) return;
                setLoadingPaths((lp) => {
                    const next = new Set(lp);
                    next.delete(path);
                    return next;
                });
            });
    }, [setExpandedPathsAndSyncRef]);

    // ── File selection ──

    const selectFile = useCallback((path: string) => {
        const capturedSession = sessionRef.current?.trim();
        if (!capturedSession) {
            setContentError("No active session");
            setIsLoadingContent(false);
            return;
        }
        const requestId = ++selectRequestRef.current;
        setSelectedPath(path);
        setIsLoadingContent(true);
        setContentError(null);
        // Clear dirError so the file content area is not blocked by a stale directory error.
        setDirError(null);
        void api.DevPanelReadFile(capturedSession, path)
            .then((content) => {
                if (!mountedRef.current) return;
                if (sessionRef.current?.trim() !== capturedSession) return;
                if (selectRequestRef.current !== requestId) return;
                setFileContent(content);
            })
            .catch((err: unknown) => {
                if (!mountedRef.current) return;
                if (sessionRef.current?.trim() !== capturedSession) return;
                if (selectRequestRef.current !== requestId) return;
                console.error("[file-tree] DevPanelReadFile failed", {
                    session: capturedSession,
                    path,
                    err,
                });
                setFileContent(null);
                setContentError(toErrorMessage(err, `Failed to read file: ${path}`));
            })
            .finally(() => {
                // Session guard omitted: requestId is invalidated on session change
                // (see session-change reset), so the requestId check alone is sufficient
                // and avoids loading-stuck when session changes before promise settles.
                if (!mountedRef.current) return;
                if (selectRequestRef.current !== requestId) return;
                setIsLoadingContent(false);
            });
    }, []);

    // ── Tree flattening ──

    const flatNodes = useMemo(
        () => flattenTree(rootEntries, expandedPaths, childrenCache, loadingPaths),
        [rootEntries, expandedPaths, childrenCache, loadingPaths],
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
