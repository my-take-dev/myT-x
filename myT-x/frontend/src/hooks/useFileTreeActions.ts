import {useCallback, useEffect, useRef} from "react";
import {api} from "../api";
import type {FileTreeStore} from "../stores/fileTreeStore";
import {toErrorMessage} from "../utils/errorUtils";
import {matchesCapturedSessionKey} from "../utils/sessionGuard";
import {fileEntriesToNodes, findNodeByPath} from "../components/viewer/views/file-tree/treeUtils";
import {EventsOn} from "../../wailsjs/runtime";

interface RefreshDirectoryOptions {
    readonly expandOnSuccess?: boolean;
}

interface UseFileTreeActionsOptions {
    readonly activeSession: string | null;
    readonly activeSessionKey: string;
    readonly loadFileContent: boolean;
}

interface RequestState {
    root: number;
    select: number;
    readonly toggleByPath: Map<string, number>;
}

// WeakMap keeps request bookkeeping outside the Zustand snapshot while still
// allowing per-store cleanup once the store instance is garbage-collected.
const requestStateByStore = new WeakMap<FileTreeStore, RequestState>();
const latestSessionKeyByStore = new WeakMap<FileTreeStore, string>();
const treeInvalidatedEventName = "devpanel:tree-invalidated";
const watcherFailedEventName = "devpanel:watcher-failed";

interface TreeInvalidationEvent {
    readonly session_name?: unknown;
    readonly paths?: unknown;
}

interface WatcherFailedEvent {
    readonly session_name?: unknown;
    readonly message?: unknown;
}

interface SessionRenamedEvent {
    readonly oldName?: unknown;
    readonly newName?: unknown;
}

interface RenderSnapshot {
    readonly activeSession: string | null;
    readonly activeSessionKey: string;
}

function renameSessionKey(sessionKey: string, oldName: string, newName: string): string {
    if (sessionKey === "") {
        return sessionKey;
    }
    const keyPrefix = `${oldName}:`;
    if (!sessionKey.startsWith(keyPrefix)) {
        return sessionKey;
    }
    return `${newName}:${sessionKey.slice(keyPrefix.length)}`;
}

function getRequestState(store: FileTreeStore): RequestState {
    const existing = requestStateByStore.get(store);
    if (existing) {
        return existing;
    }

    const initialState: RequestState = {
        root: 0,
        select: 0,
        toggleByPath: new Map(),
    };
    requestStateByStore.set(store, initialState);
    return initialState;
}

function invalidateRequestState(store: FileTreeStore): void {
    const requestState = getRequestState(store);
    requestState.root += 1;
    requestState.select += 1;
    requestState.toggleByPath.clear();
}

function getCurrentSessionKey(store: FileTreeStore): string {
    return latestSessionKeyByStore.get(store) ?? "";
}

// Frontend only normalizes backend-supplied relative panel paths for lookup
// consistency. Absolute paths and traversal should already be rejected in Go.
function normalizePanelPath(path: string): string {
    const normalizedInput = path.trim().replaceAll("\\", "/");
    if (normalizedInput === "") {
        return "";
    }

    const segments: string[] = [];
    for (const segment of normalizedInput.split("/")) {
        if (segment === "" || segment === ".") {
            continue;
        }
        if (segment === "..") {
            if (segments.length > 0 && segments[segments.length - 1] !== "..") {
                segments.pop();
                continue;
            }
            segments.push(segment);
            continue;
        }
        segments.push(segment);
    }

    return segments.join("/");
}

function normalizeRefreshPaths(paths: readonly string[]): string[] {
    const unique = new Set(paths.map((path) => normalizePanelPath(path.trim())));
    if (unique.has("")) {
        return [""];
    }
    return [...unique].sort((left, right) => left.localeCompare(right));
}

