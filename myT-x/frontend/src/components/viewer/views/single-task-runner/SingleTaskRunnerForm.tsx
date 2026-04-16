import {useCallback, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {PaneSnapshot} from "../../../../types/tmux";
import {isEditableStatus, type QueueItem} from "./useSingleTaskRunner";

interface SingleTaskRunnerFormProps {
    availablePanes: PaneSnapshot[];
    editingItem: QueueItem | null;
    onSave: (title: string, message: string, targetPaneID: string, clearBefore: boolean, clearCommand: string) => Promise<boolean>;
    onBack: () => void;
}

export function SingleTaskRunnerForm({
                                         availablePanes,
                                         editingItem,
                                         onSave,
                                         onBack,
                                     }: SingleTaskRunnerFormProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const [title, setTitle] = useState(editingItem?.title ?? "");
    const [message, setMessage] = useState(editingItem?.message ?? "");
    const [targetPaneID, setTargetPaneID] = useState(editingItem?.target_pane_id ?? "");
    const [clearBefore, setClearBefore] = useState(editingItem?.clear_before ?? false);
    const [clearCommand, setClearCommand] = useState(editingItem?.clear_command ?? "");
    const [submitting, setSubmitting] = useState(false);
    const isEditingLocked = editingItem !== null && !isEditableStatus(editingItem.status);

    const canSubmit = title.trim() !== "" && message.trim() !== "" && targetPaneID !== "" && !submitting && !isEditingLocked;

    const handleSubmit = useCallback(async () => {
        if (!canSubmit) return;
        setSubmitting(true);
        try {
            await onSave(title.trim(), message.trim(), targetPaneID, clearBefore, clearCommand.trim());
        } finally {
            setSubmitting(false);
        }
    }, [canSubmit, clearBefore, clearCommand, message, onSave, targetPaneID, title]);

    return (
        <div className="single-task-runner-form">
            <button type="button" className="single-task-runner-back-btn" onClick={onBack}>
                {"← "}{tr("viewer.singleTaskRunner.back", "戻る", "Back")}
            </button>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.singleTaskRunner.title", "タイトル", "Title")}
                </label>
                <input
                    className="form-input"
                    type="text"
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    placeholder={tr("viewer.singleTaskRunner.titlePlaceholder", "タスク名", "Task name")}
                    disabled={isEditingLocked}
                />
            </div>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.singleTaskRunner.message", "メッセージ", "Message")}
                </label>
                <textarea
                    className="single-task-runner-textarea"
                    value={message}
                    onChange={(event) => setMessage(event.target.value)}
                    placeholder={tr("viewer.singleTaskRunner.messagePlaceholder", "AIに送信する指示", "Instructions to send to AI")}
                    disabled={isEditingLocked}
                />
            </div>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.singleTaskRunner.targetPane", "ターゲットペイン", "Target Pane")}
                </label>
                <select
                    className="form-select"
                    value={targetPaneID}
                    onChange={(event) => setTargetPaneID(event.target.value)}
                    disabled={isEditingLocked}
                >
                    <option value="">
                        {tr("viewer.singleTaskRunner.selectPane", "ペインを選択", "Select pane")}
                    </option>
                    {availablePanes.map((pane) => (
                        <option key={pane.id} value={pane.id}>
                            {pane.id} {pane.title ? `(${pane.title})` : ""}
                        </option>
                    ))}
                </select>
            </div>

            <div className="form-group">
                <label className="single-task-runner-checkbox-label">
                    <input
                        type="checkbox"
                        checked={clearBefore}
                        onChange={(event) => setClearBefore(event.target.checked)}
                        disabled={isEditingLocked}
                    />
                    <span>
                        {tr(
                            "viewer.singleTaskRunner.clearBefore",
                            "タスク開始前に初期化コマンドを送信",
                            "Send a clear command before this task",
                        )}
                    </span>
                </label>
            </div>

            {clearBefore && (
                <div className="form-group">
                    <label className="form-label">
                        {tr("viewer.singleTaskRunner.clearCommand", "クリアコマンド", "Clear Command")}
                    </label>
                    <input
                        className="form-input"
                        type="text"
                        value={clearCommand}
                        onChange={(event) => setClearCommand(event.target.value)}
                        placeholder="/new"
                        disabled={isEditingLocked}
                    />
                    <span className="single-task-runner-form-hint">
                        {tr(
                            "viewer.singleTaskRunner.clearCommandHint",
                            "未入力時は /new が使われます",
                            "Defaults to /new if empty",
                        )}
                    </span>
                </div>
            )}

            <button
                type="button"
                className="single-task-runner-submit-btn"
                onClick={() => void handleSubmit()}
                disabled={!canSubmit}
            >
                {editingItem
                    ? tr("viewer.singleTaskRunner.update", "更新", "Update")
                    : tr("viewer.singleTaskRunner.add", "追加", "Add")}
            </button>
        </div>
    );
}
