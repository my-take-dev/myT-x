import {useI18n} from "../../../../i18n";
import type {QueueStatus} from "./useTaskScheduler";

interface TaskSchedulerListProps {
    status: QueueStatus | null;
    onNew: () => void;
    onEdit: (id: string) => void;
    onRemove: (id: string) => Promise<void>;
    onStart: () => void;
    onStop: () => Promise<void>;
    onPause: () => Promise<void>;
    onResume: () => Promise<void>;
    isRunning: boolean;
}

const STATUS_ICONS: Record<string, string> = {
    pending: "⏳",
    running: "▶",
    completed: "✅",
    failed: "❌",
    skipped: "⏭",
};

export function TaskSchedulerList({
    status,
    onNew,
    onEdit,
    onRemove,
    onStart,
    onStop,
    onPause,
    onResume,
    isRunning,
}: TaskSchedulerListProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const items = status?.items ?? [];
    const runStatus = status?.run_status ?? "idle";
    const hasPendingItems = items.some((i) => i.status === "pending");

    return (
        <div className="task-scheduler-list">
            <div className="task-scheduler-toolbar">
                {!isRunning && hasPendingItems && (
                    <button
                        type="button"
                        className="task-scheduler-start-queue-btn"
                        onClick={onStart}
                    >
                        {tr("viewer.taskScheduler.startQueue", "キュー開始", "Start Queue")}
                    </button>
                )}
                {runStatus === "running" && (
                    <button
                        type="button"
                        className="task-scheduler-pause-btn"
                        onClick={() => void onPause()}
                    >
                        {tr("viewer.taskScheduler.pause", "一時停止", "Pause")}
                    </button>
                )}
                {runStatus === "paused" && (
                    <button
                        type="button"
                        className="task-scheduler-resume-btn"
                        onClick={() => void onResume()}
                    >
                        {tr("viewer.taskScheduler.resume", "再開", "Resume")}
                    </button>
                )}
                {isRunning && (
                    <button
                        type="button"
                        className="task-scheduler-stop-btn"
                        onClick={() => void onStop()}
                    >
                        {tr("viewer.taskScheduler.stop", "停止", "Stop")}
                    </button>
                )}
                <button
                    type="button"
                    className="task-scheduler-new-btn"
                    onClick={onNew}
                >
                    + {tr("viewer.taskScheduler.addTask", "タスク追加", "Add Task")}
                </button>
            </div>

            {items.length === 0 ? (
                <div className="task-scheduler-empty">
                    {tr("viewer.taskScheduler.empty", "タスクがありません", "No tasks")}
                </div>
            ) : (
                items.map((item) => (
                    <div
                        key={item.id}
                        className={`task-scheduler-card ${item.status === "failed" ? "task-scheduler-card-failed" : ""} ${item.status === "running" ? "task-scheduler-card-running" : ""}`}
                    >
                        <div className="task-scheduler-card-header">
                            <span className="task-scheduler-card-status">
                                {STATUS_ICONS[item.status] ?? "?"}
                            </span>
                            <span className="task-scheduler-card-title">{item.title}</span>
                            <div className="task-scheduler-card-actions">
                                {item.status === "pending" && (
                                    <>
                                        <button
                                            type="button"
                                            className="task-scheduler-edit-card-btn"
                                            onClick={() => onEdit(item.id)}
                                        >
                                            {tr("viewer.taskScheduler.edit", "編集", "Edit")}
                                        </button>
                                        <button
                                            type="button"
                                            className="task-scheduler-delete-card-btn"
                                            onClick={() => void onRemove(item.id)}
                                        >
                                            {tr("viewer.taskScheduler.remove", "削除", "Remove")}
                                        </button>
                                    </>
                                )}
                            </div>
                        </div>
                        <div className="task-scheduler-card-meta">
                            <span>{item.target_pane_id}</span>
                            {item.clear_before && (
                                <span className="task-scheduler-card-clear">
                                    Clear{item.clear_command && item.clear_command !== "/new" ? `: ${item.clear_command}` : ""}
                                </span>
                            )}
                            {item.error_message && (
                                <span className="task-scheduler-card-error">{item.error_message}</span>
                            )}
                        </div>
                    </div>
                ))
            )}
        </div>
    );
}