function parseTreeInvalidationEvent(payload: unknown): { sessionName: string; paths: string[] } | null {
    if (!payload || typeof payload !== "object") {
        return null;
    }

    const event = payload as TreeInvalidationEvent;
    if (typeof event.session_name !== "string") {
        return null;
    }
    const sessionName = event.session_name.trim();
    if (sessionName === "") {
        return null;
    }
    if (!Array.isArray(event.paths)) {
        return {
            sessionName,
            paths: [""],
        };
    }

    const paths = event.paths.filter((path): path is string => typeof path === "string");
    return {
        sessionName,
        paths: normalizeRefreshPaths(paths),
    };
}

function parseWatcherFailedEvent(payload: unknown): { sessionName: string; message: string } | null {
    if (!payload || typeof payload !== "object") {
        return null;
    }

    const event = payload as WatcherFailedEvent;
    if (typeof event.session_name !== "string" || typeof event.message !== "string") {
        return null;
    }
    const sessionName = event.session_name.trim();
    const message = event.message.trim();
    if (sessionName === "" || message === "") {
        return null;
    }
    return {sessionName, message};
}

function parseSessionRenamedEvent(payload: unknown): { oldName: string; newName: string } | null {
    if (!payload || typeof payload !== "object") {
        return null;
    }

    const event = payload as SessionRenamedEvent;
    if (typeof event.oldName !== "string" || typeof event.newName !== "string") {
        return null;
    }
    const oldName = event.oldName.trim();
    const newName = event.newName.trim();
    if (oldName === "" || newName === "" || oldName === newName) {
        return null;
    }
    return {oldName, newName};
}

function reconcileSelection(store: FileTreeStore): void {
    const state = store.getState();
    const selectedPath = state.selectedPath;
    if (!selectedPath || findNodeByPath(state.tree, selectedPath)) {
        return;
    }

    state.setSelectedPath(null);
    state.setIsLoadingContent(false);
    state.setContentError(null);
    if (state.fileContent?.path === selectedPath) {
        state.setFileContent(null);
    }
}

