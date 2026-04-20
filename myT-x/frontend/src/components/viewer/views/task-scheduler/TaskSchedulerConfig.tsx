import {useState, useCallback, useMemo, useEffect, useRef} from "react";
import {useI18n} from "../../../../i18n";
import type {QueueConfig, QueueItem, TaskSchedulerSettings} from "./useTaskScheduler";
import {
    resolveInitialPreExecIdleTimeout,
    resolveInitialPreExecResetDelay,
    resolveInitialPreExecTargetMode,
} from "./taskSchedulerConfigForm";
import {PRE_EXEC_TARGET_MODE_ALL_PANES} from "./preExecTargetModes";

interface TaskSchedulerConfigProps {
    items: QueueItem[];
    initialConfig?: QueueConfig | null;
    savedSettings: TaskSchedulerSettings | null;
    onStart: (config: QueueConfig, items: QueueItem[]) => Promise<void>;
    onBack: () => void;
    onOpenSettings: () => void;
    onDirtyChange?: (dirty: boolean) => void;
}

export function TaskSchedulerConfig({
                                        items,
                                        initialConfig,
                                        savedSettings,
                                        onStart,
                                        onBack,
                                        onOpenSettings,
                                        onDirtyChange,
                                    }: TaskSchedulerConfigProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const initialPreExecEnabledRef = useRef(initialConfig?.pre_exec_enabled ?? false);
    const [submitting, setSubmitting] = useState(false);
    const [preExecEnabled, setPreExecEnabled] = useState(initialConfig?.pre_exec_enabled ?? false);
    const hasUnsavedChanges = preExecEnabled !== initialPreExecEnabledRef.current;

    const pendingItems = useMemo(
        () => items.filter((i) => i.status === "pending"),
        [items],
    );
    const settingsLoaded = savedSettings !== null;
    const canStart = pendingItems.length > 0 && !submitting && (!preExecEnabled || settingsLoaded);

    useEffect(() => {
        onDirtyChange?.(hasUnsavedChanges);
    }, [hasUnsavedChanges, onDirtyChange]);

    // Resolve the effective timing values returned by the backend settings API.
    const resetDelay = resolveInitialPreExecResetDelay(savedSettings?.pre_exec_reset_delay_s);
    const idleTimeout = resolveInitialPreExecIdleTimeout(savedSettings?.pre_exec_idle_timeout_s);
    const targetMode = resolveInitialPreExecTargetMode(savedSettings?.pre_exec_target_mode);

    const handleStart = useCallback(async () => {
        if (!canStart) return;
        setSubmitting(true);
        try {
            await onStart({
                pre_exec_enabled: preExecEnabled,
                pre_exec_target_mode: targetMode,
                pre_exec_reset_delay_s: resetDelay,
                pre_exec_idle_timeout_s: idleTimeout,
            }, pendingItems);
        } finally {
            setSubmitting(false);
        }
    }, [canStart, onStart, pendingItems, preExecEnabled, targetMode, resetDelay, idleTimeout]);

    return (
        <div className="task-scheduler-form">
            <button type="button" className="task-scheduler-back-btn" onClick={onBack}>
                {"\u2190 "}{tr("viewer.taskScheduler.back", "\u623b\u308b", "Back")}
            </button>

            <h3 className="task-scheduler-config-title">
                {tr("viewer.taskScheduler.queueConfig", "\u30ad\u30e5\u30fc\u8a2d\u5b9a", "Queue Settings")}
            </h3>

            <div className="task-scheduler-config-summary">
                {tr("viewer.taskScheduler.pendingCount",
                    `${pendingItems.length} \u4ef6\u306e\u30bf\u30b9\u30af\u3092\u5b9f\u884c`,
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
                        "\u30bb\u30c3\u30b7\u30e7\u30f3\u30ea\u30bb\u30c3\u30c8+\u5f79\u5272\u30ea\u30de\u30a4\u30f3\u30c0\u30fc\u3092\u5b9f\u884c\u3059\u308b",
                        "Run session reset and role reminders",
                    )}
                </span>
            </label>

            {preExecEnabled && (
                <div className="task-scheduler-config-section">
                    <div className="task-scheduler-config-hint-row">
                        <div className="task-scheduler-config-hint">
                            {settingsLoaded
                                ? tr(
                                    "viewer.taskScheduler.preExecSettingsHint",
                                    `\u5f85\u6a5f\u6642\u9593: ${resetDelay}\u79d2 / \u30bf\u30a4\u30e0\u30a2\u30a6\u30c8: ${idleTimeout}\u79d2 / \u5bfe\u8c61: ${targetMode === PRE_EXEC_TARGET_MODE_ALL_PANES ? "\u5168\u30da\u30a4\u30f3" : "\u30bf\u30b9\u30af\u30da\u30a4\u30f3\u306e\u307f"}`,
                                    `Delay: ${resetDelay}s / Timeout: ${idleTimeout}s / Target: ${targetMode === PRE_EXEC_TARGET_MODE_ALL_PANES ? "all panes" : "task panes only"}`,
                                )
                                : tr(
                                    "viewer.taskScheduler.preExecSettingsLoading",
                                    "\u30b9\u30b1\u30b8\u30e5\u30fc\u30e9\u8a2d\u5b9a\u3092\u8aad\u307f\u8fbc\u307f\u4e2d...",
                                    "Loading scheduler settings...",
                                )}
                        </div>
                        <button
                            type="button"
                            className="task-scheduler-config-link"
                            onClick={onOpenSettings}
                        >
                            {tr("viewer.taskScheduler.editInSettings", "\u8a2d\u5b9a\u753b\u9762\u3067\u7de8\u96c6", "Edit in Settings")}
                        </button>
                    </div>
                </div>
            )}

            <button
                type="button"
                className="task-scheduler-submit-btn"
                onClick={() => void handleStart()}
                disabled={!canStart}
            >
                {tr("viewer.taskScheduler.startQueue", "\u30ad\u30e5\u30fc\u958b\u59cb", "Start Queue")}
            </button>
        </div>
    );
}
