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
    onSettings: () => void;
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
            return tr("viewer.taskScheduler.preExecResetting", "\u30bb\u30c3\u30b7\u30e7\u30f3\u30ea\u30bb\u30c3\u30c8\u4e2d", "Resetting sessions");
        case "waiting_reset":
            return tr("viewer.taskScheduler.preExecWaitingReset", "\u30ea\u30bb\u30c3\u30c8\u5f85\u6a5f\u4e2d", "Waiting for reset");
        case "sending_reminders":
            return tr("viewer.taskScheduler.preExecSendingReminders", "\u5f79\u5272\u30ea\u30de\u30a4\u30f3\u30c9\u9001\u4fe1\u4e2d", "Sending role reminders");
        case "waiting_idle":
            return tr("viewer.taskScheduler.preExecWaitingIdle", "\u30a2\u30a4\u30c9\u30eb\u5f85\u6a5f\u4e2d", "Waiting for agents");
        case "idle_timeout":
            return tr(
                "viewer.taskScheduler.preExecIdleTimeout",
                "\u30a2\u30a4\u30c9\u30eb\u5f85\u6a5f\u304c\u30bf\u30a4\u30e0\u30a2\u30a6\u30c8\u3057\u305f\u305f\u3081\u3001\u305d\u306e\u307e\u307e\u7d9a\u884c\u3057\u3066\u3044\u307e\u3059",
                "Idle wait timed out, continuing queue execution",
            );
        default:
            return tr("viewer.taskScheduler.preparing", "\u524d\u6e96\u5099\u4e2d...", "Preparing...");
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
                                      onSettings,
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
                            ? tr("viewer.taskScheduler.preExecNotice", "\u901a\u77e5", "Notice")
                            : tr("viewer.taskScheduler.preparing", "\u524d\u6e96\u5099\u4e2d...", "Preparing...")}
                    </span>
                    <span className="task-scheduler-preexec-label">
                        {preExecPhaseLabel(status?.pre_exec_progress, tr)}
                    </span>
                </div>
            )}
            <div className="task-scheduler-toolbar">
                <button
                    type="button"
                    className="task-scheduler-settings-btn"
                    onClick={onSettings}
                >
                    {tr("viewer.taskScheduler.settings", "\u8a2d\u5b9a", "Settings")}
                </button>
                {!isRunning && hasPendingItems && (
                    <button
                        type="button"
                        className="task-scheduler-start-queue-btn"
                        onClick={onStart}
                    >
                        {tr("viewer.taskScheduler.startQueue", "\u30ad\u30e5\u30fc\u958b\u59cb", "Start Queue")}
                    </button>
                )}
                {runStatus === "running" && (
                    <button
                        type="button"
                        className="task-scheduler-pause-btn"
                        onClick={() => void onPause()}
                    >
                        {tr("viewer.taskScheduler.pause", "\u4e00\u6642\u505c\u6b62", "Pause")}
                    </button>
                )}
                {runStatus === "paused" && (
                    <button
                        type="button"
                        className="task-scheduler-resume-btn"
                        onClick={() => void onResume()}
                    >
                        {tr("viewer.taskScheduler.resume", "\u518d\u958b", "Resume")}
                    </button>
                )}
                {isRunning && (
                    <button
                        type="button"
                        className="task-scheduler-stop-btn"
                        onClick={() => void onStop()}
                    >
                        {tr("viewer.taskScheduler.stop", "\u505c\u6b62", "Stop")}
                    </button>
                )}
                <button
                    type="button"
                    className="task-scheduler-new-btn"
                    onClick={() => void onNew()}
                >
                    + {tr("viewer.taskScheduler.addTask", "\u30bf\u30b9\u30af\u8ffd\u52a0", "Add Task")}
                </button>
            </div>

            {items.length === 0 ? (
                <div className="task-scheduler-empty">
                    {tr("viewer.taskScheduler.empty", "\u30bf\u30b9\u30af\u304c\u3042\u308a\u307e\u305b\u3093", "No tasks")}
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
                                            {tr("viewer.taskScheduler.edit", "\u7de8\u96c6", "Edit")}
                                        </button>
                                        <button
                                            type="button"
                                            className="task-scheduler-delete-card-btn"
                                            onClick={() => void onRemove(item.id)}
                                        >
                                            {tr("viewer.taskScheduler.remove", "\u524a\u9664", "Remove")}
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
