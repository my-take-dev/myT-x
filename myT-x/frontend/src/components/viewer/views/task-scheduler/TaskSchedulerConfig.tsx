import {useState, useCallback} from "react";
import {useI18n} from "../../../../i18n";
import type {QueueConfig, QueueItem} from "./useTaskScheduler";

interface TaskSchedulerConfigProps {
    items: QueueItem[];
    onStart: (config: QueueConfig, items: QueueItem[]) => Promise<void>;
    onBack: () => void;
}

export function TaskSchedulerConfig({
    items,
    onStart,
    onBack,
}: TaskSchedulerConfigProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const [submitting, setSubmitting] = useState(false);

    const pendingItems = items.filter((i) => i.status === "pending");
    const canStart = pendingItems.length > 0 && !submitting;

    const handleStart = useCallback(async () => {
        if (!canStart) return;
        setSubmitting(true);
        try {
            await onStart({}, pendingItems);
        } finally {
            setSubmitting(false);
        }
    }, [canStart, pendingItems, onStart]);

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
