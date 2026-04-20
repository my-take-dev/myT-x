import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = {
    addNotification: vi.fn(),
    toggleView: vi.fn(),
    closeView: vi.fn(),
    config: {
        viewer_shortcuts: null as Record<string, string> | null,
    },
};

vi.mock("../src/components/viewer/ActivityStrip", () => ({
    ActivityStrip: () => <div data-testid="activity-strip"/>,
}));

vi.mock("../src/components/viewer/ViewOverlay", () => ({
    ViewOverlay: () => <div data-testid="view-overlay"/>,
}));

vi.mock("../src/components/viewer/DockedDivider", () => ({
    DockedDivider: () => <div data-testid="docked-divider"/>,
}));

vi.mock("../src/components/viewer/useIsViewerDocked", () => ({
    useIsViewerDocked: () => false,
}));

vi.mock("../src/components/viewer/viewerRegistry", () => ({
    getRegisteredViews: () => [
        {id: "git-graph", shortcut: "Ctrl+Shift+G"},
    ],
    registerView: vi.fn(),
    subscribeRegistry: () => () => {},
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {
        activeViewId: string | null;
        toggleView: typeof mocked.toggleView;
        closeView: typeof mocked.closeView;
    }) => unknown) => selector({
        activeViewId: null,
        toggleView: mocked.toggleView,
        closeView: mocked.closeView,
    }),
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {config: typeof mocked.config}) => unknown) => selector({config: mocked.config}),
}));

vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: (selector: (state: {addNotification: typeof mocked.addNotification}) => unknown) => (
        selector({addNotification: mocked.addNotification})
    ),
}));

import {ViewerSystem} from "../src/components/viewer/ViewerSystem";

describe("ViewerSystem", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        mocked.addNotification.mockReset();
        mocked.toggleView.mockReset();
        mocked.closeView.mockReset();
        mocked.config.viewer_shortcuts = null;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("warns and ignores a persisted reserved shortcut conflict", async () => {
        mocked.config.viewer_shortcuts = {
            "git-graph": "Ctrl+Shift+V",
        };

        await act(async () => {
            root.render(<ViewerSystem/>);
        });

        expect(mocked.addNotification).toHaveBeenCalledWith(
            "ショートカット \"Ctrl+Shift+V\" は \"ファイルビューのプレビュー切替\" で予約済みです",
            "warn",
        );

        await act(async () => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                key: "V",
                ctrlKey: true,
                shiftKey: true,
                bubbles: true,
                cancelable: true,
            }));
        });

        expect(mocked.toggleView).not.toHaveBeenCalled();
    });
});
