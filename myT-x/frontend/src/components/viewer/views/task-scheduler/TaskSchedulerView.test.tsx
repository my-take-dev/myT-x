import {act, type ReactNode, useState} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../../../../i18n";
import {TaskSchedulerView} from "./TaskSchedulerView";
import {useViewerStore} from "../../viewerStore";

const checkOrchestratorReadyMock = vi.fn<() => Promise<{ready: boolean}>>();
const setErrorMock = vi.fn<(message: string | null) => void>();
const startMock = vi.fn();
const stopMock = vi.fn();
const pauseMock = vi.fn();
const resumeMock = vi.fn();
const addItemMock = vi.fn();
const updateItemMock = vi.fn();
const removeItemMock = vi.fn();
const refreshStatusMock = vi.fn();
const saveSettingsMock = vi.fn();
const taskSchedulerFormMock = vi.fn();
let currentSessionKey = "alpha:1";
let currentStatus: {
    items: Array<{id: string; status: string}>;
    config?: object;
} | null = null;

function createTemplateContext(name = "Review Template", key = "template:review") {
    return {
        kind: "task-scheduler-template" as const,
        key,
        name,
        message: `Message for ${name}`,
        targetPaneID: "%1",
        clearBefore: false,
        clearCommand: "",
    };
}

function createDeferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((resolveFn, rejectFn) => {
        resolve = resolveFn;
        reject = rejectFn;
    });
    return {promise, resolve, reject};
}

vi.mock("./useTaskScheduler", () => ({
    PENDING_ITEM_STATUS: "pending",
    isActiveQueueStatus: () => false,
    useTaskScheduler: () => ({
        status: currentStatus,
        error: null,
        setError: setErrorMock,
        activeSessionKey: currentSessionKey,
        availablePanes: [{id: "%1", index: 0, title: "pane", active: true, width: 80, height: 24}],
        settings: {message_templates: []},
        start: startMock,
        stop: stopMock,
        pause: pauseMock,
        resume: resumeMock,
        addItem: addItemMock,
        removeItem: removeItemMock,
        reorderItems: vi.fn(),
        updateItem: updateItemMock,
        refreshStatus: refreshStatusMock,
        checkOrchestratorReady: () => checkOrchestratorReadyMock(),
        saveSettings: saveSettingsMock,
        shouldIgnoreSessionResult: (capturedSessionKey: string) => capturedSessionKey !== currentSessionKey,
    }),
}));

vi.mock("../shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({
        children,
        onClose,
    }: {
        children: ReactNode;
        onClose: () => void;
    }) => (
        <div>
            <button type="button" data-testid="shell-close" onClick={onClose}>shell-close</button>
            {children}
        </div>
    ),
}));

vi.mock("./TaskSchedulerList", () => ({
    TaskSchedulerList: ({
        onSettings,
        onNew,
        onStart,
        onEdit,
    }: {
        onSettings: () => void;
        onNew: () => void;
        onStart: () => void;
        onEdit: (id: string) => void;
    }) => (
        <div>
            <button type="button" data-testid="open-settings" onClick={onSettings}>task-list</button>
            <button type="button" data-testid="open-new" onClick={onNew}>task-new</button>
            <button type="button" data-testid="open-config" onClick={onStart}>task-config</button>
            <button type="button" data-testid="edit-item-1" onClick={() => onEdit("item-1")}>edit-item-1</button>
            <button type="button" data-testid="edit-item-2" onClick={() => onEdit("item-2")}>edit-item-2</button>
        </div>
    ),
}));

vi.mock("./TaskSchedulerConfig", () => ({
    TaskSchedulerConfig: (props: {
        onBack: () => void;
        onStart: (config: object, items: object[]) => void;
        onOpenSettings: () => void;
        onDirtyChange?: (dirty: boolean) => void;
    }) => (
        <div data-testid="task-config">
            <button
                type="button"
                data-testid="mark-config-dirty"
                onClick={() => props.onDirtyChange?.(true)}
            >
                mark-config-dirty
            </button>
            <button type="button" data-testid="config-open-settings" onClick={props.onOpenSettings}>
                config-open-settings
            </button>
            <button type="button" data-testid="config-start" onClick={() => props.onStart({}, [])}>
                config-start
            </button>
            <button type="button" data-testid="config-back" onClick={props.onBack}>
                config-back
            </button>
        </div>
    ),
}));

vi.mock("./TaskSchedulerAlert", () => ({
    TaskSchedulerAlert: () => <div data-testid="task-alert">task-alert</div>,
}));

