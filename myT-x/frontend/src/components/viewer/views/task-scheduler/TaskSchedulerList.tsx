import {useI18n} from "../../../../i18n";
import {isEditableStatus, RUNNING_ITEM_STATUS, type QueueStatus} from "./useTaskScheduler";

interface TaskSchedulerListProps {
    status: QueueStatus | null;
    onNew: () => void | Promise<void>;
    onEdit: (id: string) => void | Promise<void>;
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

function preExecPhaseLabel(
    progress: string | undefined,
    tr: (key: string, jaText: string, enText: string) => string,
): string {
    switch (progress) {
        case "resetting":
            return tr("viewer.taskScheduler.preExecResetting", "セッションリセット中", "Resetting sessions");
        case "waiting_reset":
            return tr("viewer.taskScheduler.preExecWaitingReset", "リセット待機中", "Waiting for reset");
        case "sending_reminders":
            return tr("viewer.taskScheduler.preExecSendingReminders", "役割リマインド送信中", "Sending role reminders");
        case "waiting_idle":
            return tr("viewer.taskScheduler.preExecWaitingIdle", "アイドル待機中", "Waiting for agents");
        case "idle_timeout":
            return tr(
                "viewer.taskScheduler.preExecIdleTimeout",
                "アイドル待機がタイムアウトしたため、そのまま続行しています",
                "Idle wait timed out, continuing queue execution",
            );
        default:
            return tr("viewer.taskScheduler.preparing", "前準備中...", "Preparing...");
    }
}

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
    const preExecTimedOut = status?.pre_exec_progress === "idle_timeout";
    const hasPendingItems = items.some((i) => i.status === "pending");

    return (
        <div className="task-scheduler-list">
            {(runStatus === "preparing" || preExecTimedOut) && (
                <div
                    className={`task-scheduler-preexec-status ${preExecTimedOut ? "task-scheduler-preexec-status-timeout" : ""}`}>
                    <span className="task-scheduler-preexec-badge">
                        {preExecTimedOut
                            ? tr("viewer.taskScheduler.preExecNotice", "通知", "Notice")
                            : tr("viewer.taskScheduler.preparing", "前準備中...", "Preparing...")}
                    </span>
                    <span className="task-scheduler-preexec-label">
                        {preExecPhaseLabel(status?.pre_exec_progress, tr)}
                    </span>
                </div>
            )}
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
                    onClick={() => void onNew()}
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
                        className={`task-scheduler-card ${item.status === "failed" ? "task-scheduler-card-failed" : ""} ${item.status === RUNNING_ITEM_STATUS ? "task-scheduler-card-running" : ""}`}
                    >
                        <div className="task-scheduler-card-header">
                            <span className="task-scheduler-card-status">
                                {STATUS_ICONS[item.status] ?? "?"}
                            </span>
                            <span className="task-scheduler-card-title">{item.title}</span>
                            <div className="task-scheduler-card-actions">
                                {isEditableStatus(item.status) && (
                                    <>
                                        <button
                                            type="button"
                                            className="task-scheduler-edit-card-btn"
                                            onClick={() => void onEdit(item.id)}
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
