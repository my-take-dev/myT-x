import {useState, useCallback, useRef} from "react";
import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {
    isActiveQueueStatus,
    PENDING_ITEM_STATUS,
    useTaskScheduler,
    type QueueConfig,
    type QueueItem,
    type OrchestratorReadiness,
} from "./useTaskScheduler";
import {TaskSchedulerList} from "./TaskSchedulerList";
import {TaskSchedulerForm} from "./TaskSchedulerForm";
import {TaskSchedulerConfig} from "./TaskSchedulerConfig";
import {TaskSchedulerAlert} from "./TaskSchedulerAlert";
import {TaskSchedulerSettingsPanel} from "./TaskSchedulerSettings";

type Screen = "list" | "form" | "config" | "alert" | "settings";

export function TaskSchedulerView() {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const closeView = useViewerStore((s) => s.closeView);
    const openViewWithContext = useViewerStore((s) => s.openViewWithContext);
    const hook = useTaskScheduler();
    const [screen, setScreen] = useState<Screen>("list");
    const [editingItemID, setEditingItemID] = useState<string | null>(null);
    const [orchestratorReadiness, setOrchestratorReadiness] = useState<OrchestratorReadiness | null>(null);
    const statusRef = useRef(hook.status);
    statusRef.current = hook.status;

    const handleBack = useCallback(() => {
        setEditingItemID(null);
        setScreen("list");
    }, []);

    const handleNew = useCallback(async () => {
        hook.setError(null);
        const readiness = await hook.checkOrchestratorReady();
        if (!readiness.ready) {
            setOrchestratorReadiness(readiness);
            setScreen("alert");
            return;
        }
        setEditingItemID(null);
        setScreen("form");
    }, [hook.setError, hook.checkOrchestratorReady]);

    const handleEdit = useCallback(async (id: string) => {
        hook.setError(null);
        // Non-pending items will be auto-reset to pending on save — verify orchestrator readiness.
        const item = statusRef.current?.items.find((i) => i.id === id);
        if (item && item.status !== PENDING_ITEM_STATUS) {
            const readiness = await hook.checkOrchestratorReady();
            if (!readiness.ready) {
                setOrchestratorReadiness(readiness);
                setScreen("alert");
                return;
            }
        }
        setEditingItemID(id);
        setScreen("form");
    }, [hook.setError, hook.checkOrchestratorReady]);

    const handleOpenConfig = useCallback(() => {
        hook.setError(null);
        setScreen("config");
    }, [hook.setError]);

    const handleOpenSettings = useCallback(() => {
        hook.setError(null);
        setScreen("settings");
    }, [hook.setError]);

    const handleStart = useCallback(async (config: QueueConfig, items: QueueItem[]) => {
        const ok = await hook.start(config, items);
        if (ok) {
            setScreen("list");
        }
    }, [hook.start]);

    // Navigate to orchestrator-teams with the first available pane as a default hint.
    // The orchestrator-teams view provides its own pane selection UI.
    const handleRegisterMember = useCallback(() => {
        if (hook.availablePanes.length > 0) {
            openViewWithContext("orchestrator-teams", {
                addTermMemberPaneId: hook.availablePanes[0].id,
            });
            return;
        }
        openViewWithContext("orchestrator-teams", {});
    }, [hook.availablePanes, openViewWithContext]);

    const isRunning = isActiveQueueStatus(hook.status?.run_status);

    return (
        <ViewerPanelShell
            className="task-scheduler-view"
            title={tr("viewer.taskScheduler.title", "\u30bf\u30b9\u30af\u30b9\u30b1\u30b8\u30e5\u30fc\u30e9", "Task Scheduler")}
            onClose={closeView}
            onRefresh={hook.refreshStatus}
        >
            <div className="task-scheduler-body">
                {hook.error && (
                    <div className="task-scheduler-error">
                        <span>{hook.error}</span>
                        <button type="button" onClick={() => hook.setError(null)}>
                            {tr("viewer.taskScheduler.dismiss", "\u9589\u3058\u308b", "Dismiss")}
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
                        onSettings={handleOpenSettings}
                        isRunning={isRunning}
                    />
                )}

                {screen === "form" && (
                    <TaskSchedulerForm
                        availablePanes={hook.availablePanes}
                        messageTemplates={hook.settings?.message_templates ?? []}
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
                    />
                )}

                {screen === "config" && (
                    <TaskSchedulerConfig
                        items={hook.status?.items ?? []}
                        initialConfig={hook.status?.config ?? null}
                        savedSettings={hook.settings}
                        onStart={handleStart}
                        onBack={handleBack}
                        onOpenSettings={handleOpenSettings}
                    />
                )}

                {screen === "settings" && (
                    <TaskSchedulerSettingsPanel
                        initialSettings={hook.settings}
                        onSave={hook.saveSettings}
                        onBack={handleBack}
                    />
                )}

                {screen === "alert" && orchestratorReadiness && (
                    <TaskSchedulerAlert
                        readiness={orchestratorReadiness}
                        onBack={handleBack}
                        onRegisterMember={handleRegisterMember}
                    />
                )}
            </div>
        </ViewerPanelShell>
    );
}
