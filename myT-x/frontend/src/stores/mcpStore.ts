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

    /** Replace the full MCP snapshot list for a session. */
    setSnapshots: (sessionName: string, snapshots: MCPSnapshot[]) => void;
    /** Insert or replace one MCP snapshot within a session list. */
    upsertSnapshot: (sessionName: string, snapshot: MCPSnapshot) => void;
    /** Atomically begin a session load (loading=true, error=null). */
    beginSessionLoad: (sessionName: string) => void;
    /** Set loading state for a single session. */
    setSessionLoading: (sessionName: string, loading: boolean) => void;
    /** Set error state for a single session. */
    setSessionError: (sessionName: string, error: string | null) => void;
    /** Remove all MCP state for a destroyed session. */
    clearSession: (sessionName: string) => void;
    /** Remove all MCP state across all sessions. */
    clearAllSessions: () => void;
}

export const useMCPStore = create<MCPState>((set) => ({
    snapshots: {},
    sessionStates: {},

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

    clearSession: (sessionName) =>
        set((state) => {
            const nextSnapshots = {...state.snapshots};
            delete nextSnapshots[sessionName];
            const nextSessionStates = {...state.sessionStates};
            delete nextSessionStates[sessionName];

            return {
                snapshots: nextSnapshots,
                sessionStates: nextSessionStates,
            };
        }),

    clearAllSessions: () =>
        set({
            snapshots: {},
            sessionStates: {},
        }),
}));
