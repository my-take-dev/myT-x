import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    ToggleViewerSidebarMode: vi.fn<() => Promise<void>>(),
    toggleView: vi.fn<(viewID: string) => void>(),
    views: [{
        id: "test-view",
        icon: () => null,
        label: "Test View",
        component: () => null,
    }],
}));

let mockSidebarMode: string | undefined;
let resolveToggleViewerSidebarMode: (() => void) | null = null;

vi.mock("../src/api", () => ({
    api: {
        ToggleViewerSidebarMode: () => mocked.ToggleViewerSidebarMode(),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {
        config: { viewer_shortcuts: null; viewer_sidebar_mode?: string }
    }) => unknown) =>
        selector({
            config: {
                viewer_shortcuts: null,
                viewer_sidebar_mode: mockSidebarMode,
            },
        }),
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {
        activeViewId: string | null;
        toggleView: (viewID: string) => void
    }) => unknown) =>
        selector({
            activeViewId: null,
            toggleView: mocked.toggleView,
        }),
}));

vi.mock("../src/components/viewer/viewerRegistry", () => ({
    getRegisteredViews: () => mocked.views,
}));

import {ActivityStrip} from "../src/components/viewer/ActivityStrip";

describe("ActivityStrip", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mockSidebarMode = undefined;
        resolveToggleViewerSidebarMode = null;
        mocked.ToggleViewerSidebarMode.mockReset();
        mocked.toggleView.mockReset();
        mocked.ToggleViewerSidebarMode.mockImplementation(() => new Promise<void>((resolve) => {
            resolveToggleViewerSidebarMode = resolve;
        }));
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

    it("blocks duplicate sidebar-mode toggles while a toggle is already in flight", async () => {
        act(() => {
            root.render(<ActivityStrip/>);
        });

        const toggleButton = container.querySelector<HTMLButtonElement>("[aria-label='ドッキング表示に切替']");
        expect(toggleButton).not.toBeNull();

        await act(async () => {
            toggleButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            toggleButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(mocked.ToggleViewerSidebarMode).toHaveBeenCalledTimes(1);
        expect(toggleButton?.disabled).toBe(true);

        await act(async () => {
            resolveToggleViewerSidebarMode?.();
            await Promise.resolve();
        });

        expect(toggleButton?.disabled).toBe(false);
    });
});
