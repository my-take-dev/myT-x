import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../../../../api";
import {useMCPStore} from "../../../../stores/mcpStore";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {MCPSnapshot} from "../../../../types/mcp";
import {logFrontendEventSafe} from "../../../../utils/logFrontendEventSafe";

interface UseMcpManagerResult {
    mcpList: MCPSnapshot[];
    selectedMCP: MCPSnapshot | null;
    isLoading: boolean;
    error: string | null;
    toggleMCP: (mcpId: string, enabled: boolean) => void;
    togglingIds: ReadonlySet<string>;
    selectMCP: (mcpId: string | null) => void;
    activeSession: string | null;
    retryLoad: () => void;
    dismissError: () => void;
}

export function useMcpManager(): UseMcpManagerResult {
    const activeSession = useTmuxStore((s) => s.activeSession);
    const addNotification = useNotificationStore((s) => s.addNotification);

    const snapshots = useMCPStore((s) => s.snapshots);
    const sessionStates = useMCPStore((s) => s.sessionStates);
    const selectedMCPId = useMCPStore((s) => s.selectedMCPId);

    const setSnapshots = useMCPStore((s) => s.setSnapshots);
    const beginSessionLoad = useMCPStore((s) => s.beginSessionLoad);
    const updateMCPState = useMCPStore((s) => s.updateMCPState);
    const setSessionLoading = useMCPStore((s) => s.setSessionLoading);
    const setSessionError = useMCPStore((s) => s.setSessionError);
    const selectMCP = useMCPStore((s) => s.selectMCP);

    const isMountedRef = useRef(true);
    const loadTokenRef = useRef(0);
    const retryInFlightRef = useRef(false);
    const togglingIdsRef = useRef<Set<string>>(new Set());
    const [togglingIds, setTogglingIds] = useState<Set<string>>(new Set());
    const notifyWarn = useCallback(
        (message: string) => {
            addNotification(message, "warn");
        },
        [addNotification],
    );

    useEffect(() => {
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    const beginToggle = useCallback((mcpId: string): boolean => {
        if (togglingIdsRef.current.has(mcpId)) {
            return false;
        }
        const next = new Set(togglingIdsRef.current);
        next.add(mcpId);
        togglingIdsRef.current = next;
        setTogglingIds(next);
        return true;
    }, []);

    const endToggle = useCallback((mcpId: string) => {
        if (!togglingIdsRef.current.has(mcpId)) {
            return;
        }
        const next = new Set(togglingIdsRef.current);
        next.delete(mcpId);
        togglingIdsRef.current = next;
        setTogglingIds(next);
    }, []);

    const loadSnapshots = useCallback(
        async (sessionName: string, token: number) => {
            beginSessionLoad(sessionName);

            try {
                const result = await api.ListMCPServers(sessionName);
                if (!isMountedRef.current || loadTokenRef.current !== token) {
                    return;
                }
                setSnapshots(sessionName, result ?? []);
            } catch (err: unknown) {
                if (!isMountedRef.current || loadTokenRef.current !== token) {
                    return;
                }
                const message = err instanceof Error ? err.message : String(err);
                setSessionError(sessionName, message);
                notifyWarn(`Failed to load MCP servers (${sessionName}): ${message}`);
                logFrontendEventSafe("warn", `ListMCPServers failed (${sessionName}): ${message}`, "frontend/mcp");
                if (import.meta.env.DEV) {
                    console.warn("[mcp-manager] ListMCPServers failed:", err);
                }
            } finally {
                if (!isMountedRef.current) {
                    return;
                }
                if (loadTokenRef.current === token) {
                    setSessionLoading(sessionName, false);
                }
            }
        },
        [beginSessionLoad, notifyWarn, setSessionError, setSessionLoading, setSnapshots],
    );

    useEffect(() => {
        const token = ++loadTokenRef.current;
        togglingIdsRef.current = new Set();
        setTogglingIds(new Set());
        if (!activeSession) {
            selectMCP(null);
            return;
        }

        void loadSnapshots(activeSession, token);
    }, [activeSession, loadSnapshots, selectMCP]);

    const mcpList = useMemo(() => {
        if (!activeSession) {
            return [];
        }
        return snapshots[activeSession] ?? [];
    }, [activeSession, snapshots]);

    useEffect(() => {
        if (selectedMCPId == null) {
            return;
        }
        if (mcpList.some((m) => m.id === selectedMCPId)) {
            return;
        }
        selectMCP(null);
    }, [mcpList, selectMCP, selectedMCPId]);

    const selectedMCP = mcpList.find((m) => m.id === selectedMCPId) ?? null;

    const sessionState = activeSession ? sessionStates[activeSession] : undefined;
    const isLoading = sessionState?.loading ?? false;
    const error = sessionState?.error ?? null;

    const toggleMCP = useCallback(
        (mcpId: string, enabled: boolean) => {
            if (!activeSession) {
                return;
            }
            if (!beginToggle(mcpId)) {
                return;
            }
            const sessionName = activeSession;
            const prevEnabled =
                useMCPStore
                    .getState()
                    .snapshots[sessionName]
                    ?.find((item) => item.id === mcpId)
                    ?.enabled ?? !enabled;

            updateMCPState(sessionName, mcpId, {enabled});

            void api.ToggleMCPServer(sessionName, mcpId, enabled)
                .then(() => {
                    setSessionError(sessionName, null);
                })
                .catch((err: unknown) => {
                    updateMCPState(sessionName, mcpId, {enabled: prevEnabled});
                    if (!isMountedRef.current) {
                        return;
                    }
                    const message = err instanceof Error ? err.message : String(err);
                    setSessionError(sessionName, message);
                    notifyWarn(`Failed to update MCP state (${sessionName}/${mcpId}): ${message}`);
                    logFrontendEventSafe(
                        "warn",
                        `ToggleMCPServer failed (${sessionName}/${mcpId}): ${message}`,
                        "frontend/mcp",
                    );
                    if (import.meta.env.DEV) {
                        console.warn("[mcp-manager] ToggleMCPServer failed:", err);
                    }
                })
                .finally(() => {
                    if (!isMountedRef.current) {
                        return;
                    }
                    endToggle(mcpId);
                });
        },
        [activeSession, beginToggle, endToggle, notifyWarn, setSessionError, updateMCPState],
    );

    const retryLoad = useCallback(() => {
        if (!activeSession) {
            return;
        }
        if (retryInFlightRef.current) {
            return;
        }
        retryInFlightRef.current = true;
        const token = ++loadTokenRef.current;
        void loadSnapshots(activeSession, token).finally(() => {
            retryInFlightRef.current = false;
        });
    }, [activeSession, loadSnapshots]);

    const dismissError = useCallback(() => {
        if (!activeSession) {
            return;
        }
        setSessionError(activeSession, null);
    }, [activeSession, setSessionError]);

    return {
        mcpList,
        selectedMCP,
        isLoading,
        error,
        toggleMCP,
        togglingIds,
        selectMCP,
        activeSession,
        retryLoad,
        dismissError,
    };
}
