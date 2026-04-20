import {useState, useCallback, useEffect, useRef} from "react";
import {useI18n} from "../../../../i18n";
import {ConfirmDialog} from "../../../ConfirmDialog";
import type {TaskSchedulerTemplateViewContext} from "../../viewerContext";
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
type PendingNavigation = "back" | "close" | "palette" | "settings" | null;

interface TaskSchedulerPaletteDraft {
    key: string;
    title: string;
    message: string;
    targetPaneID: string;
    clearBefore: boolean;
    clearCommand: string;
}

export function parseTaskSchedulerPaletteDraft(
    viewContext: TaskSchedulerTemplateViewContext | null,
): TaskSchedulerPaletteDraft | null {
    if (viewContext === null || viewContext.kind !== "task-scheduler-template") {
        return null;
    }

    const message = viewContext.message.trim();
    if (message === "") {
        return null;
    }

    const name = viewContext.name.trim();
    const targetPaneID = viewContext.targetPaneID;
    const clearCommand = viewContext.clearCommand;

    return {
        key: viewContext.key.trim() !== ""
            ? viewContext.key
            : `task-template:${name}:${message}`,
        title: name,
        message,
        targetPaneID,
        clearBefore: viewContext.clearBefore,
        clearCommand,
    };
}

export function TaskSchedulerView() {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const closeView = useViewerStore((s) => s.closeView);
    const clearViewContext = useViewerStore((s) => s.clearViewContext);
    const openViewWithContext = useViewerStore((s) => s.openViewWithContext);
    const viewContext = useViewerStore((s) => s.viewContext);
    const hook = useTaskScheduler();
    const [screen, setScreen] = useState<Screen>("list");
    const [editingItemID, setEditingItemID] = useState<string | null>(null);
    const [paletteDraft, setPaletteDraft] = useState<TaskSchedulerPaletteDraft | null>(null);
    const [pendingPaletteDraft, setPendingPaletteDraft] = useState<TaskSchedulerPaletteDraft | null>(null);
    const [orchestratorReadiness, setOrchestratorReadiness] = useState<OrchestratorReadiness | null>(null);
    const [formDirty, setFormDirty] = useState(false);
    const [configDirty, setConfigDirty] = useState(false);
    const [settingsDirty, setSettingsDirty] = useState(false);
    const [pendingNavigation, setPendingNavigation] = useState<PendingNavigation>(null);
    const readinessRequestIDRef = useRef(0);
    const statusRef = useRef(hook.status);
    statusRef.current = hook.status;

    const hasUnsavedChanges = (
        (screen === "form" && formDirty)
        || (screen === "config" && configDirty)
        || (screen === "settings" && settingsDirty)
    );

    const resetDirtyState = useCallback(() => {
        setFormDirty(false);
        setConfigDirty(false);
        setSettingsDirty(false);
    }, []);

    const resetToList = useCallback(() => {
        setEditingItemID(null);
        setPaletteDraft(null);
        setPendingPaletteDraft(null);
        resetDirtyState();
        setScreen("list");
    }, [resetDirtyState]);

    const openSettingsScreen = useCallback(() => {
        hook.setError(null);
        setPaletteDraft(null);
        setConfigDirty(false);
        setSettingsDirty(false);
        setScreen("settings");
    }, [hook.setError]);

    const requestOrchestratorReadiness = useCallback(async (capturedSessionKey: string) => {
        const requestID = ++readinessRequestIDRef.current;
        let readiness: OrchestratorReadiness;
        try {
            readiness = await hook.checkOrchestratorReady();
        } catch {
            // The hook already recorded the user-visible error state.
            return null;
        }
        if (
            readinessRequestIDRef.current !== requestID
            || hook.shouldIgnoreSessionResult(capturedSessionKey)
        ) {
            return null;
        }
        return readiness;
    }, [hook.checkOrchestratorReady, hook.shouldIgnoreSessionResult]);

    const handleBack = useCallback(() => {
        if (hasUnsavedChanges) {
            setPendingNavigation("back");
            return;
        }
        resetToList();
    }, [hasUnsavedChanges, resetToList]);

    const handleClose = useCallback(() => {
        if (hasUnsavedChanges) {
            setPendingNavigation("close");
            return;
        }
        closeView();
    }, [closeView, hasUnsavedChanges]);

    const handleNew = useCallback(async () => {
        const capturedSessionKey = hook.activeSessionKey;
        hook.setError(null);
        setPaletteDraft(null);
        const readiness = await requestOrchestratorReadiness(capturedSessionKey);
        if (readiness === null) {
            return;
        }
        if (!readiness.ready) {
            setOrchestratorReadiness(readiness);
            setScreen("alert");
            return;
        }
        setEditingItemID(null);
        setFormDirty(false);
        setScreen("form");
    }, [hook.activeSessionKey, hook.setError, requestOrchestratorReadiness]);

    const handleEdit = useCallback(async (id: string) => {
        const capturedSessionKey = hook.activeSessionKey;
        hook.setError(null);
        setPaletteDraft(null);
        const item = statusRef.current?.items.find((entry) => entry.id === id);
        if (item && item.status !== PENDING_ITEM_STATUS) {
            const readiness = await requestOrchestratorReadiness(capturedSessionKey);
            if (readiness === null) {
                return;
            }
            if (!readiness.ready) {
                setOrchestratorReadiness(readiness);
                setScreen("alert");
                return;
            }
        }
        if (hook.shouldIgnoreSessionResult(capturedSessionKey)) {
            return;
        }
        setEditingItemID(id);
        setFormDirty(false);
        setScreen("form");
    }, [hook.activeSessionKey, hook.setError, requestOrchestratorReadiness]);

    const handleOpenConfig = useCallback(() => {
        hook.setError(null);
        setPaletteDraft(null);
        setConfigDirty(false);
        setScreen("config");
    }, [hook.setError]);

    const handleOpenSettings = useCallback(() => {
        if (screen === "config" && configDirty) {
            setPendingNavigation("settings");
            return;
        }
        openSettingsScreen();
    }, [configDirty, openSettingsScreen, screen]);

    const openPaletteDraft = useCallback(async (nextPaletteDraft: TaskSchedulerPaletteDraft) => {
        const capturedSessionKey = hook.activeSessionKey;
        hook.setError(null);
        const readiness = await requestOrchestratorReadiness(capturedSessionKey);
        if (readiness === null) {
            return;
        }

        resetDirtyState();
        if (!readiness.ready) {
            setPaletteDraft(null);
            setOrchestratorReadiness(readiness);
            setScreen("alert");
            return;
        }

        setEditingItemID(null);
        setPaletteDraft(nextPaletteDraft);
        setScreen("form");
    }, [hook.activeSessionKey, hook.setError, requestOrchestratorReadiness, resetDirtyState]);

    const handleConfirmDiscardChanges = useCallback((action: string) => {
        if (action !== "discard") {
            return;
        }
        const nextNavigation = pendingNavigation;
        const nextPaletteDraft = pendingPaletteDraft;
        setPendingNavigation(null);
        setPendingPaletteDraft(null);
        if (nextNavigation === "close") {
            resetDirtyState();
            closeView();
            return;
        }
        if (nextNavigation === "palette" && nextPaletteDraft !== null) {
            void openPaletteDraft(nextPaletteDraft);
            return;
        }
        if (nextNavigation === "settings") {
            resetDirtyState();
            openSettingsScreen();
            return;
        }
        resetDirtyState();
        resetToList();
    }, [closeView, openPaletteDraft, openSettingsScreen, pendingNavigation, pendingPaletteDraft, resetDirtyState, resetToList]);

    const handleStart = useCallback(async (config: QueueConfig, items: QueueItem[]) => {
        if (hook.activeSessionKey === "") {
            hook.setError(tr(
                "viewer.taskScheduler.error.noActiveSession",
                "アクティブなセッションがありません",
                "No active session",
            ));
            return;
        }
        const ok = await hook.start(config, items);
        if (ok) {
            setConfigDirty(false);
            setScreen("list");
        }
    }, [hook.activeSessionKey, hook.setError, hook.start, tr]);

    const confirmDialogTitle = screen === "settings"
        ? tr(
            "viewer.taskScheduler.unsaved.title",
            "未保存の設定",
            "Unsaved Settings",
        )
        : tr(
            "viewer.taskScheduler.unsaved.genericTitle",
            "未保存の変更",
            "Unsaved Changes",
        );

    const confirmDialogMessage = screen === "settings"
        ? tr(
            "viewer.taskScheduler.unsaved.message",
            "変更を保存せずに移動しますか？",
            "Leave without saving these scheduler settings?",
        )
        : pendingNavigation === "close"
            ? tr(
                "viewer.taskScheduler.unsaved.genericCloseMessage",
                "タスクスケジューラの変更を保存せずに閉じますか？",
                "Close without saving your task scheduler changes?",
            )
            : tr(
                "viewer.taskScheduler.unsaved.genericBackMessage",
                "タスクスケジューラの変更を保存せずに移動しますか？",
                "Leave without saving your task scheduler changes?",
            );

    const confirmDiscardLabel = screen === "settings"
        ? tr(
            "viewer.taskScheduler.unsaved.discard",
            "保存せずに移動",
            "Discard Changes",
        )
        : pendingNavigation === "close"
            ? tr(
                "viewer.taskScheduler.unsaved.genericDiscardAndClose",
                "保存せずに閉じる",
                "Discard and Close",
            )
            : tr(
                "viewer.taskScheduler.unsaved.genericDiscard",
                "保存せずに移動",
                "Discard Changes",
            );

    useEffect(() => {
        const nextPaletteDraft = parseTaskSchedulerPaletteDraft(
            viewContext?.kind === "task-scheduler-template" ? viewContext : null,
        );
        if (nextPaletteDraft === null) {
            return;
        }

        clearViewContext();

        if (hasUnsavedChanges) {
            setPendingPaletteDraft(nextPaletteDraft);
            setPendingNavigation("palette");
            return;
        }

        void openPaletteDraft(nextPaletteDraft);
    }, [clearViewContext, hasUnsavedChanges, openPaletteDraft, viewContext]);

    const handleRegisterMember = useCallback(() => {
        if (hook.availablePanes.length > 0) {
            openViewWithContext("orchestrator-teams", {
                kind: "orchestrator-teams-add-term-member",
                addTermMemberPaneId: hook.availablePanes[0].id,
            });
            return;
        }
        openViewWithContext("orchestrator-teams", {kind: "orchestrator-teams-default"});
    }, [hook.availablePanes, openViewWithContext]);

    const isRunning = isActiveQueueStatus(hook.status?.run_status);

    return (
        <ViewerPanelShell
            className="task-scheduler-view"
            title={tr("viewer.taskScheduler.panelTitle", "タスクスケジューラ", "Task Scheduler")}
            onClose={handleClose}
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
                        onSettings={handleOpenSettings}
                        isRunning={isRunning}
                    />
                )}

                {screen === "form" && (
                    <TaskSchedulerForm
                        key={editingItemID ?? paletteDraft?.key ?? "new-task"}
                        availablePanes={hook.availablePanes}
                        messageTemplates={hook.settings?.message_templates ?? []}
                        editingItem={editingItemID && hook.status
                            ? hook.status.items.find((item) => item.id === editingItemID) ?? null
                            : null
                        }
                        initialDraft={editingItemID ? null : paletteDraft}
                        onDirtyChange={setFormDirty}
                        onSave={async (title, message, targetPaneID, clearBefore, clearCommand) => {
                            let saved: boolean;
                            if (editingItemID) {
                                saved = await hook.updateItem(editingItemID, title, message, targetPaneID, clearBefore, clearCommand);
                            } else {
                                saved = await hook.addItem(title, message, targetPaneID, clearBefore, clearCommand);
                            }
                            if (saved) {
                                resetToList();
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
                        onDirtyChange={setConfigDirty}
                    />
                )}

                {screen === "settings" && (
                    <TaskSchedulerSettingsPanel
                        initialSettings={hook.settings}
                        onSave={hook.saveSettings}
                        onError={hook.setError}
                        onBack={handleBack}
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
                open={pendingNavigation !== null}
                title={confirmDialogTitle}
                message={confirmDialogMessage}
                actions={[{
                    label: confirmDiscardLabel,
                    value: "discard",
                    variant: "danger",
                }]}
                onAction={handleConfirmDiscardChanges}
                onClose={() => {
                    setPendingNavigation(null);
                    setPendingPaletteDraft(null);
                }}
            />
        </ViewerPanelShell>
    );
}
