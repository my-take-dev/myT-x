import {useState, useCallback} from "react";
import {useI18n} from "../../../../i18n";
import type {PaneSnapshot} from "../../../../types/tmux";
import {isEditableStatus, type QueueItem} from "./useTaskScheduler";

interface TaskSchedulerFormProps {
    availablePanes: PaneSnapshot[];
    editingItem: QueueItem | null;
    onSave: (title: string, message: string, targetPaneID: string, clearBefore: boolean, clearCommand: string) => Promise<void>;
    onBack: () => void;
}

export function TaskSchedulerForm({
    availablePanes,
    editingItem,
    onSave,
    onBack,
}: TaskSchedulerFormProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const [title, setTitle] = useState(editingItem?.title ?? "");
    const [message, setMessage] = useState(editingItem?.message ?? "");
    const [targetPaneID, setTargetPaneID] = useState(editingItem?.target_pane_id ?? "");
    const [clearBefore, setClearBefore] = useState(editingItem?.clear_before ?? false);
    const [clearCommand, setClearCommand] = useState(editingItem?.clear_command ?? "");
    const [submitting, setSubmitting] = useState(false);
    const isEditingLocked = !isEditableStatus(editingItem?.status);

    const canSubmit = title.trim() !== "" && message.trim() !== "" && targetPaneID !== "" && !submitting;

    const handleSubmit = useCallback(async () => {
        if (!canSubmit) return;
        setSubmitting(true);
        try {
            await onSave(title.trim(), message.trim(), targetPaneID, clearBefore, clearCommand.trim());
        } finally {
            setSubmitting(false);
        }
    }, [canSubmit, onSave, title, message, targetPaneID, clearBefore, clearCommand]);

    return (
        <div className="task-scheduler-form">
            <button type="button" className="task-scheduler-back-btn" onClick={onBack}>
                ← {tr("viewer.taskScheduler.back", "戻る", "Back")}
            </button>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.taskScheduler.title", "タイトル", "Title")}
                </label>
                <input
                    className="form-input"
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder={tr("viewer.taskScheduler.titlePlaceholder", "タスク名", "Task name")}
                    disabled={isEditingLocked}
                />
            </div>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.taskScheduler.message", "メッセージ", "Message")}
                </label>
                <textarea
                    className="task-scheduler-textarea"
                    value={message}
                    onChange={(e) => setMessage(e.target.value)}
                    placeholder={tr("viewer.taskScheduler.messagePlaceholder", "AIに送信する指示", "Instructions to send to AI")}
                    disabled={isEditingLocked}
                />
            </div>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.taskScheduler.targetPane", "ターゲットペイン", "Target Pane")}
                </label>
                <select
                    className="form-select"
                    value={targetPaneID}
                    onChange={(e) => setTargetPaneID(e.target.value)}
                    disabled={isEditingLocked}
                >
                    <option value="">
                        {tr("viewer.taskScheduler.selectPane", "ペインを選択", "Select pane")}
                    </option>
                    {availablePanes.map((pane) => (
                        <option key={pane.id} value={pane.id}>
                            {pane.id} {pane.title ? `(${pane.title})` : ""}
                        </option>
                    ))}
                </select>
            </div>

            <div className="form-group">
                <label className="task-scheduler-checkbox-label">
                    <input
                        type="checkbox"
                        checked={clearBefore}
                        onChange={(e) => setClearBefore(e.target.checked)}
                        disabled={isEditingLocked}
                    />
                    <span>
                        {tr("viewer.taskScheduler.clearBefore",
                            "タスク開始前にコンテキストクリア",
                            "Clear context before this task")}
                    </span>
                </label>
            </div>

            {clearBefore && (
                <div className="form-group">
                    <label className="form-label">
                        {tr("viewer.taskScheduler.clearCommand", "クリアコマンド", "Clear Command")}
                    </label>
                    <input
                        className="form-input"
                        type="text"
                        value={clearCommand}
                        onChange={(e) => setClearCommand(e.target.value)}
                        placeholder="/new"
                        disabled={isEditingLocked}
                    />
                    <span className="task-scheduler-config-hint">
                        {tr("viewer.taskScheduler.clearCommandHint",
                            "未入力時は /new がデフォルト",
                            "Defaults to /new if empty")}
                    </span>
                </div>
            )}

            <button
                type="button"
                className="task-scheduler-submit-btn"
                onClick={() => void handleSubmit()}
                disabled={!canSubmit}
            >
                {editingItem
                    ? tr("viewer.taskScheduler.update", "更新", "Update")
                    : tr("viewer.taskScheduler.add", "追加", "Add")
                }
            </button>
        </div>
    );
}
