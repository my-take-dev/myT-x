import {useCallback, useEffect, useMemo, useRef} from "react";
import {api} from "../../../../api";
import {useMCPStore} from "../../../../stores/mcpStore";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {MCPSnapshot} from "../../../../types/mcp";
import {logFrontendEventSafe} from "../../../../utils/logFrontendEventSafe";
import {createConsecutiveFailureCounter} from "../../../../utils/notifyUtils";
import {shouldIgnoreSessionMutation} from "../../../../utils/sessionGuard";

const MCP_KIND_CUSTOM = "custom";
const MCP_KIND_ORCHESTRATOR = "orchestrator";
const MCP_KIND_SINGLE_TASK_RUNNER = "single-task-runner";

// Module-level consecutive failure counter for MCP manager load operations.
// Gates toast + error log notifications; inline error display via Zustand
// store remains immediate regardless of counter state.
const mcpManagerFailureCounter = createConsecutiveFailureCounter(3);

interface UseMcpManagerResult {
    lspMcpList: MCPSnapshot[];
    customMcpList: MCPSnapshot[];
    orchMcpList: MCPSnapshot[];
    strMcpList: MCPSnapshot[];
    representativeMCP: MCPSnapshot | null;
    customRepresentativeMCP: MCPSnapshot | null;
    orchRepresentativeMCP: MCPSnapshot | null;
    strRepresentativeMCP: MCPSnapshot | null;
    isLoading: boolean;
    error: string | null;
    activeSession: string | null;
    activeSessionKey: string;
    retryLoad: () => void;
    dismissError: () => void;
}

export function normalizeActiveSessionName(sessionName: string | null): string | null {
    const trimmed = sessionName?.trim() ?? "";
    return trimmed === "" ? null : trimmed;
}

function normalizedKind(snapshot: MCPSnapshot): string {
    return snapshot.kind?.trim() ?? "";
}

function hasLegacyOrchestratorIdentity(snapshot: MCPSnapshot): boolean {
    return snapshot.id.toLowerCase().startsWith("orch-");
}

function hasReservedEmbeddedIdentity(snapshot: MCPSnapshot): boolean {
    return snapshot.id.toLowerCase() === MCP_KIND_SINGLE_TASK_RUNNER;
}

/**
 * Returns true when a snapshot belongs to the LSP-backed MCP group.
 * LSP snapshots use the built-in "lsp-" ID prefix and must not swallow
 * non-LSP/custom MCPs as a catch-all.
 */
export function isLspMcp(snapshot: MCPSnapshot): boolean {
    const normalizedID = snapshot.id.toLowerCase();
    return normalizedKind(snapshot) === "" && normalizedID.startsWith("lsp-");
}

/**
 * Returns true when a snapshot belongs to the orchestrator MCP group.
 * Matching uses the explicit kind first and falls back to the legacy "orch-"
 * namespace because built-in orchestrator MCPs share that ID prefix.
 */
export function isOrchMcp(snapshot: MCPSnapshot): boolean {
    const kind = normalizedKind(snapshot);
    if (kind !== "") {
        return kind === MCP_KIND_ORCHESTRATOR;
    }
    return hasLegacyOrchestratorIdentity(snapshot);
}

/**
 * Returns true when a snapshot belongs to the single-task-runner MCP group.
 * Matching prefers the kind field and falls back to the single built-in MCP ID.
 * Exact fallback is intentional so custom IDs such as
 * "single-task-runner-helper" are not swallowed as embedded runtimes.
 */
export function isStrMcp(snapshot: MCPSnapshot): boolean {
    const kind = normalizedKind(snapshot);
    if (kind !== "") {
        return kind === MCP_KIND_SINGLE_TASK_RUNNER;
    }
    return hasReservedEmbeddedIdentity(snapshot);
}

export function isCustomMcp(snapshot: MCPSnapshot): boolean {
    const kind = normalizedKind(snapshot);
    if (kind !== "") {
        if (hasReservedEmbeddedIdentity(snapshot)) {
            return false;
        }
        return kind === MCP_KIND_CUSTOM
            || (kind !== MCP_KIND_ORCHESTRATOR && kind !== MCP_KIND_SINGLE_TASK_RUNNER);
    }
    return !isOrchMcp(snapshot) && !isStrMcp(snapshot) && !isLspMcp(snapshot);
}

