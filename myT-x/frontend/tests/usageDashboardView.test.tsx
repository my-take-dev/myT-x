import type {ReactNode} from "react";
import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../src/i18n";

const closeViewMock = vi.fn();
const refreshMock = vi.fn();
const hookState = {
    snapshot: null,
    isLoading: true,
    error: null,
    hasActiveSession: true,
    activeSessionName: "session-a",
    refresh: refreshMock,
};

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {closeView: typeof closeViewMock}) => unknown) =>
        selector({closeView: closeViewMock}),
}));

vi.mock("../src/components/viewer/views/usage-dashboard/useUsageDashboard", () => ({
    useUsageDashboard: () => hookState,
}));

vi.mock("../src/components/viewer/views/shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({children}: {children?: ReactNode}) => <div>{children}</div>,
}));

import {UsageDashboardView} from "../src/components/viewer/views/usage-dashboard/UsageDashboardView";

describe("UsageDashboardView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        setLanguage("en");
        closeViewMock.mockReset();
        refreshMock.mockReset();
        hookState.snapshot = null;
        hookState.isLoading = true;
        hookState.error = null;
        hookState.hasActiveSession = true;
        hookState.activeSessionName = "session-a";
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
        setLanguage("ja");
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("announces the loading skeleton as a polite status region", () => {
        act(() => {
            root.render(<UsageDashboardView/>);
        });

        const status = container.querySelector<HTMLElement>('[role="status"][aria-live="polite"]');
        expect(status).not.toBeNull();
        expect(status?.getAttribute("aria-busy")).toBe("true");
        expect(status?.getAttribute("aria-atomic")).toBe("true");
        expect(status?.getAttribute("aria-label")).toBe("Aggregating...");
        expect(container.textContent).toContain("Aggregating...");
        expect(container.querySelectorAll(".usage-dashboard-skeleton-card")).toHaveLength(3);
        expect(container.querySelectorAll(".usage-dashboard-skeleton-chart")).toHaveLength(1);
        expect(container.querySelectorAll(".usage-dashboard-skeleton-row")).toHaveLength(5);
    });
});
