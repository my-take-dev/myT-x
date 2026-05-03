import {useCallback, useEffect, useState} from "react";
import {useI18n} from "../../../../i18n";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {SingleTaskRunnerForm} from "./SingleTaskRunnerForm";
import {SingleTaskRunnerList} from "./SingleTaskRunnerList";
import {type QueueItem, useSingleTaskRunner} from "./useSingleTaskRunner";

type Screen = "list" | "form";

export function SingleTaskRunnerView() {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const closeView = useViewerStore((s) => s.closeView);
    const activeSession = useTmuxStore((s) => s.activeSession);
    const hook = useSingleTaskRunner();
    const {setError, updateItem} = hook;
    const [screen, setScreen] = useState<Screen>("list");
    const [editingItemID, setEditingItemID] = useState<string | null>(null);

    // Reset local form state when the active session changes.
    useEffect(() => {
        setScreen("list");
        setEditingItemID(null);
    }, [activeSession]);

    const handleBack = useCallback(() => {
        setEditingItemID(null);
        setScreen("list");
    }, []);

    const handleNew = useCallback(() => {
        setError(null);
        setEditingItemID(null);
        setScreen("form");
    }, [setError]);

    const handleEdit = useCallback((id: string) => {
        setError(null);
        setEditingItemID(id);
        setScreen("form");
    }, [setError]);

    const handleToggleClearBefore = useCallback(async (item: QueueItem, clearBefore: boolean) => {
        return await updateItem(
            item.id,
            item.title,
            item.message,
            item.target_pane_id,
            clearBefore,
            item.clear_command ?? "",
        );
    }, [updateItem]);

    return (
        <ViewerPanelShell
            className="single-task-runner-view"
            title={tr("viewer.singleTaskRunner.title", "シングルタスクランナー", "Single Task Runner")}
            onClose={closeView}
            onRefresh={hook.refreshStatus}
        >
            <div className="single-task-runner-body">
                {hook.error && (
                    <div className="single-task-runner-error">
                        <span>{hook.error}</span>
                        <button type="button" onClick={() => hook.setError(null)}>
                            {tr("viewer.singleTaskRunner.dismiss", "閉じる", "Dismiss")}
                        </button>
                    </div>
                )}

                {screen === "list" && (
                    <SingleTaskRunnerList
                        defaultClearDelay={hook.defaultClearDelay}
                        status={hook.status}
                        onNew={handleNew}
                        onEdit={handleEdit}
                        onRemove={hook.removeItem}
                        onStart={hook.start}
                        onStop={hook.stop}
                        onSetClearDelay={hook.setClearDelay}
                        onToggleClearBefore={handleToggleClearBefore}
                        onError={hook.setError}
                    />
                )}

                {screen === "form" && (
                    <SingleTaskRunnerForm
                        availablePanes={hook.availablePanes}
                        editingItem={editingItemID && hook.status
                            ? hook.status.items.find((item) => item.id === editingItemID) ?? null
                            : null}
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
                            return saved;
                        }}
                        onBack={handleBack}
                    />
                )}
            </div>
        </ViewerPanelShell>
    );
}
