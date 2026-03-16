import {useEffect} from "react";
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
                const results = await Promise.allSettled([
                    api.ListOrchestratorTasks(sessionName),
                    api.ListOrchestratorAgents(sessionName),
                    api.GetPaneProcessStatus(sessionName),
                ]);
                if (cancelled) return;

                for (const result of results) {
                    if (result.status === "rejected") {
                        console.warn("[canvas-sync] poll partial failure", result.reason);
                    }
                }

                if (results[0].status === "fulfilled" && results[0].value) {
                    useCanvasStore.getState().updateTaskEdges(results[0].value);
                }
                if (results[1].status === "fulfilled" && results[1].value) {
                    useCanvasStore.getState().updateAgents(results[1].value);
                }
                if (results[2].status === "fulfilled" && results[2].value) {
                    useCanvasStore.getState().updateProcessStatus(results[2].value);
                }
            } catch (err) {
                console.warn("[canvas-sync] poll failed", err);
            }

            if (!cancelled) {
                timerId = setTimeout(() => {
                    void poll();
                }, POLL_INTERVAL_MS);
            }
        };

        let timerId: ReturnType<typeof setTimeout> | null = null;

        // 初回即時実行
        void poll();

        return () => {
            cancelled = true;
            if (timerId != null) {
                clearTimeout(timerId);
            }
        };
    }, [mode, sessionName]);
}
