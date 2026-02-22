import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {DiffTreeNode, WorkingDiffFile, WorkingDiffResult} from "./diffViewTypes";

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
            nodes.push({
                name: parts[i - 1],
                path: dirPath,
                isDir: true,
                depth: i - 1,
                isExpanded: expandedDirs.has(dirPath),
            });
        }

        const parentDir = parts.length > 1 ? parts.slice(0, -1).join("/") : "";
        if (parentDir === "" || expandedDirs.has(parentDir)) {
            nodes.push({
                name: parts[parts.length - 1],
                path: file.path,
                isDir: false,
                depth: parts.length - 1,
                isExpanded: false,
                file,
            });
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

export function useDiffView() {
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [diffResult, setDiffResult] = useState<WorkingDiffResult | null>(null);
    const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
    const [selectedPath, setSelectedPath] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const mountedRef = useRef(false);
    const sessionRef = useRef<string | null>(activeSession);
    const requestIDRef = useRef(0);
    const lastAutoLoadedSessionRef = useRef<string | null>(null);

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

    // Reset local state when session changes.
    useEffect(() => {
        setDiffResult(null);
        setExpandedDirs(new Set());
        setSelectedPath(null);
        setError(null);
        setIsLoading(false);
    }, [activeSession]);

    const loadDiff = useCallback((sessionName?: string) => {
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

        setIsLoading(true);
        setError(null);

        void api.DevPanelWorkingDiff(targetSession)
            .then((result) => {
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
                setIsLoading(false);
            })
            .catch((err: unknown) => {
                if (!mountedRef.current || requestIDRef.current !== requestID) {
                    return;
                }
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-viewer] DevPanelWorkingDiff failed", err);
                }
                setError(String(err));
                setIsLoading(false);
            });
    }, []);

    // Load once per active-session change.
    useEffect(() => {
        if (!activeSession) {
            lastAutoLoadedSessionRef.current = null;
            return;
        }
        if (lastAutoLoadedSessionRef.current === activeSession) {
            return;
        }
        lastAutoLoadedSessionRef.current = activeSession;
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
    };
}
