import {useEffect} from "react";
import {EventsOn} from "../../wailsjs/runtime/runtime";
import {api} from "../api";
import {useCanvasStore} from "../stores/canvasStore";
import {createConsecutiveFailureCounter, notifyAndLog} from "../utils/notifyUtils";

const POLL_INTERVAL_MS = 3000;

// Module-level consecutive failure counter for canvas polling.
// 3-second polling can produce many failures when backend is disconnected;
// only notify after 3 consecutive failures (≈9 seconds of sustained failure).
const canvasSyncFailureCounter = createConsecutiveFailureCounter(3);

function eventTargetsSession(payload: unknown, sessionName: string): boolean {
    if (!payload || typeof payload !== "object") {
        return false;
    }
    const candidate = (payload as {sessionName?: unknown}).sessionName;
    return typeof candidate === "string" && candidate === sessionName;
}

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
        let timerId: ReturnType<typeof setTimeout> | null = null;
        let pollInFlight = false;
        let pollPending = false;

        const schedulePoll = (delayMs: number): void => {
            if (cancelled) {
                return;
            }
            if (timerId != null) {
                clearTimeout(timerId);
            }
            timerId = setTimeout(() => {
                timerId = null;
                void poll();
            }, delayMs);
        };

        const poll = async (): Promise<void> => {
            if (cancelled) {
                return;
            }
            if (pollInFlight) {
                pollPending = true;
                return;
            }
            pollInFlight = true;
            try {
                const results = await Promise.allSettled([
                    api.ListOrchestratorTasks(sessionName),
                    api.ListOrchestratorAgents(sessionName),
                    api.GetPaneProcessStatus(sessionName),
                ]);
                if (cancelled) return;

                const rejectedReasons = results
                    .filter((r): r is PromiseRejectedResult => r.status === "rejected")
                    .map((r) => r.reason instanceof Error ? r.reason.message : String(r.reason));

                for (const reason of rejectedReasons) {
                    console.warn("[canvas-sync] poll partial failure", reason);
                }

                // Only record failure when ALL requests failed. Partial success
                // (e.g., 2/3 succeeded) resets the counter so that intermittent
                // single-endpoint failures do not accumulate to the threshold.
                if (rejectedReasons.length === results.length) {
                    canvasSyncFailureCounter.recordFailure(() => {
                        notifyAndLog("Canvas data sync", "warn", new Error(rejectedReasons.join("; ")), "CanvasTaskSync");
                    });
                } else {
                    canvasSyncFailureCounter.recordSuccess();
                }

                if (results[0].status === "fulfilled" && results[0].value) {
                    useCanvasStore.getState().updateTaskEdges(results[0].value);
                }
                if (results[1].status === "fulfilled" && results[1].value) {
                    useCanvasStore.getState().updateAgents(results[1].value);
                } else if (results[1].status === "rejected") {
                    // Fail closed for auto-layout root selection: stale agent roles can
                    // pick the wrong root after a partial polling failure.
                    useCanvasStore.getState().clearAgents();
                }
                if (results[2].status === "fulfilled" && results[2].value) {
                    useCanvasStore.getState().updateProcessStatus(results[2].value);
                }
            } catch (err) {
                console.warn("[canvas-sync] poll failed", err);
                canvasSyncFailureCounter.recordFailure(() => {
                    notifyAndLog("Canvas data sync", "warn", err, "CanvasTaskSync");
                });
            } finally {
                pollInFlight = false;
            }

            if (cancelled) {
                return;
            }
            if (pollPending) {
                pollPending = false;
                schedulePoll(0);
                return;
            }
            schedulePoll(POLL_INTERVAL_MS);
        };

        // 初回即時実行
        void poll();

        const cancelAgentsUpdated = EventsOn("orchestrator:agents-updated", (payload: unknown) => {
            if (!eventTargetsSession(payload, sessionName)) {
                return;
            }
            if (timerId != null) {
                clearTimeout(timerId);
                timerId = null;
            }
            void poll();
        });

        return () => {
            cancelled = true;
            cancelAgentsUpdated();
            if (timerId != null) {
                clearTimeout(timerId);
            }
        };
    }, [mode, sessionName]);
}
