import {useCallback, useEffect, useMemo, useState} from "react";
import {api} from "../../../../api";
import {useI18n} from "../../../../i18n";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {isEditableStatus, type QueueStatus} from "./useSingleTaskRunner";

interface SingleTaskRunnerListProps {
    readonly defaultClearDelay: number | null;
    status: QueueStatus | null;
    onNew: () => void;
    onEdit: (id: string) => void;
    onRemove: (id: string) => Promise<void>;
    onStart: () => Promise<boolean>;
    onStop: () => Promise<void>;
    onSetClearDelay: (delaySec: number) => Promise<boolean>;
    onError: (message: string | null) => void;
}

const STATUS_ICONS: Record<string, string> = {
    pending: "⏳",
    sending: "📤",
    active: "▶",
    done: "✅",
    failed: "❌",
    cancelled: "⛔",
};

// Matches the current backend defaults and is only used if validation rules
// cannot be fetched during startup.
const fallbackClearDelayBounds = {
    min: 0,
    max: 300,
};

export function SingleTaskRunnerList({
    defaultClearDelay,
    status,
    onNew,
    onEdit,
    onRemove,
    onStart,
    onStop,
    onSetClearDelay,
    onError,
}: SingleTaskRunnerListProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const items = status?.items ?? [];
    const runStatus = status?.run_status ?? "idle";
    const isRunning = runStatus === "running";
    const hasPendingItems = useMemo(
        () => items.some((item) => item.status === "pending"),
        [items],
    );

    const currentClearDelay = status?.clear_delay_sec ?? defaultClearDelay;
    const [clearDelayInput, setClearDelayInput] = useState(currentClearDelay === null ? "" : String(currentClearDelay));
    const [clearDelayBounds, setClearDelayBounds] = useState(fallbackClearDelayBounds);

    useEffect(() => {
        setClearDelayInput(currentClearDelay === null ? "" : String(currentClearDelay));
    }, [currentClearDelay]);

    useEffect(() => {
        let cancelled = false;
        void api.GetValidationRules()
            .then((rules) => {
                if (cancelled) {
                    return;
                }
                setClearDelayBounds({
                    min: rules.min_single_task_runner_clear_delay,
                    max: rules.max_single_task_runner_clear_delay,
                });
            })
            .catch((err: unknown) => {
                console.warn("[single-task-runner] failed to load clear delay bounds, using fallback", err);
            });
        return () => {
            cancelled = true;
        };
    }, []);

    const commitClearDelay = useCallback(async () => {
        const parsed = Number.parseInt(clearDelayInput, 10);
        if (Number.isNaN(parsed) || parsed < clearDelayBounds.min || parsed > clearDelayBounds.max) {
            setClearDelayInput(currentClearDelay === null ? "" : String(currentClearDelay));
            return;
        }
        try {
            const ok = await onSetClearDelay(parsed);
            if (!ok) {
                setClearDelayInput(currentClearDelay === null ? "" : String(currentClearDelay));
            }
        } catch (err: unknown) {
            console.warn("[single-task-runner] failed to update clear delay", err);
            onError(toErrorMessage(err, "Failed to update clear delay"));
            setClearDelayInput(currentClearDelay === null ? "" : String(currentClearDelay));
        }
    }, [clearDelayBounds.max, clearDelayBounds.min, clearDelayInput, currentClearDelay, onError, onSetClearDelay]);

    return (
        <div className="single-task-runner-list">
            <div className="single-task-runner-delay-panel">
                <label className="form-label" htmlFor="single-task-runner-clear-delay">
                    {tr("viewer.singleTaskRunner.clearDelay", "初期化待ち時間", "Clear Delay")}
                </label>
                <div className="single-task-runner-delay-row">
                    <input
                        id="single-task-runner-clear-delay"
                        className="form-input single-task-runner-delay-input"
                        type="text"
                        inputMode="numeric"
                        value={clearDelayInput}
                        onChange={(event) => {
                            const next = event.target.value.replace(/\D/g, "");
                            setClearDelayInput(next);
                        }}
                        onBlur={() => void commitClearDelay()}
                    />
                    <span className="single-task-runner-delay-unit">
                        {tr("viewer.singleTaskRunner.seconds", "秒", "sec")}
                    </span>
                </div>
                <span className="single-task-runner-delay-hint">
                    {tr(
                        "viewer.singleTaskRunner.clearDelayHint",
                        "clear_before のタスクで /new 実行後に待機する秒数",
                        "Seconds to wait after clear_before commands before sending the task",
                    )}
                </span>
            </div>

            <div className="single-task-runner-toolbar">
                {!isRunning && hasPendingItems && (
                    <button
                        type="button"
                        className="single-task-runner-start-btn"
                        onClick={() => void onStart()}
                    >
                        {tr("viewer.singleTaskRunner.startQueue", "キュー開始", "Start Queue")}
                    </button>
                )}
                {isRunning && (
                    <button
                        type="button"
                        className="single-task-runner-stop-btn"
                        onClick={() => void onStop()}
                    >
                        {tr("viewer.singleTaskRunner.stop", "停止", "Stop")}
                    </button>
                )}
                <button
                    type="button"
                    className="single-task-runner-new-btn"
                    onClick={onNew}
                >
                    + {tr("viewer.singleTaskRunner.addTask", "タスク追加", "Add Task")}
                </button>
            </div>

            {items.length === 0 ? (
                <div className="single-task-runner-empty">
                    {tr("viewer.singleTaskRunner.empty", "タスクがありません", "No tasks")}
                    <p className="single-task-runner-empty-desc">
                        {tr(
                            "viewer.singleTaskRunner.emptyDesc",
                            "Single Task Runner MCPのタスクランナーです。MCPツールからタスクを追加できます。",
                            "Task runner for Single Task Runner MCP. Add tasks via MCP tools.",
                        )}
                    </p>
                </div>
            ) : (
                items.map((item) => (
                    <div
                        key={item.id}
                        className={[
                            "single-task-runner-card",
                            item.status === "failed" ? "single-task-runner-card-failed" : "",
                            item.status === "active" ? "single-task-runner-card-active" : "",
                        ].filter(Boolean).join(" ")}
                    >
                        <div className="single-task-runner-card-header">
                            <span className="single-task-runner-card-status">
                                {STATUS_ICONS[item.status] ?? "?"}
                            </span>
                            <span className="single-task-runner-card-title">{item.title}</span>
                            {isEditableStatus(item.status) && (
                                <div className="single-task-runner-card-actions">
                                    <button
                                        type="button"
                                        className="single-task-runner-edit-btn"
                                        onClick={() => onEdit(item.id)}
                                    >
                                        {tr("viewer.singleTaskRunner.edit", "編集", "Edit")}
                                    </button>
                                    <button
                                        type="button"
                                        className="single-task-runner-delete-btn"
                                        onClick={() => void onRemove(item.id)}
                                    >
                                        {tr("viewer.singleTaskRunner.remove", "削除", "Remove")}
                                    </button>
                                </div>
                            )}
                        </div>
                        <div className="single-task-runner-card-meta">
                            <span>{item.target_pane_id}</span>
                            {item.clear_before && (
                                <span className="single-task-runner-card-clear">
                                    Clear{item.clear_command && item.clear_command !== "/new" ? `: ${item.clear_command}` : ""}
                                </span>
                            )}
                            {item.error_message && (
                                <span className="single-task-runner-card-error">{item.error_message}</span>
                            )}
                        </div>
                        {item.result_message && (
                            <div className="single-task-runner-card-result">{item.result_message}</div>
                        )}
                    </div>
                ))
            )}
        </div>
    );
}
