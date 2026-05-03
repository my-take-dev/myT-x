import type {ReactNode} from "react";
import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../src/i18n";

const closeViewMock = vi.fn();
const refreshMock = vi.fn();
const hookState: {
    snapshot: unknown;
    isLoading: boolean;
    error: string | null;
    hasActiveSession: boolean;
    activeSessionName: string;
    refresh: typeof refreshMock;
} = {
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

vi.mock("../src/components/viewer/views/usage-dashboard/ClaudePanel", () => ({
    ClaudePanel: ({compact, titlePrefix}: {compact?: boolean; titlePrefix?: string}) => (
        <section data-testid="claude-panel" data-compact={String(Boolean(compact))}>
            {titlePrefix ?? "Claude Code"}
        </section>
    ),
}));

vi.mock("../src/components/viewer/views/usage-dashboard/CodexPanel", () => ({
    CodexPanel: ({compact, titlePrefix}: {compact?: boolean; titlePrefix?: string}) => (
        <section data-testid="codex-panel" data-compact={String(Boolean(compact))}>
            {titlePrefix ?? "Codex"}
        </section>
    ),
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

    it("starts in compare mode with Claude Code and Codex panels selected", () => {
        hookState.snapshot = {
            work_dir: "D:/myT-x/dev-myT-x",
            last_updated_at: "2026-04-15T20:00:00Z",
            claude: {},
            codex: {},
        };
        hookState.isLoading = false;

        act(() => {
            root.render(<UsageDashboardView/>);
        });

        const selector = container.querySelector<HTMLSelectElement>(".usage-dashboard-mode-select");
        const checkboxes = container.querySelectorAll<HTMLInputElement>('.usage-dashboard-comparison-controls input[type="checkbox"]');
        expect(selector?.value).toBe("compare");
        expect(container.querySelector('[data-testid="claude-panel"]')).not.toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();
        expect(checkboxes).toHaveLength(2);
        expect(Array.from(checkboxes).every((checkbox) => checkbox.checked)).toBe(true);
    });

    it("switches dashboard source display without calling refresh", () => {
        hookState.snapshot = {
            work_dir: "D:/myT-x/dev-myT-x",
            last_updated_at: "2026-04-15T20:00:00Z",
            claude: {},
            codex: {},
        };
        hookState.isLoading = false;

        act(() => {
            root.render(<UsageDashboardView/>);
        });

        const selector = container.querySelector<HTMLSelectElement>(".usage-dashboard-mode-select");
        expect(selector).not.toBeNull();

        act(() => {
            if (!selector) return;
            selector.value = "claude";
            selector.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="claude-panel"]')).not.toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).toBeNull();
        expect(container.querySelector('[data-testid="claude-panel"]')?.getAttribute("data-compact")).toBe("false");
        expect(container.querySelector(".usage-dashboard-comparison-controls")).toBeNull();

        act(() => {
            if (!selector) return;
            selector.value = "codex";
            selector.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="claude-panel"]')).toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')?.getAttribute("data-compact")).toBe("false");

        act(() => {
            if (!selector) return;
            selector.value = "compare";
            selector.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="claude-panel"]')).not.toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();
        expect(container.querySelector('[data-testid="claude-panel"]')?.getAttribute("data-compact")).toBe("true");
        expect(container.querySelector('[data-testid="codex-panel"]')?.getAttribute("data-compact")).toBe("true");
        expect(refreshMock).not.toHaveBeenCalled();
    });

    it("keeps at least one comparison checkbox selected", () => {
        hookState.snapshot = {
            work_dir: "D:/myT-x/dev-myT-x",
            last_updated_at: "2026-04-15T20:00:00Z",
            claude: {},
            codex: {},
        };
        hookState.isLoading = false;

        act(() => {
            root.render(<UsageDashboardView/>);
        });

        const checkboxes = container.querySelectorAll<HTMLInputElement>('.usage-dashboard-comparison-controls input[type="checkbox"]');

        act(() => {
            checkboxes[0]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="claude-panel"]')).toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')?.getAttribute("data-compact")).toBe("false");
        expect(checkboxes[1]?.checked).toBe(true);
        expect(checkboxes[1]?.disabled).toBe(false);
        expect(checkboxes[1]?.getAttribute("aria-disabled")).toBe("true");
        expect(checkboxes[1]?.getAttribute("aria-describedby")).toBeNull();
        expect(container.querySelector(".usage-dashboard-comparison-help")?.textContent).toBe("Select at least one source.");

        act(() => {
            checkboxes[1]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();
    });

    it("keeps comparison source selection when the active session changes", () => {
        hookState.snapshot = {
            work_dir: "D:/myT-x/dev-myT-x",
            last_updated_at: "2026-04-15T20:00:00Z",
            claude: {},
            codex: {},
        };
        hookState.isLoading = false;

        act(() => {
            root.render(<UsageDashboardView/>);
        });

        const checkboxes = container.querySelectorAll<HTMLInputElement>('.usage-dashboard-comparison-controls input[type="checkbox"]');

        act(() => {
            checkboxes[0]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="claude-panel"]')).toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();

        hookState.activeSessionName = "session-b";
        hookState.snapshot = {
            work_dir: "D:/myT-x/dev-myT-x-2",
            last_updated_at: "2026-04-15T21:00:00Z",
            claude: {},
            codex: {},
        };

        act(() => {
            root.render(<UsageDashboardView/>);
        });

        expect(container.textContent).toContain("session-b");
        expect(container.querySelector('[data-testid="claude-panel"]')).toBeNull();
        expect(container.querySelector('[data-testid="codex-panel"]')).not.toBeNull();
    });
});
