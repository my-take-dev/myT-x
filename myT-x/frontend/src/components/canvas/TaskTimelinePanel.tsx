import {useCallback, useMemo, useState} from "react";
import {useCanvasStore} from "../../stores/canvasStore";
import type {OrchestratorTaskDetail, OrchestratorTask} from "../../types/canvas";
import {api} from "../../api";
import {notifyAndLog} from "../../utils/notifyUtils";

interface TaskTimelinePanelProps {
    sessionName: string;
    onClose: () => void;
}

/** ステータスに対応する色 */
const STATUS_COLORS: Record<string, string> = {
    pending: "#58a6ff",
    completed: "rgba(63, 185, 80, 0.7)",
    failed: "#ff4444",
    abandoned: "#666",
};

/** ステータスの日本語表示 */
const STATUS_LABELS: Record<string, string> = {
    pending: "実行中",
    completed: "完了",
    failed: "失敗",
    abandoned: "中断",
};

/** sent_at を表示用に短縮フォーマット */
function formatTime(iso: string): string {
    if (!iso) return "";
    try {
        const d = new Date(iso);
        const h = String(d.getHours()).padStart(2, "0");
        const m = String(d.getMinutes()).padStart(2, "0");
        const s = String(d.getSeconds()).padStart(2, "0");
        return `${h}:${m}:${s}`;
    } catch {
        return iso;
    }
}

// --- TaskTicket ---

interface TaskTicketProps {
    task: OrchestratorTask;
    sessionName: string;
    isExpanded: boolean;
    onToggleExpand: (taskId: string) => void;
}

function TaskTicket({task, sessionName, isExpanded, onToggleExpand}: TaskTicketProps) {
    const [detail, setDetail] = useState<OrchestratorTaskDetail | null>(null);
    const [loading, setLoading] = useState(false);

    const handleClick = useCallback(() => {
        // 展開するときに詳細を取得
        if (!isExpanded && !detail) {
            setLoading(true);
            api.GetOrchestratorTaskDetail(sessionName, task.task_id)
                .then((d) => setDetail(d))
                .catch((err) => {
                    console.warn("[DEBUG-timeline] fetch detail failed:", err);
                    notifyAndLog("Load task detail", "warn", err, "TaskTimeline");
                })
                .finally(() => setLoading(false));
        }
        onToggleExpand(task.task_id);
    }, [isExpanded, detail, sessionName, task.task_id, onToggleExpand]);

    const statusClass = `status-${task.status}`;

    return (
        <div
            className={`canvas-timeline-ticket ${statusClass} ${isExpanded ? "expanded" : ""}`}
            onClick={handleClick}
        >
            {/* ヘッダー: from → to + ステータス */}
            <div className="canvas-timeline-ticket-header">
                <div className="canvas-timeline-ticket-from-to">
                    <span className="canvas-timeline-ticket-sender">{task.sender_name || "?"}</span>
                    <span className="canvas-timeline-ticket-arrow"> → </span>
                    <span className="canvas-timeline-ticket-agent">{task.agent_name}</span>
                </div>
                <span
                    className="canvas-timeline-ticket-badge"
                    style={{color: STATUS_COLORS[task.status] ?? "#666"}}
                >
                    {STATUS_LABELS[task.status] ?? task.status}
                </span>
            </div>

            {/* 時刻 */}
            <div className="canvas-timeline-ticket-time">
                {formatTime(task.sent_at)}
                {task.completed_at && ` → ${formatTime(task.completed_at)}`}
            </div>

            {/* 依頼プレビュー */}
            {task.message_preview && (
                <div className="canvas-timeline-ticket-message">
                    {task.message_preview}
                    {task.message_preview.length >= 80 && "…"}
                </div>
            )}

            {/* 応答プレビュー（折りたたみ時） */}
            {!isExpanded && task.response_preview && (
                <div className="canvas-timeline-ticket-response">
                    {task.response_preview}
                    {task.response_preview.length >= 80 && "…"}
                </div>
            )}

            {/* 展開時: 全文表示 */}
            {isExpanded && (
                <div className="canvas-timeline-ticket-detail">
                    {loading && <div className="canvas-timeline-ticket-loading">読込中...</div>}
                    {detail && (
                        <>
                            <div className="canvas-timeline-ticket-section">
                                <div className="canvas-timeline-ticket-section-label">依頼</div>
                                <div className="canvas-timeline-ticket-message canvas-timeline-ticket-full">
                                    {detail.message_content || "(なし)"}
                                </div>
                            </div>
                            {detail.response_content && (
                                <div className="canvas-timeline-ticket-section">
                                    <div className="canvas-timeline-ticket-section-label">応答</div>
                                    <div className="canvas-timeline-ticket-response canvas-timeline-ticket-full">
                                        {detail.response_content}
                                    </div>
                                </div>
                            )}
                        </>
                    )}
                    {!loading && !detail && (
                        <div className="canvas-timeline-ticket-loading">詳細取得に失敗</div>
                    )}
                </div>
            )}
        </div>
    );
}

// --- TaskTimelinePanel ---

export function TaskTimelinePanel({sessionName, onClose}: TaskTimelinePanelProps) {
    const taskEdgeMap = useCanvasStore((s) => s.taskEdgeMap);
    const [expandedTaskId, setExpandedTaskId] = useState<string | null>(null);

    // sent_at降順でソート
    const sortedTasks = useMemo(() => {
        return Object.values(taskEdgeMap)
            .sort((a, b) => b.sent_at.localeCompare(a.sent_at));
    }, [taskEdgeMap]);

    const handleToggleExpand = useCallback((taskId: string) => {
        setExpandedTaskId((prev) => (prev === taskId ? null : taskId));
    }, []);

    return (
        <div className="canvas-timeline-panel">
            <div className="canvas-timeline-header">
                <span>タスクタイムライン</span>
                <button
                    type="button"
                    className="canvas-timeline-close-btn"
                    onClick={onClose}
                    title="閉じる"
                >
                    ✕
                </button>
            </div>
            <div className="canvas-timeline-list">
                {sortedTasks.length === 0 && (
                    <div className="canvas-timeline-empty">タスクなし</div>
                )}
                {sortedTasks.map((task) => (
                    <TaskTicket
                        key={task.task_id}
                        task={task}
                        sessionName={sessionName}
                        isExpanded={expandedTaskId === task.task_id}
                        onToggleExpand={handleToggleExpand}
                    />
                ))}
            </div>
        </div>
    );
}