vi.mock("./TaskSchedulerSettings", () => ({
    TaskSchedulerSettingsPanel: (props: {
        onBack: () => void;
        onDirtyChange?: (dirty: boolean) => void;
    }) => (
        <div>
            <button
                type="button"
                data-testid="mark-dirty"
                onClick={() => props.onDirtyChange?.(true)}
            >
                mark-dirty
            </button>
            <button type="button" data-testid="settings-back" onClick={props.onBack}>
                settings-back
            </button>
        </div>
    ),
}));

vi.mock("./TaskSchedulerForm", () => ({
    TaskSchedulerForm: (props: {
        editingItem: {id: string} | null;
        initialDraft: {targetPaneID: string} | null;
        onDirtyChange?: (dirty: boolean) => void;
    }) => {
        taskSchedulerFormMock(props);
        const [paneValue] = useState(props.initialDraft?.targetPaneID ?? "");
        return (
            <div data-testid="task-form">
                <span data-testid="form-pane-value">{paneValue}</span>
                <span data-testid="form-editing-id">{props.editingItem?.id ?? ""}</span>
                <button
                    type="button"
                    data-testid="mark-form-dirty"
                    onClick={() => props.onDirtyChange?.(true)}
                >
                    mark-form-dirty
                </button>
            </div>
        );
    },
}));

vi.mock("../../../ConfirmDialog", () => ({
    ConfirmDialog: (props: {
        open: boolean;
        title: string;
        message: string;
        actions: Array<{label: string; value: string}>;
        onAction: (value: string) => void;
        onClose: () => void;
    }) => {
        if (!props.open) {
            return null;
        }
        return (
            <div data-testid="confirm-dialog">
                <div data-testid="confirm-title">{props.title}</div>
                <div data-testid="confirm-message">{props.message}</div>
                <button type="button" data-testid="discard-changes" onClick={() => props.onAction("discard")}>
                    {props.actions[0]?.label ?? "discard"}
                </button>
                <button type="button" data-testid="close-dialog" onClick={props.onClose}>
                    close
                </button>
            </div>
        );
    },
}));

