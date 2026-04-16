import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    t: vi.fn((_: string, fallback: string) => fallback),
}));

const apiMock = vi.hoisted(() => ({
    GetValidationRules: vi.fn<() => Promise<unknown>>(),
}));

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: mocked.t,
    }),
}));

vi.mock("../src/api", () => ({
    api: {
        GetValidationRules: () => apiMock.GetValidationRules(),
    },
}));

import {TaskSchedulerConfig} from "../src/components/viewer/views/task-scheduler/TaskSchedulerConfig";
import {TaskSchedulerForm} from "../src/components/viewer/views/task-scheduler/TaskSchedulerForm";
import {TaskSchedulerList} from "../src/components/viewer/views/task-scheduler/TaskSchedulerList";
import {TaskSchedulerSettingsPanel} from "../src/components/viewer/views/task-scheduler/TaskSchedulerSettings";
import type {QueueItem, QueueStatus, TaskSchedulerSettings} from "../src/components/viewer/views/task-scheduler/useTaskScheduler";
import type {PaneSnapshot} from "../src/types/tmux";

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

function setFieldValue(element: HTMLInputElement | HTMLTextAreaElement, value: string): void {
    const prototype = element instanceof HTMLTextAreaElement
        ? HTMLTextAreaElement.prototype
        : HTMLInputElement.prototype;
    const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
    if (!descriptor?.set) {
        throw new Error("value setter is unavailable");
    }

    act(() => {
        descriptor.set.call(element, value);
        element.dispatchEvent(new Event("input", {bubbles: true}));
        element.dispatchEvent(new Event("change", {bubbles: true}));
    });
}

