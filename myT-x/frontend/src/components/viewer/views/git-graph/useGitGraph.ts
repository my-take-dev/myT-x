import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../../../api";
import { useTmuxStore } from "../../../../stores/tmuxStore";
import type { GitGraphCommit, GitStatusResult } from "./gitGraphTypes";
import { computeLanes } from "./laneComputation";

const DEFAULT_LOG_COUNT = 100;

export function useGitGraph() {
  const activeSession = useTmuxStore((s) => s.activeSession);

  const [commits, setCommits] = useState<GitGraphCommit[]>([]);
  const [status, setStatus] = useState<GitStatusResult | null>(null);
  const [branches, setBranches] = useState<string[]>([]);
  const [selectedCommit, setSelectedCommit] = useState<GitGraphCommit | null>(null);
  const [diff, setDiff] = useState<string | null>(null);
  const [isLoadingDiff, setIsLoadingDiff] = useState(false);
  const [allBranches, setAllBranches] = useState(false);
  const [logCount, setLogCount] = useState(DEFAULT_LOG_COUNT);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const prevSessionRef = useRef<string | null>(null);

  // Guard against updates after unmount or stale session responses.
  const mountedRef = useRef(true);
  const sessionRef = useRef(activeSession);

  // Track the latest selectCommit request to discard stale responses.
  const selectRequestRef = useRef(0);

  // Keep session ref in sync.
  useEffect(() => {
    sessionRef.current = activeSession;
  }, [activeSession]);

  // Cleanup on unmount.
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  // Reset state on session change.
  useEffect(() => {
    if (prevSessionRef.current === activeSession) return;
    prevSessionRef.current = activeSession;
    setCommits([]);
    setStatus(null);
    setBranches([]);
    setSelectedCommit(null);
    setDiff(null);
    setAllBranches(false);
    setLogCount(DEFAULT_LOG_COUNT);
    setError(null);
  }, [activeSession]);

  // Load git data.
  const loadData = useCallback(() => {
    if (!activeSession) {
      setError("No active session");
      return;
    }
    const capturedSession = activeSession;
    setLoading(true);
    setError(null);

    // Load log, status, and branches in parallel using Promise.allSettled.
    void Promise.allSettled([
      api.DevPanelGitLog(capturedSession, logCount, allBranches),
      api.DevPanelGitStatus(capturedSession),
      api.DevPanelListBranches(capturedSession),
    ]).then((results) => {
      if (!mountedRef.current) return;
      if (sessionRef.current !== capturedSession) return;

      const [logResult, statusResult, branchResult] = results;

      if (logResult.status === "fulfilled") {
        setCommits(logResult.value);
      } else {
        console.warn("[DEBUG-viewer] DevPanelGitLog failed", logResult.reason);
        setError(String(logResult.reason));
      }

      if (statusResult.status === "fulfilled") {
        setStatus(statusResult.value);
      } else {
        console.warn("[DEBUG-viewer] DevPanelGitStatus failed", statusResult.reason);
      }

      if (branchResult.status === "fulfilled") {
        setBranches(branchResult.value);
      } else {
        console.warn("[DEBUG-viewer] DevPanelListBranches failed", branchResult.reason);
      }

      setLoading(false);
    });
  }, [activeSession, logCount, allBranches]);

  // Load on mount / session change.
  useEffect(() => {
    if (activeSession) {
      loadData();
    }
  }, [activeSession, loadData]);

  // Select a commit and load its diff.
  const selectCommit = useCallback((commit: GitGraphCommit) => {
    const currentSession = sessionRef.current;
    if (!currentSession) return;
    const capturedSession = currentSession;
    const requestId = ++selectRequestRef.current;
    setSelectedCommit(commit);
    setIsLoadingDiff(true);
    setDiff(null);
    void api.DevPanelCommitDiff(capturedSession, commit.full_hash)
      .then((result) => {
        if (!mountedRef.current) return;
        if (sessionRef.current !== capturedSession) return;
        if (selectRequestRef.current !== requestId) return;
        setDiff(result);
        setIsLoadingDiff(false);
      })
      .catch((err) => {
        if (!mountedRef.current) return;
        if (sessionRef.current !== capturedSession) return;
        if (selectRequestRef.current !== requestId) return;
        console.warn("[DEBUG-viewer] DevPanelCommitDiff failed", err);
        setDiff(null);
        setIsLoadingDiff(false);
      });
  }, []);

  // Load more commits.
  const loadMore = useCallback(() => {
    setLogCount((prev) => Math.min(prev + 100, 1000));
  }, []);

  // Toggle all branches mode.
  const toggleAllBranches = useCallback((value: boolean) => {
    setAllBranches(value);
    // Reload will happen via loadData dependency change.
  }, []);

  // Compute lane assignments.
  const laneAssignments = useMemo(() => computeLanes(commits), [commits]);

  return {
    commits,
    laneAssignments,
    status,
    branches,
    selectedCommit,
    diff,
    isLoadingDiff,
    allBranches,
    loading,
    error,
    logCount,
    selectCommit,
    loadMore,
    loadData,
    toggleAllBranches,
    activeSession,
  };
}
