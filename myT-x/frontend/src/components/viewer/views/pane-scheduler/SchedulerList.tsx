import {useCallback, useState} from "react";
import {useI18n} from "../../../../i18n";
import {isSchedulerInfiniteCount} from "./usePaneScheduler";
import type {SchedulerEntry, SchedulerTemplate} from "./usePaneScheduler";

interface SchedulerListProps {
    entries: SchedulerEntry[];
    onStart: (id: string) => void;
    onDelete: (id: string) => void;
    onEdit: (entry: SchedulerEntry) => void;
    onStop: (id: string) => void;
    onNew: () => void;
    onSaveTemplate: (tmpl: SchedulerTemplate) => Promise<void>;
}

export function SchedulerList({entries, onStart, onDelete, onEdit, onStop, onNew, onSaveTemplate}: SchedulerListProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const [savingEntryID, setSavingEntryID] = useState<string | null>(null);

    const handleSaveTemplate = useCallback(
        async (entry: SchedulerEntry) => {
            if (savingEntryID !== null) {
                return;
            }
            setSavingEntryID(entry.id);
            try {
                await onSaveTemplate({
                    title: entry.title,
                    message: entry.message,
                    interval_seconds: entry.interval_seconds,
                    max_count: entry.max_count,
                });
            } catch (e) {
                console.warn("[pane-scheduler] handleSaveTemplate(list) catch", e);
            } finally {
                setSavingEntryID(null);
            }
        },
        [onSaveTemplate, savingEntryID],
    );

    return (
        <div className="pane-scheduler-list">
            <div className="pane-scheduler-toolbar">
                <button
                    type="button"
                    className="pane-scheduler-new-btn"
                    onClick={onNew}
                >
                    {tr("viewer.scheduler.list.new", "+ 新規", "+ New")}
                </button>
            </div>

            {entries.length === 0 ? (
                <div className="pane-scheduler-empty">{tr("viewer.scheduler.list.empty", "スケジューラーなし", "No schedulers")}</div>
            ) : (
                entries.map((entry) => (
                    <div
                        key={entry.id}
                        className={`pane-scheduler-card${entry.running ? "" : " pane-scheduler-card-stopped"}`}
                    >
                        <div className="pane-scheduler-card-header">
                            <span className="pane-scheduler-card-title">{entry.title}</span>
                            <div className="pane-scheduler-card-actions">
                                <button
                                    type="button"
                                    className="pane-scheduler-edit-card-btn"
                                    onClick={() => onEdit(entry)}
                                    title={tr("viewer.scheduler.list.edit", "編集", "Edit this scheduler")}
                                >
                                    {tr("viewer.scheduler.list.edit", "編集", "Edit")}
                                </button>
                                <button
                                    type="button"
                                    className="pane-scheduler-save-card-btn"
                                    onClick={() => void handleSaveTemplate(entry)}
                                    disabled={savingEntryID !== null}
                                    title={tr("viewer.scheduler.form.saveAsTemplate", "テンプレートとして保存", "Save as template")}
                                >
                                    {savingEntryID === entry.id
                                        ? tr("viewer.scheduler.form.saving", "保存中...", "Saving...")
                                        : tr("viewer.scheduler.list.save", "保存", "Save")}
                                </button>
                                {entry.running ? (
                                    <button
                                        type="button"
                                        className="pane-scheduler-stop-btn"
                                        onClick={() => onStop(entry.id)}
                                        title={tr("viewer.scheduler.list.stop", "停止", "Stop this scheduler")}
                                    >
                                        {tr("viewer.scheduler.list.stop", "停止", "Stop")}
                                    </button>
                                ) : (
                                    <>
                                        <button
                                            type="button"
                                            className="pane-scheduler-start-card-btn"
                                            onClick={() => onStart(entry.id)}
                                            title={tr("viewer.scheduler.list.start", "再開", "Restart this scheduler")}
                                        >
                                            {tr("viewer.scheduler.list.start", "開始", "Start")}
                                        </button>
                                        <button
                                            type="button"
                                            className="pane-scheduler-delete-card-btn"
                                            onClick={() => onDelete(entry.id)}
                                            title={tr("viewer.scheduler.list.delete", "削除", "Delete this scheduler")}
                                        >
                                            {tr("viewer.scheduler.list.delete", "削除", "Delete")}
                                        </button>
                                    </>
                                )}
                            </div>
                        </div>
                        <div className="pane-scheduler-card-meta">
                            <span>{tr("viewer.scheduler.list.status", "状態", "Status")}: {entry.running
                                ? tr("viewer.scheduler.list.running", "実行中", "Running")
                                : tr("viewer.scheduler.list.stopped", "停止", "Stopped")}</span>
                            <span>{tr("viewer.scheduler.list.pane", "ペイン", "Pane")}: {entry.pane_id}</span>
                            <span>{tr("viewer.scheduler.list.every", "間隔", "Every")} {entry.interval_seconds}s</span>
                        </div>
                        <div className="pane-scheduler-card-progress">
                            {tr("viewer.scheduler.list.sent", "送信済", "Sent")}: {entry.current_count} / {isSchedulerInfiniteCount(entry.max_count) ? "\u221E" : entry.max_count}
                        </div>
                    </div>
                ))
            )}
        </div>
    );
}
