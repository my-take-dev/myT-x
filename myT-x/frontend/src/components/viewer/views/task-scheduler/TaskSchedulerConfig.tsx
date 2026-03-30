import {useState, useCallback, useMemo} from "react";
import {useI18n} from "../../../../i18n";
import type {QueueConfig, QueueItem} from "./useTaskScheduler";
import {
    preExecFieldBounds,
    readNumberInput,
    resolveInitialPreExecIdleTimeout,
    resolveInitialPreExecResetDelay,
    resolveInitialPreExecTargetMode,
} from "./taskSchedulerConfigForm";

interface TaskSchedulerConfigProps {
    items: QueueItem[];
    initialConfig?: QueueConfig | null;
    onStart: (config: QueueConfig, items: QueueItem[]) => Promise<void>;
    onBack: () => void;
}

export function TaskSchedulerConfig({
                                        items,
                                        initialConfig,
                                        onStart,
                                        onBack,
                                    }: TaskSchedulerConfigProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const [submitting, setSubmitting] = useState(false);
    const [preExecEnabled, setPreExecEnabled] = useState(initialConfig?.pre_exec_enabled ?? false);
    const [preExecTargetMode, setPreExecTargetMode] = useState(
        resolveInitialPreExecTargetMode(initialConfig?.pre_exec_target_mode),
    );
    const [preExecResetDelay, setPreExecResetDelay] = useState(
        resolveInitialPreExecResetDelay(initialConfig?.pre_exec_reset_delay_s),
    );
    const [preExecIdleTimeout, setPreExecIdleTimeout] = useState(
        resolveInitialPreExecIdleTimeout(initialConfig?.pre_exec_idle_timeout_s),
    );

    const pendingItems = useMemo(
        () => items.filter((i) => i.status === "pending"),
        [items],
    );
    const canStart = pendingItems.length > 0 && !submitting;

    const handleStart = useCallback(async () => {
        if (!canStart) return;
        setSubmitting(true);
        try {
            await onStart({
                pre_exec_enabled: preExecEnabled,
                pre_exec_target_mode: preExecTargetMode,
                pre_exec_reset_delay_s: resolveInitialPreExecResetDelay(preExecResetDelay),
                pre_exec_idle_timeout_s: resolveInitialPreExecIdleTimeout(preExecIdleTimeout),
            }, pendingItems);
        } finally {
            setSubmitting(false);
        }
    }, [
        canStart,
        onStart,
        pendingItems,
        preExecEnabled,
        preExecIdleTimeout,
        preExecResetDelay,
        preExecTargetMode,
    ]);

    return (
        <div className="task-scheduler-form">
            <button type="button" className="task-scheduler-back-btn" onClick={onBack}>
                ← {tr("viewer.taskScheduler.back", "戻る", "Back")}
            </button>

            <h3 className="task-scheduler-config-title">
                {tr("viewer.taskScheduler.queueConfig", "キュー設定", "Queue Settings")}
            </h3>

            <div className="task-scheduler-config-summary">
                {tr("viewer.taskScheduler.pendingCount",
                    `${pendingItems.length} 件のタスクを実行`,
                    `${pendingItems.length} task(s) to execute`)}
            </div>

            <label className="task-scheduler-checkbox-label">
                <input
                    type="checkbox"
                    checked={preExecEnabled}
                    onChange={(event) => setPreExecEnabled(event.target.checked)}
                />
                <span>
                    {tr(
                        "viewer.taskScheduler.preExecEnabled",
                        "セッションリセット+役割リマインダーを実行する",
                        "Run session reset and role reminders",
                    )}
                </span>
            </label>

            {preExecEnabled && (
                <div className="task-scheduler-config-section">
                    <div className="task-scheduler-config-group">
                        <div className="task-scheduler-config-label">
                            {tr(
                                "viewer.taskScheduler.preExecTarget",
                                "対象ペイン",
                                "Target panes",
                            )}
                        </div>
                        <label className="task-scheduler-radio-label">
                            <input
                                type="radio"
                                name="task-scheduler-preexec-target"
                                checked={preExecTargetMode === "task_panes"}
                                onChange={() => setPreExecTargetMode("task_panes")}
                            />
                            <span>
                                {tr(
                                    "viewer.taskScheduler.preExecTaskPanes",
                                    "タスクで使用するペインのみ",
                                    "Only panes used by tasks",
                                )}
                            </span>
                        </label>
                        <label className="task-scheduler-radio-label">
                            <input
                                type="radio"
                                name="task-scheduler-preexec-target"
                                checked={preExecTargetMode === "all_panes"}
                                onChange={() => setPreExecTargetMode("all_panes")}
                            />
                            <span>
                                {tr(
                                    "viewer.taskScheduler.preExecAllPanes",
                                    "セッション内の全ペイン",
                                    "All panes in the session",
                                )}
                            </span>
                        </label>
                    </div>

                    <div className="task-scheduler-config-group">
                        <label className="task-scheduler-config-field">
                            <span className="task-scheduler-config-label">
                                {tr(
                                    "viewer.taskScheduler.preExecResetDelay",
                                    "/new 後の待機時間 (秒)",
                                    "Wait after /new (seconds)",
                                )}
                            </span>
                            <input
                                type="number"
                                step={1}
                                min={preExecFieldBounds.resetDelay.min}
                                max={preExecFieldBounds.resetDelay.max}
                                value={preExecResetDelay}
                                onChange={(event) => {
                                    setPreExecResetDelay((previousValue) =>
                                        readNumberInput(
                                            event.target.value,
                                            previousValue,
                                            preExecFieldBounds.resetDelay.min,
                                            preExecFieldBounds.resetDelay.max,
                                        ),
                                    );
                                }}
                                className="task-scheduler-config-input"
                            />
                        </label>

                        <label className="task-scheduler-config-field">
                            <span className="task-scheduler-config-label">
                                {tr(
                                    "viewer.taskScheduler.preExecIdleTimeout",
                                    "アイドル待機タイムアウト (秒)",
                                    "Idle wait timeout (seconds)",
                                )}
                            </span>
                            <input
                                type="number"
                                step={1}
                                min={preExecFieldBounds.idleTimeout.min}
                                max={preExecFieldBounds.idleTimeout.max}
                                value={preExecIdleTimeout}
                                onChange={(event) => {
                                    setPreExecIdleTimeout((previousValue) =>
                                        readNumberInput(
                                            event.target.value,
                                            previousValue,
                                            preExecFieldBounds.idleTimeout.min,
                                            preExecFieldBounds.idleTimeout.max,
                                        ),
                                    );
                                }}
                                className="task-scheduler-config-input"
                            />
                        </label>
                    </div>
                </div>
            )}

            <button
                type="button"
                className="task-scheduler-submit-btn"
                onClick={() => void handleStart()}
                disabled={!canStart}
            >
                {tr("viewer.taskScheduler.startQueue", "キュー開始", "Start Queue")}
            </button>
        </div>
    );
}
