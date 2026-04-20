import {useState, useCallback, useEffect, useRef, type ChangeEvent} from "react";
import {useI18n} from "../../../../i18n";
import type {PaneSnapshot} from "../../../../types/tmux";
import {isEditableStatus, type QueueItem, type MessageTemplate} from "./useTaskScheduler";

interface TaskSchedulerFormProps {
    availablePanes: PaneSnapshot[];
    messageTemplates: MessageTemplate[];
    editingItem: QueueItem | null;
    initialDraft: {
        title: string;
        message: string;
        targetPaneID: string;
        clearBefore: boolean;
        clearCommand: string;
    } | null;
    onSave: (title: string, message: string, targetPaneID: string, clearBefore: boolean, clearCommand: string) => Promise<void>;
    onBack: () => void;
    onDirtyChange?: (dirty: boolean) => void;
}

export function TaskSchedulerForm({
    availablePanes,
    messageTemplates,
    editingItem,
    initialDraft,
    onSave,
    onBack,
    onDirtyChange,
}: TaskSchedulerFormProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const initialSnapshotRef = useRef({
        title: editingItem?.title ?? initialDraft?.title ?? "",
        message: editingItem?.message ?? initialDraft?.message ?? "",
        targetPaneID: editingItem?.target_pane_id ?? initialDraft?.targetPaneID ?? "",
        clearBefore: editingItem?.clear_before ?? initialDraft?.clearBefore ?? false,
        clearCommand: editingItem?.clear_command ?? initialDraft?.clearCommand ?? "",
    });
    const [title, setTitle] = useState(editingItem?.title ?? initialDraft?.title ?? "");
    const [message, setMessage] = useState(editingItem?.message ?? initialDraft?.message ?? "");
    const [targetPaneID, setTargetPaneID] = useState(editingItem?.target_pane_id ?? initialDraft?.targetPaneID ?? "");
    const [clearBefore, setClearBefore] = useState(editingItem?.clear_before ?? initialDraft?.clearBefore ?? false);
    const [clearCommand, setClearCommand] = useState(editingItem?.clear_command ?? initialDraft?.clearCommand ?? "");
    const [selectedTemplateIndex, setSelectedTemplateIndex] = useState("");
    const [submitting, setSubmitting] = useState(false);
    const isEditingLocked = editingItem !== null && !isEditableStatus(editingItem.status);
    const hasUnsavedChanges = (
        title !== initialSnapshotRef.current.title
        || message !== initialSnapshotRef.current.message
        || targetPaneID !== initialSnapshotRef.current.targetPaneID
        || clearBefore !== initialSnapshotRef.current.clearBefore
        || clearCommand !== initialSnapshotRef.current.clearCommand
    );

    useEffect(() => {
        onDirtyChange?.(hasUnsavedChanges);
    }, [hasUnsavedChanges, onDirtyChange]);

    const handleTemplateSelect = useCallback((e: ChangeEvent<HTMLSelectElement>) => {
        const nextIndex = e.target.value;
        setSelectedTemplateIndex(nextIndex);
        const idx = Number(nextIndex);
        if (Number.isNaN(idx) || idx < 0 || idx >= messageTemplates.length) return;
        const templateMessage = messageTemplates[idx].message;
        setMessage((prev) => (prev.trim() ? prev + "\n" + templateMessage : templateMessage));
        setSelectedTemplateIndex("");
    }, [messageTemplates]);

    const canSubmit = title.trim() !== "" && message.trim() !== "" && targetPaneID !== "" && !submitting && !isEditingLocked;

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
                {"\u2190 "}{tr("viewer.taskScheduler.back", "\u623b\u308b", "Back")}
            </button>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.taskScheduler.titleLabel", "\u30bf\u30a4\u30c8\u30eb", "Title")}
                </label>
                <input
                    className="form-input"
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder={tr("viewer.taskScheduler.titlePlaceholder", "\u30bf\u30b9\u30af\u540d", "Task name")}
                    disabled={isEditingLocked}
                />
            </div>

            <div className="form-group">
                <label className="form-label">
                    {tr("viewer.taskScheduler.message", "\u30e1\u30c3\u30bb\u30fc\u30b8", "Message")}
                </label>
                {messageTemplates.length > 0 && !isEditingLocked && (
                    <select
                        className="task-scheduler-template-select"
                        onChange={handleTemplateSelect}
                        value={selectedTemplateIndex}
                    >
                        <option value="">
                            {tr("viewer.taskScheduler.selectTemplate", "\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u304b\u3089\u8ffd\u52a0...", "Add from template...")}
                        </option>
                        {messageTemplates.map((tmpl, idx) => (
                            <option key={tmpl.name === "" ? `template-${idx}` : `${tmpl.name}:${idx}`} value={String(idx)}>
                                {tmpl.name}
                            </option>
                        ))}
                    </select>
                )}
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
                    {tr("viewer.taskScheduler.targetPane", "\u30bf\u30fc\u30b2\u30c3\u30c8\u30da\u30a4\u30f3", "Target Pane")}
                </label>
                <select
                    className="form-select"
                    value={targetPaneID}
                    onChange={(e) => setTargetPaneID(e.target.value)}
                    disabled={isEditingLocked}
                >
                    <option value="">
                        {tr("viewer.taskScheduler.selectPane", "\u30da\u30a4\u30f3\u3092\u9078\u629e", "Select pane")}
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
                            "\u30bf\u30b9\u30af\u958b\u59cb\u524d\u306b\u30b3\u30f3\u30c6\u30ad\u30b9\u30c8\u30af\u30ea\u30a2",
                            "Clear context before this task")}
                    </span>
                </label>
            </div>

            {clearBefore && (
                <div className="form-group">
                    <label className="form-label">
                        {tr("viewer.taskScheduler.clearCommand", "\u30af\u30ea\u30a2\u30b3\u30de\u30f3\u30c9", "Clear Command")}
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
                            "\u672a\u5165\u529b\u6642\u306f /new \u304c\u4f7f\u308f\u308c\u307e\u3059\u3002\u9001\u4fe1\u306b\u5931\u6557\u3057\u305f\u5834\u5408\u306f\u305d\u306e\u30bf\u30b9\u30af\u306f failed \u306b\u306a\u308a\u307e\u3059",
                            "Defaults to /new if empty. If the clear command fails, that task is marked as failed.")}
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
                    ? tr("viewer.taskScheduler.update", "\u66f4\u65b0", "Update")
                    : tr("viewer.taskScheduler.add", "\u8ffd\u52a0", "Add")
                }
            </button>
        </div>
    );
}
