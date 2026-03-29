import {useEffect, useRef} from "react";
import {api} from "../../api";
import {useMCPStore} from "../../stores/mcpStore";
import {logFrontendEventSafe} from "../../utils/logFrontendEventSafe";
import {createConsecutiveFailureCounter} from "../../utils/notifyUtils";
import {asObject} from "../../utils/typeGuards";
import {cleanupEventListeners, createEventSubscriber, notifyWarn} from "./eventHelpers";

// SUG-20: Module-level constant — shared across renders, avoids re-creation inside useEffect.
const DEBOUNCE_MS = 80;

// Module-level consecutive failure counter — shared across renders.
// Single counter for all MCP sync operations; per-key errors are tracked
// in the Zustand store for inline display. The counter gates toast + error
// log notifications to prevent spam during systemic failures (e.g. backend down).
const mcpSyncFailureCounter = createConsecutiveFailureCounter(3);

// Payload types are compile-time documentation only.
interface MCPEventMap {
    // MCP state change — ping event triggering session-wide or per-item refresh.
    "mcp:state-changed": {session_name?: string; mcp_id?: string};
    // MCP manager lifecycle event emitted by backend on shutdown/close.
    "mcp:manager-closed": null;
}

/**
 * Subscribes to MCP state change events and refreshes MCP server snapshots.
 *
 * Uses per-key debouncing: each (sessionName, mcpID) pair has its own timer
 * to prevent flooding when multiple MCP servers change state simultaneously.
 *
 * Fetch sequence maps prevent stale responses from overwriting newer data.
 */
export function useMCPSync(): void {
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;
        const cleanupFns: Array<() => void> = [];
        const onEvent = createEventSubscriber<MCPEventMap>(cleanupFns);

        const debounceTimers = new Map<string, ReturnType<typeof setTimeout>>();
        const fetchSeqMap = new Map<string, number>();

        const scheduleMCPRefresh = (sessionName: string, mcpID: string | null) => {
            if (!isMountedRef.current) {
                return;
            }
            const normalizedMcpID = mcpID?.trim() ?? "";
            const key = normalizedMcpID === "" ? sessionName : `${sessionName}:${normalizedMcpID}`;
            const prevTimer = debounceTimers.get(key);
            if (prevTimer != null) {
                clearTimeout(prevTimer);
            }
            const timer = setTimeout(() => {
                debounceTimers.delete(key);
                const seq = (fetchSeqMap.get(key) ?? 0) + 1;
                fetchSeqMap.set(key, seq);

                const store = useMCPStore.getState();
                if (normalizedMcpID === "") {
                    void api.ListMCPServers(sessionName)
                        .then((result) => {
                            if (!isMountedRef.current) {
                                return;
                            }
                            if ((fetchSeqMap.get(key) ?? 0) !== seq) {
                                return;
                            }
                            store.setSnapshots(sessionName, result ?? []);
                            store.setSessionError(sessionName, null);
                            mcpSyncFailureCounter.recordSuccess();
                        })
                        .catch((err: unknown) => {
                            if (!isMountedRef.current) {
                                return;
                            }
                            if ((fetchSeqMap.get(key) ?? 0) !== seq) {
                                return;
                            }
                            const message = err instanceof Error ? err.message : String(err);
                            store.setSessionError(sessionName, message);
                            mcpSyncFailureCounter.recordFailure(() => {
                                notifyWarn(`MCP state refresh failed (${sessionName}): ${message}`);
                                logFrontendEventSafe("warn", `MCP state refresh failed (${sessionName}): ${message}`, "frontend/mcp");
                            });
                            if (import.meta.env.DEV) {
                                console.warn("[SYNC] ListMCPServers failed:", err);
                            }
                        });
                    return;
                }

                void api.GetMCPDetail(sessionName, normalizedMcpID)
                    .then((detail) => {
                        if (!isMountedRef.current) {
                            return;
                        }
                        if ((fetchSeqMap.get(key) ?? 0) !== seq) {
                            return;
                        }
                        store.upsertSnapshot(sessionName, detail);
                        store.setSessionError(sessionName, null);
                        mcpSyncFailureCounter.recordSuccess();
                    })
                    .catch((err: unknown) => {
                        if (!isMountedRef.current) {
                            return;
                        }
                        if ((fetchSeqMap.get(key) ?? 0) !== seq) {
                            return;
                        }
                        const message = err instanceof Error ? err.message : String(err);
                        store.setSessionError(sessionName, message);
                        mcpSyncFailureCounter.recordFailure(() => {
                            notifyWarn(`MCP detail refresh failed (${sessionName}/${normalizedMcpID}): ${message}`);
                            logFrontendEventSafe(
                                "warn",
                                `MCP detail refresh failed (${sessionName}/${normalizedMcpID}): ${message}`,
                                "frontend/mcp",
                            );
                        });
                        if (import.meta.env.DEV) {
                            console.warn("[SYNC] GetMCPDetail failed:", err);
                        }
                    });
            }, DEBOUNCE_MS);
            debounceTimers.set(key, timer);
        };

        onEvent("mcp:state-changed", (payload) => {
            if (!isMountedRef.current) return;
            const event = asObject<{session_name?: unknown; mcp_id?: unknown}>(payload);
            if (!event) {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] mcp:state-changed: invalid payload", payload);
                }
                return;
            }
            const sessionName = typeof event.session_name === "string" ? event.session_name.trim() : "";
            if (sessionName === "") {
                if (import.meta.env.DEV) {
                    console.warn("[SYNC] mcp:state-changed: empty session_name", payload);
                }
                return;
            }
            const mcpID = typeof event.mcp_id === "string" ? event.mcp_id.trim() : "";
            scheduleMCPRefresh(sessionName, mcpID === "" ? null : mcpID);
        });

        onEvent("mcp:manager-closed", () => {
            if (!isMountedRef.current) return;
            for (const timer of debounceTimers.values()) {
                clearTimeout(timer);
            }
            debounceTimers.clear();
            fetchSeqMap.clear();
            useMCPStore.getState().clearAllSessions();
        });

        return () => {
            isMountedRef.current = false;
            for (const timer of debounceTimers.values()) {
                clearTimeout(timer);
            }
            debounceTimers.clear();
            fetchSeqMap.clear();
            cleanupEventListeners(cleanupFns);
        };
    }, []);
}
