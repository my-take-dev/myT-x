import {useCallback, useState} from "react";
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
                    interval_minutes: entry.interval_minutes,
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
                    + New
                </button>
            </div>

            {entries.length === 0 ? (
                <div className="pane-scheduler-empty">No schedulers</div>
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
                                    title="Edit this scheduler"
                                >
                                    Edit
                                </button>
                                <button
                                    type="button"
                                    className="pane-scheduler-save-card-btn"
                                    onClick={() => void handleSaveTemplate(entry)}
                                    disabled={savingEntryID !== null}
                                    title="Save as template"
                                >
                                    {savingEntryID === entry.id ? "Saving..." : "Save"}
                                </button>
                                {entry.running ? (
                                    <button
                                        type="button"
                                        className="pane-scheduler-stop-btn"
                                        onClick={() => onStop(entry.id)}
                                        title="Stop this scheduler"
                                    >
                                        Stop
                                    </button>
                                ) : (
                                    <>
                                        <button
                                            type="button"
                                            className="pane-scheduler-start-card-btn"
                                            onClick={() => onStart(entry.id)}
                                            title="Restart this scheduler"
                                        >
                                            Start
                                        </button>
                                        <button
                                            type="button"
                                            className="pane-scheduler-delete-card-btn"
                                            onClick={() => onDelete(entry.id)}
                                            title="Delete this scheduler"
                                        >
                                            Delete
                                        </button>
                                    </>
                                )}
                            </div>
                        </div>
                        <div className="pane-scheduler-card-meta">
                            <span>Status: {entry.running ? "Running" : "Stopped"}</span>
                            <span>Pane: {entry.pane_id}</span>
                            <span>Every {entry.interval_minutes}m</span>
                        </div>
                        <div className="pane-scheduler-card-progress">
                            Sent: {entry.current_count} / {isSchedulerInfiniteCount(entry.max_count) ? "\u221E" : entry.max_count}
                        </div>
                    </div>
                ))
            )}
        </div>
    );
}
