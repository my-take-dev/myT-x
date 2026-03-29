import {useState, useCallback} from "react";
import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {useTaskScheduler, type QueueConfig, type QueueItem} from "./useTaskScheduler";
import {TaskSchedulerList} from "./TaskSchedulerList";
import {TaskSchedulerForm} from "./TaskSchedulerForm";
import {TaskSchedulerConfig} from "./TaskSchedulerConfig";

type Screen = "list" | "form" | "config";

export function TaskSchedulerView() {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const closeView = useViewerStore((s) => s.closeView);
    const hook = useTaskScheduler();
    const [screen, setScreen] = useState<Screen>("list");
    const [editingItemID, setEditingItemID] = useState<string | null>(null);

    const handleBack = useCallback(() => {
        setEditingItemID(null);
        setScreen("list");
    }, []);

    const handleNew = useCallback(() => {
        hook.setError(null);
        setEditingItemID(null);
        setScreen("form");
    }, [hook.setError]);

    const handleEdit = useCallback((id: string) => {
        hook.setError(null);
        setEditingItemID(id);
        setScreen("form");
    }, [hook.setError]);

    const handleOpenConfig = useCallback(() => {
        hook.setError(null);
        setScreen("config");
    }, [hook.setError]);

    const handleStart = useCallback(async (config: QueueConfig, items: QueueItem[]) => {
        const ok = await hook.start(config, items);
        if (ok) {
            setScreen("list");
        }
    }, [hook.start]);

    const isRunning = hook.status?.run_status === "running" || hook.status?.run_status === "paused";

    return (
        <ViewerPanelShell
            className="task-scheduler-view"
            title={tr("viewer.taskScheduler.title", "タスクスケジューラ", "Task Scheduler")}
            onClose={closeView}
            onRefresh={hook.refreshStatus}
        >
            <div className="task-scheduler-body">
                {hook.error && (
                    <div className="task-scheduler-error">
                        <span>{hook.error}</span>
                        <button type="button" onClick={() => hook.setError(null)}>
                            {tr("viewer.taskScheduler.dismiss", "閉じる", "Dismiss")}
                        </button>
                    </div>
                )}

                {screen === "list" && (
                    <TaskSchedulerList
                        status={hook.status}
                        onNew={handleNew}
                        onEdit={handleEdit}
                        onRemove={hook.removeItem}
                        onStart={handleOpenConfig}
                        onStop={hook.stop}
                        onPause={hook.pause}
                        onResume={hook.resume}
                        isRunning={isRunning}
                    />
                )}

                {screen === "form" && (
                    <TaskSchedulerForm
                        availablePanes={hook.availablePanes}
                        editingItem={editingItemID && hook.status
                            ? hook.status.items.find((i) => i.id === editingItemID) ?? null
                            : null
                        }
                        onSave={async (title, message, targetPaneID, clearBefore, clearCommand) => {
                            if (editingItemID) {
                                await hook.updateItem(editingItemID, title, message, targetPaneID, clearBefore, clearCommand);
                            } else {
                                await hook.addItem(title, message, targetPaneID, clearBefore, clearCommand);
                            }
                            handleBack();
                        }}
                        onBack={handleBack}
                        isRunning={isRunning}
                    />
                )}

                {screen === "config" && (
                    <TaskSchedulerConfig
                        items={hook.status?.items ?? []}
                        onStart={handleStart}
                        onBack={handleBack}
                    />
                )}
            </div>
        </ViewerPanelShell>
    );
}
