import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../../../../i18n";
import {singletaskrunner} from "../../../../../wailsjs/go/models";
import {SingleTaskRunnerList} from "./SingleTaskRunnerList";
import type {QueueItem, QueueStatus} from "./useSingleTaskRunner";

const getValidationRulesMock = vi.fn();

vi.mock("../../../../api", () => ({
    api: {
        GetValidationRules: () => getValidationRulesMock(),
    },
}));

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

function buildItem(overrides: Partial<QueueItem> = {}): QueueItem {
    return singletaskrunner.QueueItem.createFrom({
        id: "item-1",
        title: "Review task",
        message: "Fix this",
        target_pane_id: "%1",
        order_index: 0,
        status: "pending",
        created_at: "",
        started_at: undefined,
        completed_at: undefined,
        error_message: undefined,
        result_message: undefined,
        clear_before: false,
        clear_command: "",
        ...overrides,
    });
}

function buildStatus(items: QueueItem[]): QueueStatus {
    return singletaskrunner.QueueStatus.createFrom({
        run_status: "idle",
        current_index: -1,
        session_name: "alpha",
        generation_id: "gen-1",
        clear_delay_sec: 1,
        last_stop_reason: undefined,
        items,
    });
}

describe("SingleTaskRunnerList", () => {
    let container: HTMLDivElement;
    let root: Root;
    let onToggleClearBefore: ReturnType<typeof vi.fn<(item: QueueItem, clearBefore: boolean) => Promise<boolean>>>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
        setLanguage("en");
        getValidationRulesMock.mockReset().mockResolvedValue({
            min_single_task_runner_clear_delay: 0,
            max_single_task_runner_clear_delay: 300,
        });
        onToggleClearBefore = vi.fn<(item: QueueItem, clearBefore: boolean) => Promise<boolean>>().mockResolvedValue(true);
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        setLanguage("ja");
        container.remove();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    function renderList(status: QueueStatus) {
        root.render(
            <SingleTaskRunnerList
                defaultClearDelay={1}
                status={status}
                onNew={vi.fn()}
                onEdit={vi.fn()}
                onRemove={vi.fn()}
                onStart={vi.fn()}
                onStop={vi.fn()}
                onSetClearDelay={vi.fn().mockResolvedValue(true)}
                onToggleClearBefore={onToggleClearBefore}
                onError={vi.fn()}
            />,
        );
    }

    it("shows clear state for every task", async () => {
        await act(async () => {
            renderList(buildStatus([
                buildItem({id: "clear-off", title: "Clear off", clear_before: false}),
                buildItem({id: "clear-on", title: "Clear on", clear_before: true, clear_command: "/reset"}),
            ]));
        });

        const labels = Array.from(container.querySelectorAll(".single-task-runner-card-clear-toggle"));
        expect(labels.map((label) => label.textContent)).toEqual(["Clear disabled", "Clear enabled: /reset"]);
        const checkboxes = Array.from(container.querySelectorAll(".single-task-runner-card-clear-toggle input"));
        expect((checkboxes[0] as HTMLInputElement | undefined)?.checked).toBe(false);
        expect((checkboxes[1] as HTMLInputElement | undefined)?.checked).toBe(true);
    });

    it("allows clear toggling only for editable tasks", async () => {
        const editableItem = buildItem({id: "editable", title: "Editable", status: "pending"});
        const lockedItem = buildItem({id: "locked", title: "Locked", status: "active", clear_before: true});

        await act(async () => {
            renderList(buildStatus([editableItem, lockedItem]));
        });

        const checkboxes = Array.from(container.querySelectorAll(".single-task-runner-card-clear-toggle input")) as HTMLInputElement[];
        expect(checkboxes).toHaveLength(2);
        expect(checkboxes[0]?.disabled).toBe(false);
        expect(checkboxes[1]?.disabled).toBe(true);

        await act(async () => {
            checkboxes[0]?.click();
        });

        expect(onToggleClearBefore).toHaveBeenCalledTimes(1);
        expect(onToggleClearBefore).toHaveBeenCalledWith(editableItem, true);
    });

    it("fails closed when clear toggle item status is missing", async () => {
        const malformedItem = buildItem({id: "malformed", title: "Malformed"}) as unknown as Record<string, unknown>;
        delete malformedItem.status;

        await act(async () => {
            renderList(buildStatus([malformedItem as unknown as QueueItem]));
        });

        const checkbox = container.querySelector(".single-task-runner-card-clear-toggle input") as HTMLInputElement | null;
        expect(checkbox).not.toBeNull();
        expect(checkbox?.disabled).toBe(true);

        await act(async () => {
            checkbox?.click();
        });

        expect(onToggleClearBefore).not.toHaveBeenCalled();
    });

    it("fails closed when clear toggle item status is unknown", async () => {
        const malformedItem = buildItem({id: "unknown", title: "Unknown", status: "unexpected"});

        await act(async () => {
            renderList(buildStatus([malformedItem]));
        });

        const checkbox = container.querySelector(".single-task-runner-card-clear-toggle input") as HTMLInputElement | null;
        expect(checkbox).not.toBeNull();
        expect(checkbox?.disabled).toBe(true);

        await act(async () => {
            checkbox?.click();
        });

        expect(onToggleClearBefore).not.toHaveBeenCalled();
    });

    it("disables a clear toggle while its update is in flight", async () => {
        const request = deferred<boolean>();
        onToggleClearBefore.mockReturnValueOnce(request.promise);
        const editableItem = buildItem({id: "editable", title: "Editable", status: "pending"});

        await act(async () => {
            renderList(buildStatus([editableItem]));
        });

        const checkbox = container.querySelector(".single-task-runner-card-clear-toggle input") as HTMLInputElement | null;
        expect(checkbox).not.toBeNull();

        await act(async () => {
            checkbox?.click();
            await Promise.resolve();
        });

        expect(onToggleClearBefore).toHaveBeenCalledTimes(1);
        expect(checkbox?.disabled).toBe(true);
        expect(container.querySelector(".single-task-runner-card-clear-toggle")?.textContent).toBe("Updating...");

        await act(async () => {
            request.resolve(true);
            await Promise.resolve();
        });

        expect(checkbox?.disabled).toBe(false);
    });
});