describe("task scheduler components", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.t.mockImplementation((_: string, fallback: string) => fallback);
        apiMock.GetValidationRules.mockReset();
        apiMock.GetValidationRules.mockResolvedValue(null);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("TaskSchedulerConfig blocks pre-exec start until saved settings are loaded", () => {
        const onStart = vi.fn(async (_config: unknown, _items: QueueItem[]) => undefined);
        const pendingItem = {id: "task-1", status: "pending"} as QueueItem;

        act(() => {
            root.render(
                <TaskSchedulerConfig
                    items={[pendingItem]}
                    initialConfig={null}
                    savedSettings={null}
                    onStart={onStart}
                    onBack={() => undefined}
                    onOpenSettings={() => undefined}
                />,
            );
        });

        const checkbox = container.querySelector("input[type=\"checkbox\"]");
        expect(checkbox).not.toBeNull();
        act(() => {
            checkbox?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const submitButton = container.querySelector(".task-scheduler-submit-btn") as HTMLButtonElement | null;
        expect(submitButton).not.toBeNull();
        expect(submitButton?.disabled).toBe(true);
        expect(container.textContent).toContain("Loading scheduler settings...");

        act(() => {
            submitButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        expect(onStart).not.toHaveBeenCalled();
    });

    it("TaskSchedulerSettingsPanel syncs local draft values when async settings arrive", async () => {
        const initialSettings = {
            pre_exec_reset_delay_s: 0,
            pre_exec_idle_timeout_s: 30,
            pre_exec_target_mode: "task_panes",
            message_templates: [],
        } as TaskSchedulerSettings;
        const updatedSettings = {
            pre_exec_reset_delay_s: 12,
            pre_exec_idle_timeout_s: 90,
            pre_exec_target_mode: "all_panes",
            message_templates: [{name: "Reminder", message: "Do the thing"}],
        } as TaskSchedulerSettings;

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={initialSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        let inputs = Array.from(container.querySelectorAll(".task-scheduler-config-input")) as HTMLInputElement[];
        expect(inputs).toHaveLength(2);
        expect(inputs[0]?.value).toBe("0");
        expect(inputs[1]?.value).toBe("30");
        expect(container.textContent).not.toContain("Reminder");

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={updatedSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        inputs = Array.from(container.querySelectorAll(".task-scheduler-config-input")) as HTMLInputElement[];
        expect(inputs[0]?.value).toBe("12");
        expect(inputs[1]?.value).toBe("90");
        expect(container.textContent).toContain("Reminder");
    });

    it("TaskSchedulerSettingsPanel requests validation rules and applies returned bounds", async () => {
        apiMock.GetValidationRules.mockResolvedValue({
            min_pre_exec_reset_delay: 5,
            max_pre_exec_reset_delay: 9,
            min_pre_exec_idle_timeout: 45,
            max_pre_exec_idle_timeout: 99,
        });

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={{
                        pre_exec_reset_delay_s: 0,
                        pre_exec_idle_timeout_s: 0,
                        pre_exec_target_mode: "task_panes",
                        message_templates: [],
                    } as TaskSchedulerSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        const inputs = Array.from(container.querySelectorAll(".task-scheduler-config-input")) as HTMLInputElement[];
        expect(apiMock.GetValidationRules).toHaveBeenCalledTimes(1);
        expect(inputs[0]?.value).toBe("5");
        expect(inputs[1]?.value).toBe("45");
    });

    it("TaskSchedulerSettingsPanel preserves dirty draft values when external settings refresh", async () => {
        const initialSettings = {
            pre_exec_reset_delay_s: 0,
            pre_exec_idle_timeout_s: 30,
            pre_exec_target_mode: "task_panes",
            message_templates: [],
        } as TaskSchedulerSettings;
        const updatedSettings = {
            pre_exec_reset_delay_s: 12,
            pre_exec_idle_timeout_s: 90,
            pre_exec_target_mode: "all_panes",
            message_templates: [{name: "Reminder", message: "Do the thing"}],
        } as TaskSchedulerSettings;

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={initialSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        const inputs = Array.from(container.querySelectorAll(".task-scheduler-config-input")) as HTMLInputElement[];
        expect(inputs).toHaveLength(2);
        setFieldValue(inputs[0]!, "18");
        expect(inputs[0]?.value).toBe("18");

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={updatedSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        const rerenderedInputs = Array.from(container.querySelectorAll(".task-scheduler-config-input")) as HTMLInputElement[];
        expect(rerenderedInputs[0]?.value).toBe("18");
        expect(rerenderedInputs[1]?.value).toBe("30");
        expect(container.textContent).not.toContain("Reminder");
    });

    it("TaskSchedulerSettingsPanel stays clean until backend settings are available", async () => {
        const onDirtyChange = vi.fn();
        const loadedSettings = {
            pre_exec_reset_delay_s: 5,
            pre_exec_idle_timeout_s: 30,
            pre_exec_target_mode: "task_panes",
            message_templates: [],
        } as TaskSchedulerSettings;

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={null}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                    onDirtyChange={onDirtyChange}
                />,
            );
        });
        await flushEffects();

        expect(onDirtyChange).toHaveBeenLastCalledWith(false);

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={loadedSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                    onDirtyChange={onDirtyChange}
                />,
            );
        });
        await flushEffects();

        const inputs = Array.from(container.querySelectorAll(".task-scheduler-config-input")) as HTMLInputElement[];
        expect(inputs).toHaveLength(2);
        setFieldValue(inputs[0]!, "9");
        expect(onDirtyChange).toHaveBeenLastCalledWith(true);
    });

    it("TaskSchedulerSettingsPanel blocks save until the initial settings snapshot is loaded", async () => {
        const onSave = vi.fn(async () => true);

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={null}
                    onSave={onSave}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        const submitButton = container.querySelector(".task-scheduler-submit-btn") as HTMLButtonElement | null;
        expect(submitButton).not.toBeNull();
        expect(submitButton?.disabled).toBe(true);
        expect(container.textContent).toContain("Loading scheduler settings...");
        expect(container.querySelector(".task-scheduler-config-input")).toBeNull();

        act(() => {
            submitButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(onSave).not.toHaveBeenCalled();
    });

    it("TaskSchedulerSettingsPanel blocks duplicate template names before save", async () => {
        const initialSettings = {
            pre_exec_reset_delay_s: 0,
            pre_exec_idle_timeout_s: 30,
            pre_exec_target_mode: "task_panes",
            message_templates: [{name: "Reminder", message: "Do the thing"}],
        } as TaskSchedulerSettings;

        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={initialSettings}
                    onSave={async () => true}
                    onError={() => undefined}
                    onBack={() => undefined}
                />,
            );
        });
        await flushEffects();

        const addButton = container.querySelector(".task-scheduler-template-add-btn") as HTMLButtonElement | null;
        expect(addButton).not.toBeNull();
        act(() => {
            addButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const nameInput = container.querySelector(".task-scheduler-template-form input") as HTMLInputElement | null;
        const messageInput = container.querySelector(".task-scheduler-template-form textarea") as HTMLTextAreaElement | null;
        const saveButton = container.querySelector(".task-scheduler-template-save-btn") as HTMLButtonElement | null;
        expect(nameInput).not.toBeNull();
        expect(messageInput).not.toBeNull();
        expect(saveButton).not.toBeNull();

        setFieldValue(nameInput!, "Reminder");
        setFieldValue(messageInput!, "Something else");

        expect(saveButton?.disabled).toBe(true);
        expect(container.textContent).toContain("A template with this name already exists.");
    });

    it("TaskSchedulerList hides edit controls for malformed item payloads", () => {
        const status = {
            items: [{
                id: "task-1",
                title: "Task",
                message: "Message",
                target_pane_id: "%1",
                order_index: 0,
                created_at: "2026-04-10T00:00:00Z",
                clear_before: false,
            }],
            run_status: "idle",
            current_index: -1,
            session_name: "session-a",
            generation_id: "gen-a",
        } as QueueStatus;

        act(() => {
            root.render(
                <TaskSchedulerList
                    status={status}
                    onNew={() => undefined}
                    onEdit={() => undefined}
                    onRemove={async () => undefined}
                    onStart={() => undefined}
                    onStop={async () => undefined}
                    onPause={async () => undefined}
                    onResume={async () => undefined}
                    onSettings={() => undefined}
                    isRunning={false}
                />,
            );
        });

        expect(container.textContent).not.toContain("Edit");
        expect(container.textContent).not.toContain("Remove");
    });

    it("TaskSchedulerForm disables submit when the editing item is not editable", () => {
        const onSave = vi.fn(async () => undefined);
        const availablePane: PaneSnapshot = {id: "%1", index: 0, active: true, width: 80, height: 24};
        const editingItem = {
            id: "task-1",
            title: "Locked",
            message: "Message",
            target_pane_id: "%1",
            order_index: 0,
            created_at: "2026-04-10T00:00:00Z",
            clear_before: false,
        } as QueueItem;

        act(() => {
            root.render(
                <TaskSchedulerForm
                    availablePanes={[availablePane]}
                    messageTemplates={[]}
                    editingItem={editingItem}
                    onSave={onSave}
                    onBack={() => undefined}
                />,
            );
        });

        const submitButton = container.querySelector(".task-scheduler-submit-btn") as HTMLButtonElement | null;
        expect(submitButton).not.toBeNull();
        expect(submitButton?.disabled).toBe(true);

        act(() => {
            submitButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        expect(onSave).not.toHaveBeenCalled();
    });
});