export function selectRepresentativeMcp(snapshots: readonly MCPSnapshot[]): MCPSnapshot | null {
    return snapshots.find((snapshot) => (snapshot.bridge_command?.trim() ?? "") !== "") ?? snapshots[0] ?? null;
}

export function useMcpManager(): UseMcpManagerResult {
    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = normalizeActiveSessionName(useTmuxStore((s) => s.activeSession));
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";
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
    const latestSessionKeyRef = useRef(activeSessionKey);

    latestSessionKeyRef.current = activeSessionKey;

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
        async (sessionName: string, capturedSessionKey: string) => {
            const loadToken = ++loadTokenRef.current;
            beginSessionLoad(sessionName);

            try {
                const result = await api.ListMCPServers(sessionName);
                if (
                    shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)
                    || loadTokenRef.current !== loadToken
                ) {
                    return;
                }
                setSnapshots(sessionName, result ?? []);
                mcpManagerFailureCounter.recordSuccess();
            } catch (err: unknown) {
                if (
                    shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)
                    || loadTokenRef.current !== loadToken
                ) {
                    return;
                }
                const message = err instanceof Error ? err.message : String(err);
                setSessionError(sessionName, message);
                mcpManagerFailureCounter.recordFailure(() => {
                    notifyWarn(`Failed to load MCP servers (${sessionName}): ${message}`);
                    logFrontendEventSafe("warn", `ListMCPServers failed (${sessionName}): ${message}`, "frontend/mcp");
                });
                if (import.meta.env.DEV) {
                    console.warn("[mcp-manager] ListMCPServers failed:", err);
                }
            } finally {
                if (isMountedRef.current && loadTokenRef.current === loadToken) {
                    setSessionLoading(sessionName, false);
                }
            }
        },
        [beginSessionLoad, notifyWarn, setSessionError, setSessionLoading, setSnapshots],
    );

    useEffect(() => {
        if (activeSession == null) {
            return;
        }
        void loadSnapshots(activeSession, activeSessionKey);
    }, [activeSessionKey, activeSession, loadSnapshots]);

    const lspMcpList = useMemo(() => {
        if (activeSession == null) {
            return [];
        }
        return (snapshots[activeSession] ?? []).filter(isLspMcp);
    }, [activeSession, snapshots]);

    const orchMcpList = useMemo(() => {
        if (activeSession == null) {
            return [];
        }
        return (snapshots[activeSession] ?? []).filter(isOrchMcp);
    }, [activeSession, snapshots]);

    const strMcpList = useMemo(() => {
        if (activeSession == null) {
            return [];
        }
        return (snapshots[activeSession] ?? []).filter(isStrMcp);
    }, [activeSession, snapshots]);

    const customMcpList = useMemo(() => {
        if (activeSession == null) {
            return [];
        }
        return (snapshots[activeSession] ?? []).filter(isCustomMcp);
    }, [activeSession, snapshots]);

    const representativeMCP = useMemo(() => selectRepresentativeMcp(lspMcpList), [lspMcpList]);
    const customRepresentativeMCP = useMemo(() => selectRepresentativeMcp(customMcpList), [customMcpList]);
    const orchRepresentativeMCP = useMemo(
        () => selectRepresentativeMcp(orchMcpList),
        [orchMcpList],
    );
    const strRepresentativeMCP = useMemo(
        () => selectRepresentativeMcp(strMcpList),
        [strMcpList],
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
        const capturedKey = latestSessionKeyRef.current;
        void loadSnapshots(activeSession, capturedKey).finally(() => {
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
        customMcpList,
        orchMcpList,
        strMcpList,
        representativeMCP,
        customRepresentativeMCP,
        orchRepresentativeMCP,
        strRepresentativeMCP,
        isLoading,
        error,
        activeSession,
        activeSessionKey,
        retryLoad,
        dismissError,
    };
}
