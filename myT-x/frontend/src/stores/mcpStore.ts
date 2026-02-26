import {create} from "zustand";
import type {MCPSnapshot} from "../types/mcp";

export interface SessionLoadState {
    loading: boolean;
    error: string | null;
}

interface MCPState {
    /** MCP snapshots keyed by session name. */
    snapshots: Record<string, MCPSnapshot[]>;
    /** Session-scoped loading and error state. */
    sessionStates: Record<string, SessionLoadState>;
    /** Currently selected MCP ID in the detail panel. */
    selectedMCPId: string | null;

    /** Replace the full MCP snapshot list for a session. */
    setSnapshots: (sessionName: string, snapshots: MCPSnapshot[]) => void;
    /** Insert or replace one MCP snapshot within a session list. */
    upsertSnapshot: (sessionName: string, snapshot: MCPSnapshot) => void;
    /** Atomically begin a session load (loading=true, error=null). */
    beginSessionLoad: (sessionName: string) => void;
    /** Partially update a single MCP entry within a session. */
    updateMCPState: (sessionName: string, mcpId: string, partial: Partial<MCPSnapshot>) => void;
    /** Set loading state for a single session. */
    setSessionLoading: (sessionName: string, loading: boolean) => void;
    /** Set error state for a single session. */
    setSessionError: (sessionName: string, error: string | null) => void;
    /** Set the selected MCP ID for detail view. */
    selectMCP: (mcpId: string | null) => void;
    /** Remove all MCP state for a destroyed session. */
    clearSession: (sessionName: string) => void;
    /** Remove all MCP state across all sessions. */
    clearAllSessions: () => void;
}

export const useMCPStore = create<MCPState>((set) => ({
    snapshots: {},
    sessionStates: {},
    selectedMCPId: null,

    setSnapshots: (sessionName, snapshots) =>
        set((state) => ({
            snapshots: {...state.snapshots, [sessionName]: snapshots},
        })),

    upsertSnapshot: (sessionName, snapshot) =>
        set((state) => {
            const current = state.snapshots[sessionName] ?? [];
            const index = current.findIndex((item) => item.id === snapshot.id);
            const updated = [...current];
            if (index >= 0) {
                updated[index] = snapshot;
            } else {
                updated.push(snapshot);
            }
            return {snapshots: {...state.snapshots, [sessionName]: updated}};
        }),

    beginSessionLoad: (sessionName) =>
        set((state) => {
            const prev = state.sessionStates[sessionName] ?? {loading: false, error: null};
            return {
                sessionStates: {
                    ...state.sessionStates,
                    [sessionName]: {...prev, loading: true, error: null},
                },
            };
        }),

    updateMCPState: (sessionName, mcpId, partial) =>
        set((state) => {
            const current = state.snapshots[sessionName] ?? [];
            const updated = current.map((s) =>
                s.id === mcpId ? {...s, ...partial} : s,
            );
            return {snapshots: {...state.snapshots, [sessionName]: updated}};
        }),

    setSessionLoading: (sessionName, loading) =>
        set((state) => {
            const prev = state.sessionStates[sessionName] ?? {loading: false, error: null};
            return {
                sessionStates: {
                    ...state.sessionStates,
                    [sessionName]: {...prev, loading},
                },
            };
        }),

    setSessionError: (sessionName, error) =>
        set((state) => {
            const prev = state.sessionStates[sessionName] ?? {loading: false, error: null};
            return {
                sessionStates: {
                    ...state.sessionStates,
                    [sessionName]: {...prev, error},
                },
            };
        }),

    selectMCP: (mcpId) => set({selectedMCPId: mcpId}),

    clearSession: (sessionName) =>
        set((state) => {
            const nextSnapshots = {...state.snapshots};
            const removed = nextSnapshots[sessionName] ?? [];
            delete nextSnapshots[sessionName];
            const nextSessionStates = {...state.sessionStates};
            delete nextSessionStates[sessionName];

            const selectedRemoved = removed.some((m) => m.id === state.selectedMCPId);

            return {
                snapshots: nextSnapshots,
                sessionStates: nextSessionStates,
                selectedMCPId: selectedRemoved ? null : state.selectedMCPId,
            };
        }),

    clearAllSessions: () =>
        set({
            snapshots: {},
            sessionStates: {},
            selectedMCPId: null,
        }),
}));
