import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    t: vi.fn((_: string, fallback: string) => fallback),
}));

const apiMock = vi.hoisted(() => ({
    GetValidationRules: vi.fn<() => Promise<unknown>>(),
}));

const defaultValidationRules = {
    min_override_name_len: 1,
    min_pre_exec_reset_delay: 0,
    max_pre_exec_reset_delay: 60,
    min_pre_exec_idle_timeout: 0,
    max_pre_exec_idle_timeout: 600,
    max_message_templates: 50,
    max_template_name_len: 80,
    max_template_message_len: 4000,
    min_single_task_runner_clear_delay: 0,
    max_single_task_runner_clear_delay: 300,
    min_chat_overlay_percentage: 20,
    max_chat_overlay_percentage: 80,
    default_chat_overlay_percentage: 50,
};

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

import {SingleTaskRunnerForm} from "../src/components/viewer/views/single-task-runner/SingleTaskRunnerForm";
import {SingleTaskRunnerList} from "../src/components/viewer/views/single-task-runner/SingleTaskRunnerList";
import type {QueueItem, QueueStatus} from "../src/components/viewer/views/single-task-runner/useSingleTaskRunner";
import type {PaneSnapshot} from "../src/types/tmux";

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("single task runner components", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.t.mockImplementation((_: string, fallback: string) => fallback);
        apiMock.GetValidationRules.mockReset();
        apiMock.GetValidationRules.mockResolvedValue(defaultValidationRules);
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

    it("SingleTaskRunnerList hides edit controls for malformed item payloads", async () => {
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
            clear_delay_sec: 2,
        } as QueueStatus;

        act(() => {
            root.render(
                <SingleTaskRunnerList
                    defaultClearDelay={2}
                    status={status}
                    onNew={() => undefined}
                    onEdit={() => undefined}
                    onRemove={async () => undefined}
                    onStart={async () => true}
                    onStop={async () => undefined}
                    onSetClearDelay={async () => true}
                    onError={() => undefined}
                />,
            );
        });
        await flushEffects();

        expect(container.textContent).not.toContain("Edit");
        expect(container.textContent).not.toContain("Remove");
    });

    it("SingleTaskRunnerForm disables submit when the editing item is not editable", () => {
        const onSave = vi.fn(async () => true);
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
                <SingleTaskRunnerForm
                    availablePanes={[availablePane]}
                    editingItem={editingItem}
                    onSave={onSave}
                    onBack={() => undefined}
                />,
            );
        });

        const submitButton = container.querySelector(".single-task-runner-submit-btn") as HTMLButtonElement | null;
        expect(submitButton).not.toBeNull();
        expect(submitButton?.disabled).toBe(true);

        act(() => {
            submitButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        expect(onSave).not.toHaveBeenCalled();
    });

    it("SingleTaskRunnerList restores the saved delay when updating the delay throws", async () => {
        const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);
        const onError = vi.fn();
        const status = {
            items: [],
            run_status: "idle",
            current_index: -1,
            session_name: "session-a",
            generation_id: "gen-a",
            clear_delay_sec: 12,
        } as QueueStatus;

        act(() => {
            root.render(
                <SingleTaskRunnerList
                    defaultClearDelay={12}
                    status={status}
                    onNew={() => undefined}
                    onEdit={() => undefined}
                    onRemove={async () => undefined}
                    onStart={async () => true}
                    onStop={async () => undefined}
                    onSetClearDelay={async () => {
                        throw new Error("update failed");
                    }}
                    onError={onError}
                />,
            );
        });
        await flushEffects();

        const input = container.querySelector("#single-task-runner-clear-delay") as HTMLInputElement | null;
        expect(input).not.toBeNull();
        act(() => {
            const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
            descriptor?.set?.call(input, "99");
            input?.dispatchEvent(new Event("input", {bubbles: true}));
            input?.dispatchEvent(new Event("change", {bubbles: true}));
        });

        await act(async () => {
            input?.dispatchEvent(new FocusEvent("blur", {bubbles: true}));
            input?.dispatchEvent(new FocusEvent("focusout", {bubbles: true}));
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(input?.value).toBe("12");
        expect(warnSpy).toHaveBeenCalledWith(
            "[single-task-runner] failed to update clear delay",
            expect.any(Error),
        );
        expect(onError).toHaveBeenCalledWith("update failed");
    });
});
