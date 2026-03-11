import {useState, useCallback} from "react";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {
    createSchedulerEditDraft,
    submitSchedulerChanges,
    usePaneScheduler,
    type SchedulerEditDraft,
    type SchedulerEntry,
    type SchedulerStartValues,
} from "./usePaneScheduler";
import {SchedulerList} from "./SchedulerList";
import {SchedulerForm} from "./SchedulerForm";

type Screen = "list" | "form";

export function PaneSchedulerView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        entries, templates, error, setError, availablePanes,
        start, stop, stopOrThrow, resume, deleteScheduler, deleteSchedulerOrThrow,
        saveTemplate, deleteTemplate, refreshStatuses,
    } = usePaneScheduler();
    const [screen, setScreen] = useState<Screen>("list");
    const [editingDraft, setEditingDraft] = useState<SchedulerEditDraft | null>(null);

    const handleBack = useCallback(() => {
        setEditingDraft(null);
        setScreen("list");
    }, []);
    const handleNew = useCallback(() => {
        setError(null);
        setEditingDraft(null);
        setScreen("form");
    }, [setError]);
    const handleEdit = useCallback((entry: SchedulerEntry) => {
        setError(null);
        setEditingDraft(createSchedulerEditDraft(entry));
        setScreen("form");
    }, [setError]);
    const handleStart = useCallback(async (values: SchedulerStartValues) => {
        await submitSchedulerChanges(start, stopOrThrow, deleteSchedulerOrThrow, values, editingDraft);
    }, [deleteSchedulerOrThrow, editingDraft, start, stopOrThrow]);

    return (
        <ViewerPanelShell
            className="pane-scheduler-view"
            title="Schedule"
            onClose={closeView}
            onRefresh={refreshStatuses}
        >
            <div className="pane-scheduler-body">
                {error && (
                    <div className="pane-scheduler-error">
                        <span>{error}</span>
                        <button type="button" onClick={() => setError(null)}>
                            Dismiss
                        </button>
                    </div>
                )}

                {screen === "list" ? (
                    <SchedulerList
                        entries={entries}
                        onEdit={handleEdit}
                        onStart={(id) => void resume(id)}
                        onDelete={(id) => void deleteScheduler(id)}
                        onStop={(id) => void stop(id)}
                        onNew={handleNew}
                        onSaveTemplate={saveTemplate}
                    />
                ) : (
                    <SchedulerForm
                        availablePanes={availablePanes}
                        initialDraft={editingDraft}
                        templates={templates}
                        onStart={handleStart}
                        onBack={handleBack}
                        onSaveTemplate={saveTemplate}
                        onDeleteTemplate={deleteTemplate}
                        submitLabel={editingDraft === null ? "Start" : "Apply Changes"}
                    />
                )}
            </div>
        </ViewerPanelShell>
    );
}
