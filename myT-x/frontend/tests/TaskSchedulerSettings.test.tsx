import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const apiMock = vi.hoisted(() => ({
    GetValidationRules: vi.fn<() => Promise<unknown>>(),
}));

vi.mock("../src/api", () => ({
    api: {
        GetValidationRules: () => apiMock.GetValidationRules(),
    },
}));

vi.mock("../wailsjs/go/models", () => ({
    config: {
        TaskSchedulerConfig: class {
            pre_exec_reset_delay_s: number;
            pre_exec_idle_timeout_s: number;
            pre_exec_target_mode: string;
            message_templates: Array<{name: string; message: string}>;
            constructor(data: Record<string, unknown>) {
                this.pre_exec_reset_delay_s = data.pre_exec_reset_delay_s as number;
                this.pre_exec_idle_timeout_s = data.pre_exec_idle_timeout_s as number;
                this.pre_exec_target_mode = data.pre_exec_target_mode as string;
                this.message_templates = data.message_templates as Array<{name: string; message: string}>;
            }
        },
    },
}));

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: (_key: string, fallback: string) => fallback,
    }),
}));

import {TaskSchedulerSettingsPanel} from "../src/components/viewer/views/task-scheduler/TaskSchedulerSettings";

function makeSettings(overrides: Record<string, unknown> = {}) {
    return {
        pre_exec_reset_delay_s: 5,
        pre_exec_idle_timeout_s: 30,
        pre_exec_target_mode: "task_panes",
        message_templates: [],
        ...overrides,
    };
}

describe("TaskSchedulerSettingsPanel", () => {
    let container: HTMLDivElement;
    let root: Root;
    let dirtyState: boolean;
    let lastSavedSettings: Record<string, unknown> | null;
    let lastErrorMessage: string | null;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        dirtyState = false;
        lastSavedSettings = null;
        lastErrorMessage = null;
        apiMock.GetValidationRules.mockResolvedValue(null);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    function renderPanel(initialSettings: ReturnType<typeof makeSettings> | null) {
        act(() => {
            root.render(
                <TaskSchedulerSettingsPanel
                    initialSettings={initialSettings}
                    onSave={async (settings) => {
                        lastSavedSettings = settings as unknown as Record<string, unknown>;
                        return true;
                    }}
                    onError={(message) => {
                        lastErrorMessage = message;
                    }}
                    onBack={() => {}}
                    onDirtyChange={(dirty) => {
                        dirtyState = dirty;
                    }}
                />,
            );
        });
    }

    function getResetDelayInput(): HTMLInputElement {
        const inputs = container.querySelectorAll<HTMLInputElement>(".task-scheduler-config-input");
        return inputs[0]!;
    }

    function getIdleTimeoutInput(): HTMLInputElement {
        const inputs = container.querySelectorAll<HTMLInputElement>(".task-scheduler-config-input");
        return inputs[1]!;
    }

    it("advances the baseline when initialSettings changes during a dirty edit so dirty clears when edits match new backend state", async () => {
        const original = makeSettings({pre_exec_reset_delay_s: 5});
        renderPanel(original);
        await act(async () => {});

        // Simulate user editing the delay to 10 → dirty=true
        const input = getResetDelayInput();
        act(() => {
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
                HTMLInputElement.prototype,
                "value",
            )?.set;
            nativeInputValueSetter?.call(input, "10");
            input.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(dirtyState).toBe(true);

        // Backend updates settings to match what the user typed (delay=10)
        const updatedSettings = makeSettings({pre_exec_reset_delay_s: 10});
        renderPanel(updatedSettings);
        await act(async () => {});

        // Dirty should clear: user's draft now matches the new baseline
        expect(dirtyState).toBe(false);
    });

    it("stays dirty when initialSettings changes but user edits differ from new backend state", async () => {
        const original = makeSettings({pre_exec_reset_delay_s: 5});
        renderPanel(original);
        await act(async () => {});

        // Simulate user editing the delay to 15
        const input = getResetDelayInput();
        act(() => {
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
                HTMLInputElement.prototype,
                "value",
            )?.set;
            nativeInputValueSetter?.call(input, "15");
            input.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(dirtyState).toBe(true);

        // Backend updates settings to 10 (different from user's 15)
        const updatedSettings = makeSettings({pre_exec_reset_delay_s: 10});
        renderPanel(updatedSettings);
        await act(async () => {});

        // Dirty should remain: user's 15 !== new baseline 10
        expect(dirtyState).toBe(true);
    });

    it("re-applies stricter backend bounds to invalid initial values after validation rules load", async () => {
        apiMock.GetValidationRules.mockResolvedValue({
            min_pre_exec_reset_delay: 5,
            max_pre_exec_reset_delay: 9,
            min_pre_exec_idle_timeout: 45,
            max_pre_exec_idle_timeout: 99,
        });

        renderPanel(makeSettings({
            pre_exec_reset_delay_s: undefined,
            pre_exec_idle_timeout_s: 0,
        }));
        await act(async () => {});

        expect(getResetDelayInput().value).toBe("5");
        expect(getIdleTimeoutInput().value).toBe("45");
    });

    it("submits fallback values clamped to the loaded backend rules", async () => {
        apiMock.GetValidationRules.mockResolvedValue({
            min_pre_exec_reset_delay: 5,
            max_pre_exec_reset_delay: 9,
            min_pre_exec_idle_timeout: 45,
            max_pre_exec_idle_timeout: 99,
        });

        renderPanel(makeSettings({
            pre_exec_reset_delay_s: undefined,
            pre_exec_idle_timeout_s: 0,
        }));
        await act(async () => {});

        const saveButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Save");
        await act(async () => {
            saveButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(lastSavedSettings).not.toBeNull();
        expect(lastSavedSettings?.pre_exec_reset_delay_s).toBe(5);
        expect(lastSavedSettings?.pre_exec_idle_timeout_s).toBe(45);
    });

    it("keeps the template editor open when clean settings refresh arrives mid-edit", async () => {
        renderPanel(makeSettings({
            message_templates: [{name: "Reminder", message: "Ping"}],
        }));
        await act(async () => {});

        const addButton = container.querySelector(".task-scheduler-template-add-btn") as HTMLButtonElement | null;
        expect(addButton).not.toBeNull();
        act(() => {
            addButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector(".task-scheduler-template-form")).not.toBeNull();

        renderPanel(makeSettings({
            message_templates: [{name: "Reminder", message: "Ping again"}],
        }));
        await act(async () => {});

        expect(container.querySelector(".task-scheduler-template-form")).not.toBeNull();
        expect(container.querySelector(".task-scheduler-template-add-btn")).toBeNull();
    });

    it("surfaces a warning when validation rules fall back", async () => {
        apiMock.GetValidationRules.mockRejectedValue(new Error("rules unavailable"));

        renderPanel(makeSettings());
        await act(async () => {});

        expect(lastErrorMessage).toBe("Validation rules are unavailable; using fallback limits.");
    });
});