export function useFileTreeActions(
    store: FileTreeStore,
    {activeSession, activeSessionKey, loadFileContent}: UseFileTreeActionsOptions,
) {
    const latestRenderSnapshotRef = useRef<RenderSnapshot>({
        activeSession: activeSession?.trim() ?? null,
        activeSessionKey,
    });
    latestRenderSnapshotRef.current = {
        activeSession: activeSession?.trim() ?? null,
        activeSessionKey,
    };

    const refreshDirectory = useCallback(async (dirPath: string, options?: RefreshDirectoryOptions) => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        if (!capturedSession) {
            store.getState().setError("No active session");
            throw new Error("No active session.");
        }

        if (dirPath === "") {
            const requestState = getRequestState(store);
            requestState.root += 1;
            const requestID = requestState.root;

            store.getState().setIsRootLoading(true);
            store.getState().setError(null);
            store.getState().setDirError(null);

            try {
                const entries = await api.DevPanelListDir(capturedSession, "");
                if (getCurrentSessionKey(store) !== capturedSessionKey || getRequestState(store).root !== requestID) {
                    return;
                }

                store.getState().setRootNodes(fileEntriesToNodes(entries));
                reconcileSelection(store);
            } catch (err: unknown) {
                if (getCurrentSessionKey(store) !== capturedSessionKey || getRequestState(store).root !== requestID) {
                    return;
                }

                store.getState().setError(toErrorMessage(err, "Failed to load file tree."));
                throw err;
            } finally {
                if (getCurrentSessionKey(store) === capturedSessionKey && getRequestState(store).root === requestID) {
                    store.getState().setIsRootLoading(false);
                }
            }
            return;
        }

        const requestState = getRequestState(store);
        const requestID = (requestState.toggleByPath.get(dirPath) ?? 0) + 1;
        requestState.toggleByPath.set(dirPath, requestID);
        store.getState().setLoadingPath(dirPath, true);

        try {
            const entries = await api.DevPanelListDir(capturedSession, dirPath);
            if (getCurrentSessionKey(store) !== capturedSessionKey || getRequestState(store).toggleByPath.get(dirPath) !== requestID) {
                return;
            }

            store.getState().setChildrenForPath(dirPath, fileEntriesToNodes(entries));
            reconcileSelection(store);
            if (options?.expandOnSuccess) {
                store.getState().setExpanded(dirPath, true);
            }
        } catch (err: unknown) {
            if (getCurrentSessionKey(store) !== capturedSessionKey || getRequestState(store).toggleByPath.get(dirPath) !== requestID) {
                return;
            }
            throw err;
        } finally {
            if (getCurrentSessionKey(store) === capturedSessionKey && getRequestState(store).toggleByPath.get(dirPath) === requestID) {
                store.getState().setLoadingPath(dirPath, false);
            }
        }
    }, [activeSession, activeSessionKey, store]);

    const loadRoot = useCallback(() => {
        void refreshDirectory("").catch((err: unknown) => {
            console.error("[FILE-TREE] loadRoot failed", err);
        });
    }, [refreshDirectory]);

    const toggleDir = useCallback((path: string) => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        const state = store.getState();
        const node = findNodeByPath(state.tree, path);
        if (!node?.isDir) {
            return;
        }

        if (!node.hasChildren && node.children === undefined) {
            return;
        }

        const isExpanding = !state.expandedPaths.has(path);
        if (!isExpanding) {
            state.setDirError(null);
            state.setExpanded(path, false);
            return;
        }

        state.setDirError(null);
        if (node.children !== undefined) {
            state.setExpanded(path, true);
            return;
        }

        void refreshDirectory(path, {expandOnSuccess: true}).catch((err: unknown) => {
            if (!matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store))) {
                return;
            }

            console.error("[file-tree] toggleDir failed", {
                path,
                session: capturedSession,
                err,
            });
            store.getState().setDirError(toErrorMessage(err, `Failed to load directory: ${path}`));
        });
    }, [activeSession, activeSessionKey, refreshDirectory, store]);

    const selectFile = useCallback((path: string) => {
        store.getState().setSelectedPath(path);
        store.getState().setDirError(null);

        if (!loadFileContent) {
            store.getState().setContentError(null);
            return;
        }

        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        if (!capturedSession) {
            store.getState().setContentError("No active session");
            store.getState().setIsLoadingContent(false);
            return;
        }

        const requestState = getRequestState(store);
        requestState.select += 1;
        const requestID = requestState.select;

        store.getState().setIsLoadingContent(true);
        store.getState().setContentError(null);

        void api.DevPanelReadFile(capturedSession, path)
            .then((content) => {
                if (!matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store)) || getRequestState(store).select !== requestID) {
                    return;
                }
                store.getState().setFileContent(content);
            })
            .catch((err: unknown) => {
                if (!matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store)) || getRequestState(store).select !== requestID) {
                    return;
                }

                console.error("[file-tree] DevPanelReadFile failed", {
                    session: capturedSession,
                    path,
                    err,
                });
                store.getState().setFileContent(null);
                store.getState().setContentError(toErrorMessage(err, `Failed to read file: ${path}`));
            })
            .finally(() => {
                if (matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store)) && getRequestState(store).select === requestID) {
                    store.getState().setIsLoadingContent(false);
                }
            });
    }, [activeSession, activeSessionKey, loadFileContent, store]);

    useEffect(() => {
        latestSessionKeyByStore.set(store, activeSessionKey);
    }, [activeSessionKey, store]);

    useEffect(() => {
        return () => {
            latestSessionKeyByStore.delete(store);
        };
    }, [store]);

    useEffect(() => {
        invalidateRequestState(store);
        store.getState().reset();

        if (activeSession) {
            void refreshDirectory("").catch((err: unknown) => {
                console.error("[FILE-TREE] session change root refresh failed", err);
            });
        }
    }, [activeSessionKey, refreshDirectory, store]);

    useEffect(() => {
        const capturedSession = activeSession?.trim();
        const capturedSessionKey = activeSessionKey;
        const renamedSessionRef = {current: null as string | null};
        if (!capturedSession) {
            return;
        }
        if (!capturedSessionKey) {
            return;
        }

        let disposed = false;
        void api.DevPanelStartWatcher(capturedSessionKey)
            .then(() => {
                if (disposed) {
                    return;
                }
                if (matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store))) {
                    store.getState().setWatcherError(null);
                }
            })
            .catch((err: unknown) => {
                console.warn("[file-tree] DevPanelStartWatcher failed", {
                    session: capturedSession,
                    err,
                });
                if (disposed) {
                    return;
                }
                if (matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store))) {
                    store.getState().setWatcherError("Automatic refresh is unavailable. Reload the directory manually if needed.");
                }
            });

        const cancel = EventsOn(treeInvalidatedEventName, (payload: unknown) => {
            if (disposed) {
                return;
            }

            const event = parseTreeInvalidationEvent(payload);
            if (!event || event.sessionName !== capturedSession) {
                return;
            }

            for (const dirPath of event.paths) {
                const state = store.getState();
                if (dirPath !== "") {
                    const node = findNodeByPath(state.tree, dirPath);
                    if (!node?.isDir) {
                        continue;
                    }
                }
                const shouldExpand = dirPath !== "" && state.expandedPaths.has(dirPath);

                void refreshDirectory(dirPath, {expandOnSuccess: shouldExpand}).catch((err: unknown) => {
                    console.warn("[file-tree] tree invalidation refresh failed", {
                        dirPath,
                        session: capturedSession,
                        err,
                    });
                    if (disposed) {
                        return;
                    }
                    if (matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store))) {
                        store.getState().setWatcherError(
                            toErrorMessage(err, "Automatic refresh failed. Reload the directory manually if needed."),
                        );
                    }
                });
            }
        });
        const cancelWatcherFailed = EventsOn(watcherFailedEventName, (payload: unknown) => {
            if (disposed) {
                return;
            }

            const event = parseWatcherFailedEvent(payload);
            if (!event || event.sessionName !== capturedSession) {
                return;
            }
            if (matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store))) {
                store.getState().setWatcherError(event.message);
            }
        });
        const cancelSessionRenamed = EventsOn("tmux:session-renamed", (payload: unknown) => {
            if (disposed) {
                return;
            }

            const event = parseSessionRenamedEvent(payload);
            if (!event || event.oldName !== capturedSession) {
                return;
            }
            renamedSessionRef.current = event.newName;
        });

        return () => {
            const shouldSurfaceCleanupFailure = matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store));
            disposed = true;
            cancel();
            cancelWatcherFailed();
            cancelSessionRenamed();

            const stopTargets = [capturedSessionKey];
            const latestRenderSnapshot = latestRenderSnapshotRef.current;
            const shouldKeepRenamedWatcher =
                renamedSessionRef.current !== null
                && renamedSessionRef.current === latestRenderSnapshot.activeSession
                && latestRenderSnapshot.activeSessionKey !== capturedSessionKey;
            if (
                renamedSessionRef.current
                && renamedSessionRef.current !== capturedSession
                && !shouldKeepRenamedWatcher
            ) {
                stopTargets.push(renameSessionKey(capturedSessionKey, capturedSession, renamedSessionRef.current));
            }

            void Promise.allSettled(stopTargets.map((sessionKey) => api.DevPanelStopWatcher(sessionKey))).then((results) => {
                const cleanupErrors: string[] = [];
                for (let index = 0; index < results.length; index += 1) {
                    const result = results[index];
                    if (result.status === "fulfilled") {
                        continue;
                    }

                    const sessionKey = stopTargets[index];
                    const err = result.reason;
                    console.warn("[file-tree] DevPanelStopWatcher failed", {
                        sessionKey,
                        err,
                    });
                    cleanupErrors.push(
                        toErrorMessage(err, `Automatic refresh cleanup failed for ${sessionKey}. Reload the directory manually if needed.`),
                    );
                }
                if (
                    cleanupErrors.length > 0
                    && shouldSurfaceCleanupFailure
                    && matchesCapturedSessionKey(capturedSessionKey, getCurrentSessionKey(store))
                ) {
                    store.getState().setWatcherError(cleanupErrors.join("\n"));
                }
            });
        };
    }, [activeSession, activeSessionKey, refreshDirectory, store]);

    return {
        loadRoot,
        refreshDirectory,
        selectFile,
        toggleDir,
    };
}
