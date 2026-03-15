import {type FC, useState} from "react";
import {BaseEdge, type EdgeProps, getSmoothStepPath} from "@xyflow/react";

interface TaskEdgeData {
    senderName: string;
    agentName: string;
    status: string;
    sentAt: string;
    [key: string]: unknown;
}

export const TaskEdge: FC<EdgeProps> = (props) => {
    const {
        sourceX, sourceY, sourcePosition,
        targetX, targetY, targetPosition,
        data,
    } = props;
    const [hovered, setHovered] = useState(false);

    const edgeData = data as TaskEdgeData | undefined;
    const status = edgeData?.status ?? "pending";

    const strokeColor = status === "pending" ? "#ff4444"
        : status === "completed" ? "rgba(255,255,255,0.6)"
        : "#666";

    const animated = status === "pending";

    const [edgePath, labelX, labelY] = getSmoothStepPath({
        sourceX, sourceY, sourcePosition,
        targetX, targetY, targetPosition,
        borderRadius: 16,
    });

    return (
        <>
            <BaseEdge
                path={edgePath}
                style={{
                    stroke: strokeColor,
                    strokeWidth: 2,
                    strokeDasharray: (status === "failed" || status === "abandoned") ? "5,5" : undefined,
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
            {hovered && edgeData && (
                <foreignObject
                    x={labelX - 100}
                    y={labelY - 50}
                    width={200}
                    height={80}
                    style={{overflow: "visible", pointerEvents: "none"}}
                >
                    <div className="canvas-edge-tooltip">
                        <div className="canvas-edge-tooltip-row">
                            <strong>{edgeData.senderName || "?"}</strong>
                            <span> → </span>
                            <strong>{edgeData.agentName || "?"}</strong>
                        </div>
                        <div className="canvas-edge-tooltip-row canvas-edge-tooltip-status">
                            {status}
                        </div>
                        {edgeData.sentAt && (
                            <div className="canvas-edge-tooltip-row canvas-edge-tooltip-time">
                                {new Date(edgeData.sentAt).toLocaleTimeString()}
                            </div>
                        )}
                    </div>
                </foreignObject>
            )}
        </>
    );
};
