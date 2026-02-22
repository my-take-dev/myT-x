import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../../../api";
import { useTmuxStore } from "../../../../stores/tmuxStore";
import type { FileContentResult, FileEntry } from "./fileTreeTypes";
import { flattenTree } from "./treeUtils";

export function useFileTree() {
  const activeSession = useTmuxStore((s) => s.activeSession);

  const [rootEntries, setRootEntries] = useState<FileEntry[]>([]);
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
  const [loadingPaths, setLoadingPaths] = useState<Set<string>>(new Set());
  const [childrenCache, setChildrenCache] = useState<Map<string, FileEntry[]>>(new Map());
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [fileContent, setFileContent] = useState<FileContentResult | null>(null);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [rootLoading, setRootLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Track session changes to reset state.
  const prevSessionRef = useRef<string | null>(null);

  // Guard against updates after unmount or stale session responses.
  const mountedRef = useRef(true);
  const sessionRef = useRef(activeSession);

  // Ref for childrenCache to avoid toggleDir dependency on cache state.
  const childrenCacheRef = useRef(childrenCache);

  // Track the latest selectFile request to discard stale responses.
  const selectRequestRef = useRef(0);

  // Keep refs in sync.
  useEffect(() => {
    sessionRef.current = activeSession;
  }, [activeSession]);

  useEffect(() => {
    childrenCacheRef.current = childrenCache;
  }, [childrenCache]);

  // Cleanup on unmount.
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  // Reset state when session changes.
  useEffect(() => {
    if (prevSessionRef.current === activeSession) return;
    prevSessionRef.current = activeSession;
    setRootEntries([]);
    setExpandedPaths(new Set());
    setLoadingPaths(new Set());
    setChildrenCache(new Map());
    setSelectedPath(null);
    setFileContent(null);
    setError(null);
  }, [activeSession]);

  // Load root directory.
  const loadRoot = useCallback(() => {
    if (!activeSession) {
      setError("No active session");
      return;
    }
    const capturedSession = activeSession;
    setRootLoading(true);
    setError(null);
    void api.DevPanelListDir(activeSession, "").then((entries) => {
      if (!mountedRef.current) return;
      if (sessionRef.current !== capturedSession) return;
      setRootEntries(entries);
      setRootLoading(false);
    }).catch((err) => {
      if (!mountedRef.current) return;
      if (sessionRef.current !== capturedSession) return;
      console.warn("[DEBUG-viewer] DevPanelListDir root failed", err);
      setError(String(err));
      setRootLoading(false);
    });
  }, [activeSession]);

  // Load root on mount / session change.
  useEffect(() => {
    if (activeSession) {
      loadRoot();
    }
  }, [activeSession, loadRoot]);

  // Toggle directory expansion.
  const toggleDir = useCallback((path: string) => {
    setExpandedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
        // Load children if not cached. Read from ref to avoid dependency.
        const currentCache = childrenCacheRef.current;
        const currentSession = sessionRef.current;
        if (!currentCache.has(path) && currentSession) {
          const capturedSession = currentSession;
          setLoadingPaths((lp) => new Set(lp).add(path));
          void api.DevPanelListDir(capturedSession, path).then((children) => {
            if (!mountedRef.current) return;
            if (sessionRef.current !== capturedSession) return;
            setChildrenCache((cache) => new Map(cache).set(path, children));
            setLoadingPaths((lp) => {
              const next = new Set(lp);
              next.delete(path);
              return next;
            });
          }).catch((err) => {
            if (!mountedRef.current) return;
            console.warn("[DEBUG-viewer] DevPanelListDir failed", path, err);
            setLoadingPaths((lp) => {
              const next = new Set(lp);
              next.delete(path);
              return next;
            });
          });
        }
      }
      return next;
    });
  }, []);

  // Select a file and load its content.
  const selectFile = useCallback((path: string) => {
    const currentSession = sessionRef.current;
    if (!currentSession) return;
    const capturedSession = currentSession;
    const requestId = ++selectRequestRef.current;
    setSelectedPath(path);
    setIsLoadingContent(true);
    void api.DevPanelReadFile(capturedSession, path).then((content) => {
      if (!mountedRef.current) return;
      if (sessionRef.current !== capturedSession) return;
      if (selectRequestRef.current !== requestId) return;
      setFileContent(content);
      setIsLoadingContent(false);
    }).catch((err) => {
      if (!mountedRef.current) return;
      if (sessionRef.current !== capturedSession) return;
      if (selectRequestRef.current !== requestId) return;
      console.warn("[DEBUG-viewer] DevPanelReadFile failed", path, err);
      setFileContent(null);
      setIsLoadingContent(false);
    });
  }, []);

  // Flatten tree for virtualized rendering.
  const flatNodes = useMemo(
    () => flattenTree(rootEntries, expandedPaths, childrenCache, loadingPaths),
    [rootEntries, expandedPaths, childrenCache, loadingPaths],
  );

  return {
    flatNodes,
    selectedPath,
    fileContent,
    isLoadingContent,
    rootLoading,
    error,
    toggleDir,
    selectFile,
    loadRoot,
    activeSession,
  };
}
