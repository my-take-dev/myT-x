import {useState, useCallback, useRef} from "react";
import {useI18n} from "../../../../i18n";
import {ConfirmDialog} from "../../../ConfirmDialog";
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
type PendingSettingsNavigation = "back" | "close" | null;

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
    const [settingsDirty, setSettingsDirty] = useState(false);
    const [pendingSettingsNavigation, setPendingSettingsNavigation] = useState<PendingSettingsNavigation>(null);
    const statusRef = useRef(hook.status);
    statusRef.current = hook.status;

    const handleBack = useCallback(() => {
        setEditingItemID(null);
        setSettingsDirty(false);
        setScreen("list");
    }, []);

    const handleClose = useCallback(() => {
        if (screen === "settings" && settingsDirty) {
            setPendingSettingsNavigation("close");
            return;
        }
        closeView();
    }, [closeView, screen, settingsDirty]);

    const handleNew = useCallback(async () => {
        hook.setError(null);
        let readiness: OrchestratorReadiness;
        try {
            readiness = await hook.checkOrchestratorReady();
        } catch {
            // The hook already recorded the user-visible error state.
            return;
        }
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
            let readiness: OrchestratorReadiness;
            try {
                readiness = await hook.checkOrchestratorReady();
            } catch {
                // The hook already recorded the user-visible error state.
                return;
            }
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
        setSettingsDirty(false);
        setScreen("settings");
    }, [hook.setError]);

    const handleSettingsBack = useCallback(() => {
        if (settingsDirty) {
            setPendingSettingsNavigation("back");
            return;
        }
        handleBack();
    }, [handleBack, settingsDirty]);

    const handleConfirmDiscardSettingsChanges = useCallback((action: string) => {
        if (action !== "discard") {
            return;
        }
        const pendingAction = pendingSettingsNavigation;
        setPendingSettingsNavigation(null);
        setSettingsDirty(false);
        if (pendingAction === "close") {
            closeView();
            return;
        }
        handleBack();
    }, [closeView, handleBack, pendingSettingsNavigation]);

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
            onClose={handleClose}
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
                            let saved = false;
                            if (editingItemID) {
                                saved = await hook.updateItem(editingItemID, title, message, targetPaneID, clearBefore, clearCommand);
                            } else {
                                saved = await hook.addItem(title, message, targetPaneID, clearBefore, clearCommand);
                            }
                            if (saved) {
                                handleBack();
                            }
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
                        onError={hook.setError}
                        onBack={handleSettingsBack}
                        onDirtyChange={setSettingsDirty}
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
            <ConfirmDialog
                open={pendingSettingsNavigation !== null}
                title={tr(
                    "viewer.taskScheduler.unsaved.title",
                    "\u672a\u4fdd\u5b58\u306e\u8a2d\u5b9a",
                    "Unsaved Settings",
                )}
                message={tr(
                    "viewer.taskScheduler.unsaved.message",
                    "\u5909\u66f4\u3092\u4fdd\u5b58\u305b\u305a\u306b\u79fb\u52d5\u3057\u307e\u3059\u304b\uff1f",
                    "Leave without saving these scheduler settings?",
                )}
                actions={[{
                    label: tr(
                        "viewer.taskScheduler.unsaved.discard",
                        "\u4fdd\u5b58\u305b\u305a\u306b\u79fb\u52d5",
                        "Discard Changes",
                    ),
                    value: "discard",
                    variant: "danger",
                }]}
                onAction={handleConfirmDiscardSettingsChanges}
                onClose={() => setPendingSettingsNavigation(null)}
            />
        </ViewerPanelShell>
    );
}