describe("TaskSchedulerView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;

        checkOrchestratorReadyMock.mockReset().mockResolvedValue({ready: true});
        setErrorMock.mockReset();
        startMock.mockReset();
        stopMock.mockReset();
        pauseMock.mockReset();
        resumeMock.mockReset();
        addItemMock.mockReset();
        updateItemMock.mockReset();
        removeItemMock.mockReset();
        refreshStatusMock.mockReset();
        saveSettingsMock.mockReset();
        taskSchedulerFormMock.mockReset();
        currentSessionKey = "alpha:1";
        currentStatus = null;
        setLanguage("ja");
        useViewerStore.setState({
            activeViewId: "task-scheduler",
            viewContext: null,
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("consumes template context into a prefilled task form", async () => {
        useViewerStore.setState({
            activeViewId: "task-scheduler",
            viewContext: createTemplateContext(),
        });

        await act(async () => {
            root.render(<TaskSchedulerView/>);
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(checkOrchestratorReadyMock).toHaveBeenCalledTimes(1);
        expect(taskSchedulerFormMock).toHaveBeenCalledTimes(1);
        expect(taskSchedulerFormMock.mock.calls[0]?.[0]).toEqual(expect.objectContaining({
            initialDraft: expect.objectContaining({
                title: "Review Template",
                message: "Message for Review Template",
                targetPaneID: "%1",
            }),
            editingItem: null,
        }));
        expect(useViewerStore.getState().viewContext).toBeNull();
    });

    it("shows the alert screen when template readiness is not ready", async () => {
        checkOrchestratorReadyMock.mockResolvedValueOnce({ready: false});
        useViewerStore.setState({
            activeViewId: "task-scheduler",
            viewContext: createTemplateContext(),
        });

        await act(async () => {
            root.render(<TaskSchedulerView/>);
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(taskSchedulerFormMock).not.toHaveBeenCalled();
        expect(container.querySelector("[data-testid='task-alert']")).not.toBeNull();
    });

    it("drops a pending palette draft when the confirm dialog is dismissed", async () => {
        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-settings']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext(),
            });
            await Promise.resolve();
        });

        expect(checkOrchestratorReadyMock).toHaveBeenCalledTimes(0);
        expect(container.querySelector("[data-testid='confirm-dialog']")).not.toBeNull();

        await act(async () => {
            (container.querySelector("[data-testid='close-dialog']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
        });

        await act(async () => {
            (container.querySelector("[data-testid='settings-back']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='discard-changes']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(checkOrchestratorReadyMock).toHaveBeenCalledTimes(0);
        expect(taskSchedulerFormMock).not.toHaveBeenCalled();
        expect(container.querySelector("[data-testid='open-settings']")).not.toBeNull();
    });

    it("keeps only the latest palette selection when readiness requests resolve out of order", async () => {
        const firstRequest = createDeferred<{ready: boolean}>();
        const secondRequest = createDeferred<{ready: boolean}>();
        checkOrchestratorReadyMock
            .mockImplementationOnce(() => firstRequest.promise)
            .mockImplementationOnce(() => secondRequest.promise);

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext("Alpha Template", "template:alpha"),
            });
            await Promise.resolve();
        });

        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext("Beta Template", "template:beta"),
            });
            await Promise.resolve();
        });

        await act(async () => {
            secondRequest.resolve({ready: true});
            await Promise.resolve();
            await Promise.resolve();
        });

        await act(async () => {
            firstRequest.resolve({ready: true});
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(taskSchedulerFormMock).toHaveBeenCalledTimes(1);
        expect(taskSchedulerFormMock.mock.calls[0]?.[0]).toEqual(expect.objectContaining({
            initialDraft: expect.objectContaining({
                key: "template:beta",
                title: "Beta Template",
                message: "Message for Beta Template",
            }),
        }));
    });

    it("ignores readiness results after the active session changes", async () => {
        const deferred = createDeferred<{ready: boolean}>();
        checkOrchestratorReadyMock.mockImplementationOnce(() => deferred.promise);

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-new']") as HTMLButtonElement | null)?.click();
        });

        currentSessionKey = "beta:2";

        await act(async () => {
            deferred.resolve({ready: true});
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(container.querySelector("[data-testid='task-form']")).toBeNull();
        expect(container.querySelector("[data-testid='task-alert']")).toBeNull();
        expect(container.querySelector("[data-testid='open-settings']")).not.toBeNull();
    });

    it("keeps the current form draft until dirty changes are discarded", async () => {
        await act(async () => {
            root.render(<TaskSchedulerView/>);
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext("Alpha Template", "template:alpha"),
            });
            await Promise.resolve();
            await Promise.resolve();
        });

        await act(async () => {
            (container.querySelector("[data-testid='mark-form-dirty']") as HTMLButtonElement | null)?.click();
        });

        expect((container.querySelector("[data-testid='form-pane-value']") as HTMLElement | null)?.textContent).toBe("%1");

        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: {
                    ...createTemplateContext("Beta Template", "template:beta"),
                    targetPaneID: "%2",
                },
            });
            await Promise.resolve();
        });

        expect(container.querySelector("[data-testid='confirm-dialog']")).not.toBeNull();
        expect((container.querySelector("[data-testid='form-pane-value']") as HTMLElement | null)?.textContent).toBe("%1");

        await act(async () => {
            (container.querySelector("[data-testid='discard-changes']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
            await Promise.resolve();
        });

        expect((container.querySelector("[data-testid='form-pane-value']") as HTMLElement | null)?.textContent).toBe("%2");
    });

    it("opens settings only after dirty queue config changes are discarded", async () => {
        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-config']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-config-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='config-open-settings']") as HTMLButtonElement | null)?.click();
        });

        expect(container.querySelector("[data-testid='confirm-dialog']")).not.toBeNull();
        expect(container.querySelector("[data-testid='task-config']")).not.toBeNull();

        await act(async () => {
            (container.querySelector("[data-testid='discard-changes']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
        });

        expect(container.querySelector("[data-testid='mark-dirty']")).not.toBeNull();
    });

    it("renders generic unsaved dialog copy in Japanese", async () => {
        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-config']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-config-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='config-back']") as HTMLButtonElement | null)?.click();
        });

        expect((container.querySelector("[data-testid='confirm-title']") as HTMLElement | null)?.textContent).toBe("未保存の変更");
        expect((container.querySelector("[data-testid='confirm-message']") as HTMLElement | null)?.textContent).toBe("タスクスケジューラの変更を保存せずに移動しますか？");
        expect((container.querySelector("[data-testid='discard-changes']") as HTMLElement | null)?.textContent).toBe("保存せずに移動");
    });

    it("renders generic unsaved dialog copy in English", async () => {
        setLanguage("en");

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-config']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-config-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='config-back']") as HTMLButtonElement | null)?.click();
        });

        expect((container.querySelector("[data-testid='confirm-title']") as HTMLElement | null)?.textContent).toBe("Unsaved Changes");
        expect((container.querySelector("[data-testid='confirm-message']") as HTMLElement | null)?.textContent).toBe("Leave without saving your task scheduler changes?");
        expect((container.querySelector("[data-testid='discard-changes']") as HTMLElement | null)?.textContent).toBe("Discard Changes");
    });

    it("reinitializes the form when the same template is reopened for another pane", async () => {
        await act(async () => {
            root.render(<TaskSchedulerView/>);
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext("Review Template", "template:alpha"),
            });
            await Promise.resolve();
            await Promise.resolve();
        });

        expect((container.querySelector("[data-testid='form-pane-value']") as HTMLElement | null)?.textContent).toBe("%1");

        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: {
                    ...createTemplateContext("Review Template", "template:beta"),
                    targetPaneID: "%2",
                },
            });
            await Promise.resolve();
            await Promise.resolve();
        });

        expect((container.querySelector("[data-testid='form-pane-value']") as HTMLElement | null)?.textContent).toBe("%2");
    });

    it("keeps dirty settings guarded when the deferred palette readiness check rejects", async () => {
        checkOrchestratorReadyMock.mockRejectedValueOnce(new Error("boom"));

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-settings']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext("Rejected Template", "template:reject"),
            });
            await Promise.resolve();
        });

        await act(async () => {
            (container.querySelector("[data-testid='discard-changes']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(container.querySelector("[data-testid='task-form']")).toBeNull();

        await act(async () => {
            (container.querySelector("[data-testid='settings-back']") as HTMLButtonElement | null)?.click();
        });

        expect(container.querySelector("[data-testid='confirm-dialog']")).not.toBeNull();
    });

    it("keeps dirty settings guarded when a deferred palette readiness result is ignored after a session switch", async () => {
        const deferred = createDeferred<{ready: boolean}>();
        checkOrchestratorReadyMock.mockImplementationOnce(() => deferred.promise);

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-settings']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            useViewerStore.setState({
                activeViewId: "task-scheduler",
                viewContext: createTemplateContext("Deferred Template", "template:deferred"),
            });
            await Promise.resolve();
        });

        await act(async () => {
            (container.querySelector("[data-testid='discard-changes']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
        });

        currentSessionKey = "beta:2";

        await act(async () => {
            deferred.resolve({ready: true});
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(container.querySelector("[data-testid='task-form']")).toBeNull();

        await act(async () => {
            (container.querySelector("[data-testid='settings-back']") as HTMLButtonElement | null)?.click();
        });

        expect(container.querySelector("[data-testid='confirm-dialog']")).not.toBeNull();
    });

    it("keeps only the latest same-session edit readiness request", async () => {
        const firstRequest = createDeferred<{ready: boolean}>();
        const secondRequest = createDeferred<{ready: boolean}>();
        currentStatus = {
            items: [
                {id: "item-1", status: "completed"},
                {id: "item-2", status: "completed"},
            ],
            config: {},
        };
        checkOrchestratorReadyMock
            .mockImplementationOnce(() => firstRequest.promise)
            .mockImplementationOnce(() => secondRequest.promise);

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='edit-item-1']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='edit-item-2']") as HTMLButtonElement | null)?.click();
        });

        await act(async () => {
            secondRequest.resolve({ready: true});
            await Promise.resolve();
            await Promise.resolve();
        });

        await act(async () => {
            firstRequest.resolve({ready: true});
            await Promise.resolve();
            await Promise.resolve();
        });

        expect((container.querySelector("[data-testid='form-editing-id']") as HTMLElement | null)?.textContent).toBe("item-2");
    });

    it("confirms before closing dirty settings", async () => {
        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-settings']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='mark-dirty']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='shell-close']") as HTMLButtonElement | null)?.click();
        });

        expect(container.querySelector("[data-testid='confirm-dialog']")).not.toBeNull();

        await act(async () => {
            (container.querySelector("[data-testid='discard-changes']") as HTMLButtonElement | null)?.click();
            await Promise.resolve();
        });

        expect(useViewerStore.getState().activeViewId).toBeNull();
    });

    it("shows an error instead of silently starting without an active session", async () => {
        currentSessionKey = "";

        await act(async () => {
            root.render(<TaskSchedulerView/>);
        });

        await act(async () => {
            (container.querySelector("[data-testid='open-config']") as HTMLButtonElement | null)?.click();
        });
        await act(async () => {
            (container.querySelector("[data-testid='config-start']") as HTMLButtonElement | null)?.click();
        });

        expect(startMock).not.toHaveBeenCalled();
        expect(setErrorMock).toHaveBeenLastCalledWith("アクティブなセッションがありません");
    });
});
