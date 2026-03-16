import {type FC, useState} from "react";
import {BaseEdge, type EdgeProps, getSmoothStepPath} from "@xyflow/react";
import type {AggregatedTaskInfo, AggregateStatus} from "../../utils/aggregateTaskEdges";
import {computeEdgeOffset} from "../../utils/edgeRouting";

export interface TaskEdgeData {
    aggregateStatus: AggregateStatus;
    totalCount: number;
    pendingCount: number;
    completedCount: number;
    failedCount: number;
    abandonedCount: number;
    forwardTasks: AggregatedTaskInfo[];
    reverseTasks: AggregatedTaskInfo[];
    sourceName: string;
    targetName: string;
    [key: string]: unknown;
}

const STATUS_COLORS: Record<AggregateStatus, string> = {
    pending: "#58a6ff",
    completed: "rgba(63, 185, 80, 0.7)",
    failed: "#ff4444",
    abandoned: "#666",
};

function getStrokeWidth(totalCount: number): number {
    if (totalCount >= 6) return 4;
    if (totalCount >= 2) return 3;
    return 2;
}

export const TaskEdge: FC<EdgeProps> = (props) => {
    const {
        sourceX, sourceY, sourcePosition,
        targetX, targetY, targetPosition,
        data,
    } = props;
    const [hovered, setHovered] = useState(false);

    const edgeData = data as TaskEdgeData | undefined;
    const status = edgeData?.aggregateStatus ?? "abandoned";
    const totalCount = edgeData?.totalCount ?? 0;

    const strokeColor = STATUS_COLORS[status];
    const strokeWidth = getStrokeWidth(totalCount);
    const animated = status === "pending";
    const dashed = status === "failed" || status === "abandoned";

    const dynamicOffset = computeEdgeOffset(sourceY, targetY);

    const [edgePath, labelX, labelY] = getSmoothStepPath({
        sourceX, sourceY, sourcePosition,
        targetX, targetY, targetPosition,
        borderRadius: 16,
        offset: dynamicOffset,
    });

    // ツールチップ用: 直近タスク5件（forward + reverse を sent_at 降順でマージ）
    const recentTasks = edgeData
        ? [...edgeData.forwardTasks.map((t) => ({...t, dir: "→" as const})),
            ...edgeData.reverseTasks.map((t) => ({...t, dir: "←" as const}))]
            .sort((a, b) => b.sentAt.localeCompare(a.sentAt))
            .slice(0, 5)
        : [];

    return (
        <>
            <BaseEdge
                path={edgePath}
                style={{
                    stroke: strokeColor,
                    strokeWidth,
                    strokeDasharray: dashed ? "5,5" : undefined,
                }}
                className={animated ? "canvas-edge-animated" : ""}
            />
            {/* ホバー判定用の透明な太い線 */}
            <path
                d={edgePath}
                fill="none"
                stroke="transparent"
                strokeWidth={16}
                onMouseEnter={() => setHovered(true)}
                onMouseLeave={() => setHovered(false)}
            />
            {/* タスク件数バッジ（2件以上のとき） */}
            {totalCount > 1 && (
                <foreignObject
                    x={labelX - 12}
                    y={labelY - 10}
                    width={24}
                    height={20}
                    style={{overflow: "visible", pointerEvents: "none"}}
                >
                    <div className="canvas-edge-badge">{totalCount}</div>
                </foreignObject>
            )}
            {/* ホバー時ツールチップ */}
            {hovered && edgeData && (
                <foreignObject
                    x={labelX - 140}
                    y={labelY - 90}
                    width={280}
                    height={180}
                    style={{overflow: "visible", pointerEvents: "none"}}
                >
                    <div className="canvas-edge-tooltip">
                        <div className="canvas-edge-tooltip-row">
                            <strong>{edgeData.sourceName}</strong>
                            <span> ↔ </span>
                            <strong>{edgeData.targetName}</strong>
                        </div>
                        <div className="canvas-edge-tooltip-summary">
                            {edgeData.pendingCount > 0 && (
                                <span className="canvas-edge-stat canvas-edge-stat-pending">
                                    ● {edgeData.pendingCount}
                                </span>
                            )}
                            {edgeData.completedCount > 0 && (
                                <span className="canvas-edge-stat canvas-edge-stat-completed">
                                    ● {edgeData.completedCount}
                                </span>
                            )}
                            {edgeData.failedCount > 0 && (
                                <span className="canvas-edge-stat canvas-edge-stat-failed">
                                    ● {edgeData.failedCount}
                                </span>
                            )}
                            {edgeData.abandonedCount > 0 && (
                                <span className="canvas-edge-stat canvas-edge-stat-abandoned">
                                    ● {edgeData.abandonedCount}
                                </span>
                            )}
                        </div>
                        {recentTasks.length > 0 && (
                            <div className="canvas-edge-tooltip-task-list">
                                {recentTasks.map((t, i) => (
                                    <div key={i} className="canvas-edge-tooltip-task-item">
                                        <span className="canvas-edge-tooltip-direction">{t.dir}</span>
                                        {" "}
                                        {t.senderName} → {t.agentName}
                                        {" "}
                                        <span className={`canvas-edge-stat-${t.status}`}>
                                            ({t.status})
                                        </span>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                </foreignObject>
            )}
        </>
    );
};
