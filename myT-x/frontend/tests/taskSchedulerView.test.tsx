import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const closeViewMock = vi.fn();
const openViewWithContextMock = vi.fn();

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: (_key: string, fallback: string) => fallback,
    }),
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {
        closeView: () => void;
        openViewWithContext: (...args: unknown[]) => void;
    }) => unknown) => selector({
        closeView: closeViewMock,
        openViewWithContext: openViewWithContextMock,
    }),
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

vi.mock("../src/components/viewer/views/task-scheduler/TaskSchedulerList", () => ({
    TaskSchedulerList: ({onSettings}: { onSettings: () => void }) => (
        <button type="button" data-testid="open-settings" onClick={onSettings}>Open Settings</button>
    ),
}));

vi.mock("../src/components/viewer/views/task-scheduler/TaskSchedulerSettings", async () => {
    const React = await import("react");
    return {
        TaskSchedulerSettingsPanel: ({
                                         onBack,
                                         onDirtyChange,
                                     }: {
            onBack: () => void;
            onDirtyChange?: (dirty: boolean) => void;
        }) => {
            React.useEffect(() => {
                onDirtyChange?.(true);
            }, [onDirtyChange]);
            return (
                <button type="button" data-testid="settings-back" onClick={onBack}>
                    Back From Settings
                </button>
            );
        },
    };
});

vi.mock("../src/components/viewer/views/task-scheduler/useTaskScheduler", () => ({
    isActiveQueueStatus: () => false,
    PENDING_ITEM_STATUS: "pending",
    useTaskScheduler: () => ({
        status: null,
        error: null,
        setError: vi.fn(),
        checkOrchestratorReady: vi.fn(async () => ({ready: true})),
        availablePanes: [],
        start: vi.fn(async () => true),
        stop: vi.fn(async () => true),
        pause: vi.fn(async () => true),
        resume: vi.fn(async () => true),
        addItem: vi.fn(async () => true),
        updateItem: vi.fn(async () => true),
        removeItem: vi.fn(async () => true),
        refreshStatus: vi.fn(async () => true),
        settings: null,
        saveSettings: vi.fn(async () => true),
    }),
}));

import {TaskSchedulerView} from "../src/components/viewer/views/task-scheduler/TaskSchedulerView";

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("TaskSchedulerView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        closeViewMock.mockReset();
        openViewWithContextMock.mockReset();
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("confirms before navigating back from dirty settings", async () => {
        act(() => {
            root.render(<TaskSchedulerView/>);
        });

        const openSettingsButton = container.querySelector("[data-testid='open-settings']") as HTMLButtonElement | null;
        expect(openSettingsButton).not.toBeNull();
        act(() => {
            openSettingsButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await flushEffects();

        const backButton = container.querySelector("[data-testid='settings-back']") as HTMLButtonElement | null;
        expect(backButton).not.toBeNull();
        act(() => {
            backButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.textContent).toContain("Unsaved Settings");
        expect(closeViewMock).not.toHaveBeenCalled();

        const discardButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Discard Changes");
        expect(discardButton).toBeDefined();
        act(() => {
            discardButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector("[data-testid='open-settings']")).not.toBeNull();
        expect(closeViewMock).not.toHaveBeenCalled();
    });

    it("confirms before closing dirty settings", async () => {
        act(() => {
            root.render(<TaskSchedulerView/>);
        });

        const openSettingsButton = container.querySelector("[data-testid='open-settings']") as HTMLButtonElement | null;
        expect(openSettingsButton).not.toBeNull();
        act(() => {
            openSettingsButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await flushEffects();

        const closeButton = container.querySelector("[data-testid='shell-close']") as HTMLButtonElement | null;
        expect(closeButton).not.toBeNull();
        act(() => {
            closeButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.textContent).toContain("Unsaved Settings");
        expect(closeViewMock).not.toHaveBeenCalled();

        const discardButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Discard Changes");
        expect(discardButton).toBeDefined();
        act(() => {
            discardButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(closeViewMock).toHaveBeenCalledTimes(1);
    });
});
