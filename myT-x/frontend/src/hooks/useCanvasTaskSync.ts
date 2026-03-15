import {useEffect, useRef} from "react";
import {api} from "../api";
import {useCanvasStore} from "../stores/canvasStore";

const POLL_INTERVAL_MS = 3000;

/**
 * Canvas Mode が有効な間、3秒間隔でオーケストレーターのタスク・エージェント・
 * プロセス状態をポーリングし、canvasStore に反映する。
 * セッション変更時にストアをリセットする。
 */
export function useCanvasTaskSync(sessionName: string | null): void {
    const mode = useCanvasStore((s) => s.mode);
    const resetForSession = useCanvasStore((s) => s.resetForSession);
    const mountedRef = useRef(true);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

    // セッション変更時にリセット
    useEffect(() => {
        if (sessionName) {
            resetForSession(sessionName);
        }
    }, [sessionName, resetForSession]);

    useEffect(() => {
        if (mode !== "canvas" || !sessionName) return;

        let cancelled = false;

        const poll = async (): Promise<void> => {
            try {
                const [tasks, agents, processStatus] = await Promise.all([
                    api.ListOrchestratorTasks(sessionName),
                    api.ListOrchestratorAgents(sessionName),
                    api.GetPaneProcessStatus(sessionName),
                ]);
                if (cancelled || !mountedRef.current) return;

                if (tasks) useCanvasStore.getState().updateTaskEdges(tasks);
                if (agents) useCanvasStore.getState().updateAgents(agents);
                if (processStatus) useCanvasStore.getState().updateProcessStatus(processStatus);
            } catch (err) {
                console.warn("[DEBUG-canvas-sync] poll failed", err);
            }
        };

        // 初回即時実行
        void poll();

        const id = setInterval(() => {
            void poll();
        }, POLL_INTERVAL_MS);

        return () => {
            cancelled = true;
            clearInterval(id);
        };
    }, [mode, sessionName]);
}
