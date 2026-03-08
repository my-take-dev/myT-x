import {useCallback, useEffect, useMemo, useRef} from "react";
import {api} from "../../../../api";
import {useMCPStore} from "../../../../stores/mcpStore";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {MCPStatus, MCPSnapshot} from "../../../../types/mcp";
import {logFrontendEventSafe} from "../../../../utils/logFrontendEventSafe";

interface UseMcpManagerResult {
    lspMcpList: MCPSnapshot[];
    representativeMCP: MCPSnapshot | null;
    aggregateStatus: MCPStatus | null;
    isLoading: boolean;
    error: string | null;
    activeSession: string | null;
    retryLoad: () => void;
    dismissError: () => void;
}

export function normalizeActiveSessionName(sessionName: string | null): string | null {
    const trimmed = sessionName?.trim() ?? "";
    return trimmed === "" ? null : trimmed;
}

/**
 * Returns true when a snapshot belongs to the LSP-backed MCP group.
 * Matching is case-insensitive and based on the "lsp-" ID prefix.
 */
export function isLspMcp(snapshot: MCPSnapshot): boolean {
    return snapshot.id.toLowerCase().startsWith("lsp-");
}

export function selectRepresentativeLspMcp(snapshots: readonly MCPSnapshot[]): MCPSnapshot | null {
    return snapshots.find((snapshot) => (snapshot.bridge_command?.trim() ?? "") !== "") ?? snapshots[0] ?? null;
}

/**
 * Compute aggregate status from LSP-MCP snapshots.
 * Priority: running > error > starting > stopped.
 * Precondition: snapshots must be non-empty (caller guards with length check).
 */
export function aggregateLspMcpStatus(snapshots: readonly MCPSnapshot[]): MCPStatus {
    if (snapshots.some((snapshot) => snapshot.status === "running")) {
        return "running";
    }
    if (snapshots.some((snapshot) => snapshot.status === "error")) {
        return "error";
    }
    if (snapshots.some((snapshot) => snapshot.status === "starting")) {
        return "starting";
    }
    return "stopped";
}

export function useMcpManager(): UseMcpManagerResult {
    const activeSession = normalizeActiveSessionName(useTmuxStore((s) => s.activeSession));
    const addNotification = useNotificationStore((s) => s.addNotification);

    const snapshots = useMCPStore((s) => s.snapshots);
    const sessionStates = useMCPStore((s) => s.sessionStates);

    const setSnapshots = useMCPStore((s) => s.setSnapshots);
    const beginSessionLoad = useMCPStore((s) => s.beginSessionLoad);
    const setSessionLoading = useMCPStore((s) => s.setSessionLoading);
    const setSessionError = useMCPStore((s) => s.setSessionError);

    const isMountedRef = useRef(true);
    const loadTokenRef = useRef(0);
    const retryInFlightRef = useRef(false);
    const notifyWarn = useCallback(
        (message: string) => {
            addNotification(message, "warn");
        },
        [addNotification],
    );

    useEffect(() => {
        // React StrictMode runs an extra setup+cleanup cycle in development.
        // Re-arm the mounted flag on setup so async completions are not ignored.
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
        };
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
                if (isMountedRef.current && loadTokenRef.current === token) {
                    setSessionLoading(sessionName, false);
                }
            }
        },
        [beginSessionLoad, notifyWarn, setSessionError, setSessionLoading, setSnapshots],
    );

    useEffect(() => {
        const token = ++loadTokenRef.current;
        if (activeSession == null) {
            return;
        }
        void loadSnapshots(activeSession, token);
    }, [activeSession, loadSnapshots]);

    const lspMcpList = useMemo(() => {
        if (activeSession == null) {
            return [];
        }
        // Scope: LSP-MCP only. Non-LSP MCPs are resolved through the CLI path
        // and are intentionally excluded from this aggregated UI view.
        return (snapshots[activeSession] ?? []).filter(isLspMcp);
    }, [activeSession, snapshots]);

    const representativeMCP = useMemo(() => selectRepresentativeLspMcp(lspMcpList), [lspMcpList]);
    const aggregateStatus = useMemo(
        () => (lspMcpList.length === 0 ? null : aggregateLspMcpStatus(lspMcpList)),
        [lspMcpList],
    );

    useEffect(() => {
        if (lspMcpList.length > 0 && (representativeMCP?.bridge_command?.trim() ?? "") === "") {
            console.warn("[mcp-manager] All LSP-MCP snapshots lack bridge_command");
        }
    }, [lspMcpList, representativeMCP]);

    const sessionState = activeSession ? sessionStates[activeSession] : undefined;
    const isLoading = sessionState?.loading ?? false;
    const error = sessionState?.error ?? null;

    const retryLoad = useCallback(() => {
        if (activeSession == null) {
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
        if (activeSession == null) {
            return;
        }
        setSessionError(activeSession, null);
    }, [activeSession, setSessionError]);

    return {
        lspMcpList,
        representativeMCP,
        aggregateStatus,
        isLoading,
        error,
        activeSession,
        retryLoad,
        dismissError,
    };
}
