import type {OrchestratorTask} from "../types/canvas";

export interface AggregatedTaskInfo {
    senderName: string;
    agentName: string;
    status: string;
    sentAt: string;
}

export type AggregateStatus = "pending" | "completed" | "failed" | "abandoned";

export interface AggregatedEdge {
    /** 小さい方の pane ID（React Flow source） */
    paneA: string;
    /** 大きい方の pane ID（React Flow target） */
    paneB: string;
    /** 集約ステータス */
    aggregateStatus: AggregateStatus;
    /** タスク総数 */
    totalCount: number;
    /** ステータス別件数 */
    pendingCount: number;
    completedCount: number;
    failedCount: number;
    abandonedCount: number;
    /** paneA→paneB 方向のタスク（sent_at降順） */
    forwardTasks: AggregatedTaskInfo[];
    /** paneB→paneA 方向のタスク（sent_at降順） */
    reverseTasks: AggregatedTaskInfo[];
    /** 表示用: source側の名前 */
    sourceName: string;
    /** 表示用: target側の名前 */
    targetName: string;
}

/**
 * タスクを無方向ペアでグループ化し、ペアごとに1つの集約エッジを返す。
 *
 * 集約ステータス決定ロジック（最新タスク優先）:
 * 1. pending タスクが1件でもあれば "pending"
 * 2. なければ sent_at 最新のタスクのステータスを採用
 */
export function aggregateTaskEdges(
    tasks: OrchestratorTask[],
    validPaneIds: Set<string>,
): AggregatedEdge[] {
    // 1. フィルタ: 両方のpane_idが存在し、自己参照でないタスク
    const validTasks = tasks.filter((t) =>
        t.sender_pane_id
        && t.assignee_pane_id
        && t.sender_pane_id !== t.assignee_pane_id
        && validPaneIds.has(t.sender_pane_id)
        && validPaneIds.has(t.assignee_pane_id),
    );

    // 2. 無方向ペアキーでグループ化
    const pairMap = new Map<string, OrchestratorTask[]>();
    for (const task of validTasks) {
        const a = task.sender_pane_id;
        const b = task.assignee_pane_id;
        const key = a < b ? `${a}\0${b}` : `${b}\0${a}`;
        const list = pairMap.get(key);
        if (list) {
            list.push(task);
        } else {
            pairMap.set(key, [task]);
        }
    }

    // 3. 各ペアで集約エッジを生成
    const result: AggregatedEdge[] = [];
    for (const [key, groupTasks] of pairMap) {
        const [paneA, paneB] = key.split("\0");

        let pendingCount = 0;
        let completedCount = 0;
        let failedCount = 0;
        let abandonedCount = 0;
        const forwardTasks: AggregatedTaskInfo[] = [];
        const reverseTasks: AggregatedTaskInfo[] = [];

        for (const t of groupTasks) {
            // ステータス別カウント
            switch (t.status) {
                case "pending":
                    pendingCount++;
                    break;
                case "completed":
                    completedCount++;
                    break;
                case "failed":
                    failedCount++;
                    break;
                default:
                    abandonedCount++;
                    break;
            }

            // 方向別分類
            const info: AggregatedTaskInfo = {
                senderName: t.sender_name,
                agentName: t.agent_name,
                status: t.status,
                sentAt: t.sent_at,
            };
            if (t.sender_pane_id === paneA) {
                forwardTasks.push(info);
            } else {
                reverseTasks.push(info);
            }
        }

        // sent_at降順ソート
        const sortDesc = (a: AggregatedTaskInfo, b: AggregatedTaskInfo) =>
            b.sentAt.localeCompare(a.sentAt);
        forwardTasks.sort(sortDesc);
        reverseTasks.sort(sortDesc);

        // 集約ステータス決定: pending優先、なければ最新タスクのステータス
        let aggregateStatus: AggregateStatus;
        if (pendingCount > 0) {
            aggregateStatus = "pending";
        } else {
            // sent_at最新のタスクを探す
            let latest = groupTasks[0];
            for (let i = 1; i < groupTasks.length; i++) {
                if (groupTasks[i].sent_at > latest.sent_at) {
                    latest = groupTasks[i];
                }
            }
            aggregateStatus = toAggregateStatus(latest.status);
        }

        // 表示用名前: 各方向の最初のタスクから取得
        const sourceName = forwardTasks[0]?.senderName
            ?? reverseTasks[0]?.agentName ?? paneA;
        const targetName = reverseTasks[0]?.senderName
            ?? forwardTasks[0]?.agentName ?? paneB;

        result.push({
            paneA,
            paneB,
            aggregateStatus,
            totalCount: groupTasks.length,
            pendingCount,
            completedCount,
            failedCount,
            abandonedCount,
            forwardTasks,
            reverseTasks,
            sourceName,
            targetName,
        });
    }

    return result;
}

function toAggregateStatus(status: string): AggregateStatus {
    if (status === "pending" || status === "completed" || status === "failed" || status === "abandoned") {
        return status;
    }
    return "abandoned";
}
