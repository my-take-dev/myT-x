import {create} from "zustand";
import type {
    CanvasNodePosition,
    CanvasNodeSize,
    OrchestratorAgent,
    OrchestratorTask,
    PaneProcessStatus,
} from "../types/canvas";

/** taskEdgeMap の最大エントリ数。超過時は古いエントリを破棄する。 */
const MAX_TASK_EDGES = 500;

interface CanvasState {
    mode: "simple" | "canvas";
    setMode: (mode: "simple" | "canvas") => void;

    /** 現在キャンバスが紐づいているセッション名。セッション切替時にリセットする。 */
    activeSessionName: string | null;

    nodePositions: Record<string, CanvasNodePosition>;
    setNodePosition: (paneId: string, pos: CanvasNodePosition) => void;

    nodeSizes: Record<string, CanvasNodeSize>;
    setNodeSize: (paneId: string, size: CanvasNodeSize) => void;

    /** タスクエッジ（task_id → task）。セッション内で累積。 */
    taskEdgeMap: Record<string, OrchestratorTask>;
    updateTaskEdges: (tasks: OrchestratorTask[]) => void;

    /** エージェント情報（pane_id → agent）。 */
    agentMap: Record<string, OrchestratorAgent>;
    updateAgents: (agents: OrchestratorAgent[]) => void;

    /** プロセス実行状態（pane_id → hasChildProcess）。 */
    processStatusMap: Record<string, boolean>;
    updateProcessStatus: (statuses: PaneProcessStatus[]) => void;

    /** セッション切替時にセッション固有状態をリセットする。 */
    resetForSession: (sessionName: string) => void;
}

export const useCanvasStore = create<CanvasState>((set) => ({
    mode: "simple",
    setMode: (mode) => set({mode}),

    activeSessionName: null,

    nodePositions: {},
    setNodePosition: (paneId, pos) =>
        set((state) => ({
            nodePositions: {...state.nodePositions, [paneId]: pos},
        })),

    nodeSizes: {},
    setNodeSize: (paneId, size) =>
        set((state) => ({
            nodeSizes: {...state.nodeSizes, [paneId]: size},
        })),

    taskEdgeMap: {},
    updateTaskEdges: (tasks) =>
        set((state) => {
            const next = {...state.taskEdgeMap};
            for (const task of tasks) {
                next[task.task_id] = task;
            }
            // 最大エントリ数を超過した場合、完了/失敗済みの古いエントリ（sent_at昇順）から破棄。
            // pending/assigned は保護する。
            const keys = Object.keys(next);
            if (keys.length > MAX_TASK_EDGES) {
                const removable = keys.filter((k) => {
                    const s = next[k].status;
                    return s === "completed" || s === "failed" || s === "abandoned";
                });
                const sorted = removable.sort((a, b) => {
                    const ta = next[a].sent_at;
                    const tb = next[b].sent_at;
                    return ta < tb ? -1 : ta > tb ? 1 : 0;
                });
                const removeCount = Math.min(sorted.length, keys.length - MAX_TASK_EDGES);
                for (let i = 0; i < removeCount; i++) {
                    delete next[sorted[i]];
                }
            }
            return {taskEdgeMap: next};
        }),

    agentMap: {},
    updateAgents: (agents) =>
        set(() => {
            const next: Record<string, OrchestratorAgent> = {};
            for (const ag of agents) {
                next[ag.pane_id] = ag;
            }
            return {agentMap: next};
        }),

    processStatusMap: {},
    updateProcessStatus: (statuses) =>
        set(() => {
            const next: Record<string, boolean> = {};
            for (const s of statuses) {
                next[s.pane_id] = s.has_child_process;
            }
            return {processStatusMap: next};
        }),

    resetForSession: (sessionName) =>
        set(() => ({
            activeSessionName: sessionName,
            taskEdgeMap: {},
            nodePositions: {},
            nodeSizes: {},
            agentMap: {},
            processStatusMap: {},
        })),
}));
