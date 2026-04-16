import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const closeViewMock = vi.fn();
const setErrorMock = vi.fn();

const hookState = {
    status: null,
    error: "Failed to update clear delay",
    setError: setErrorMock,
    availablePanes: [],
    defaultClearDelay: 2,
    start: vi.fn(async () => true),
    stop: vi.fn(async () => undefined),
    addItem: vi.fn(async () => true),
    removeItem: vi.fn(async () => undefined),
    updateItem: vi.fn(async () => true),
    reorderItems: vi.fn(async () => undefined),
    setClearDelay: vi.fn(async () => true),
    refreshStatus: vi.fn(async () => true),
};

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: (_key: string, fallback: string) => fallback,
    }),
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {activeSession: string | null}) => unknown) =>
        selector({activeSession: "session-a"}),
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {closeView: () => void}) => unknown) =>
        selector({closeView: closeViewMock}),
}));

vi.mock("../src/components/viewer/views/shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({
        children,
        onClose,
    }: {
        children: unknown;
        onClose: () => void;
    }) => (
        <div>
            <button type="button" data-testid="shell-close" onClick={onClose}>Close Panel</button>
            {children}
        </div>
    ),
}));

vi.mock("../src/components/viewer/views/single-task-runner/SingleTaskRunnerList", () => ({
    SingleTaskRunnerList: ({
        onNew,
        onEdit,
        status,
    }: {
        onNew: () => void;
        onEdit: (id: string) => void;
        status: {items: Array<{id: string}>} | null;
    }) => (
        <div data-testid="single-task-runner-list">
            <span>{status === null ? "loading" : `items:${status.items.length}`}</span>
            <button type="button" data-testid="single-task-runner-new" onClick={onNew}>New</button>
            <button
                type="button"
                data-testid="single-task-runner-edit"
                onClick={() => onEdit(status?.items[0]?.id ?? "item-1")}
            >
                Edit
            </button>
        </div>
    ),
}));

vi.mock("../src/components/viewer/views/single-task-runner/SingleTaskRunnerForm", () => ({
    SingleTaskRunnerForm: ({onBack}: {onBack: () => void}) => (
        <div data-testid="single-task-runner-form">
            <button type="button" data-testid="single-task-runner-back" onClick={onBack}>Back</button>
            Form
        </div>
    ),
}));

vi.mock("../src/components/viewer/views/single-task-runner/useSingleTaskRunner", () => ({
    useSingleTaskRunner: () => hookState,
}));

import {SingleTaskRunnerView} from "../src/components/viewer/views/single-task-runner/SingleTaskRunnerView";

describe("SingleTaskRunnerView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        closeViewMock.mockReset();
        setErrorMock.mockReset();
        hookState.status = null;
        hookState.error = "Failed to update clear delay";
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders the hook error banner and dismisses it through setError", () => {
        act(() => {
            root.render(<SingleTaskRunnerView/>);
        });

        expect(container.textContent).toContain("Failed to update clear delay");

        const dismissButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Dismiss");
        expect(dismissButton).toBeDefined();

        act(() => {
            dismissButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(setErrorMock).toHaveBeenCalledWith(null);
    });

    it("renders the list screen while the hook is still loading", () => {
        hookState.error = null;
        hookState.status = null;

        act(() => {
            root.render(<SingleTaskRunnerView/>);
        });

        expect(container.querySelector("[data-testid='single-task-runner-list']")).not.toBeNull();
        expect(container.textContent).toContain("loading");
    });

    it("switches between the list and form screens", () => {
        hookState.error = null;
        hookState.status = {
            run_status: "idle",
            current_index: 0,
            session_name: "session-a",
            generation_id: "gen-1",
            clear_delay_sec: 2,
            last_stop_reason: "",
            items: [{
                id: "item-1",
                title: "Title",
                message: "Message",
                target_pane_id: "%1",
                order_index: 0,
                status: "pending",
                created_at: "2026-04-12T00:00:00Z",
                clear_before: false,
                clear_command: "",
            }],
        };

        act(() => {
            root.render(<SingleTaskRunnerView/>);
        });

        const newButton = container.querySelector("[data-testid='single-task-runner-new']") as HTMLButtonElement;
        act(() => {
            newButton.click();
        });
        expect(container.querySelector("[data-testid='single-task-runner-form']")).not.toBeNull();

        const backButton = container.querySelector("[data-testid='single-task-runner-back']") as HTMLButtonElement;
        act(() => {
            backButton.click();
        });
        expect(container.querySelector("[data-testid='single-task-runner-list']")).not.toBeNull();
        expect(container.textContent).toContain("items:1");
    });
});
