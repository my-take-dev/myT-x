import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    addNotification: vi.fn(),
    getValidationRules: vi.fn(() => new Promise<null>(() => {})),
    killPane: vi.fn().mockResolvedValue(undefined),
    setActiveSession: vi.fn().mockResolvedValue(undefined),
    setPendingPrefixKillPaneId: vi.fn(),
    toggleViewerSidebarMode: vi.fn().mockResolvedValue(undefined),
    views: [{
        component: () => <div className="test-view-content"/>,
        icon: () => null,
        id: "test-view",
        label: "Test View",
    }],
}));

vi.mock("../src/api", () => ({
    api: {
        GetValidationRules: (...args: unknown[]) => mocked.getValidationRules(...args),
        KillPane: (...args: unknown[]) => mocked.killPane(...args),
        SetActiveSession: (...args: unknown[]) => mocked.setActiveSession(...args),
        ToggleViewerSidebarMode: (...args: unknown[]) => mocked.toggleViewerSidebarMode(...args),
    },
}));

vi.mock("../src/components/ConfirmDialog", () => ({
    ConfirmDialog: () => null,
}));

vi.mock("../src/components/MenuBar", () => ({
    MenuBar: () => <div className="menu-bar"/>,
}));

vi.mock("../src/components/QuickSearch", () => ({
    QuickSearch: () => null,
}));

vi.mock("../src/components/SessionView", () => ({
    SessionView: () => <div className="session-view"/>,
}));

vi.mock("../src/components/SettingsModal", () => ({
    SettingsModal: () => null,
}));

vi.mock("../src/components/Sidebar", () => ({
    Sidebar: () => <aside className="sidebar"/>,
}));

vi.mock("../src/components/ChatLayout", () => ({
    ChatLayout: ({children}: {children?: ReactNode}) => <div className="chat-layout">{children}</div>,
}));

vi.mock("../src/components/StatusBar", () => ({
    StatusBar: () => <div className="status-bar"/>,
}));

vi.mock("../src/components/ToastContainer", () => ({
    ToastContainer: () => null,
}));

vi.mock("../src/components/viewer/viewerRegistry", () => ({
    getRegisteredViews: () => mocked.views,
    subscribeRegistry: () => () => undefined,
}));

vi.mock("../src/components/viewer/views/file-tree", () => ({}));
vi.mock("../src/components/viewer/views/editor", () => ({}));
vi.mock("../src/components/viewer/views/git-graph", () => ({}));
vi.mock("../src/components/viewer/views/diff-view", () => ({}));
vi.mock("../src/components/viewer/views/input-history", () => ({}));
vi.mock("../src/components/viewer/views/mcp-manager", () => ({}));
vi.mock("../src/components/viewer/views/pane-scheduler", () => ({}));
vi.mock("../src/components/viewer/views/orchestrator-teams", () => ({}));
vi.mock("../src/components/viewer/views/single-task-runner", () => ({}));
vi.mock("../src/components/viewer/views/task-scheduler", () => ({}));
vi.mock("../src/components/viewer/views/error-log", () => ({}));

vi.mock("../src/hooks/useAppImeRecovery", () => ({
    useAppImeRecovery: () => null,
}));

vi.mock("../src/hooks/useBackendSync", () => ({
    useBackendSync: () => undefined,
}));

vi.mock("../src/hooks/useFileDrop", () => ({
    useFileDrop: () => undefined,
}));

vi.mock("../src/hooks/usePrefixKeyMode", () => ({
    usePrefixKeyMode: () => undefined,
}));

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: (_key: string, fallback: string) => fallback,
    }),
}));

vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: (selector: (state: {addNotification: typeof mocked.addNotification}) => unknown) =>
        selector({addNotification: mocked.addNotification}),
}));

vi.mock("../src/stores/tmuxStore", () => {
    const state = {
        activeSession: "session-1",
        config: {
            chat_overlay_percentage: 40,
            viewer_shortcuts: null,
            viewer_sidebar_mode: "docked",
        },
        pendingPrefixKillPaneId: null,
        sessions: [{active_window_id: "", name: "session-1", windows: []}],
        setPendingPrefixKillPaneId: (...args: unknown[]) => mocked.setPendingPrefixKillPaneId(...args),
    };
    const useTmuxStore = (selector: (store: typeof state) => unknown) => selector(state);
    return {useTmuxStore};
});

vi.mock("../src/utils/ime", () => ({
    isImeTransitionalEvent: () => false,
}));

vi.mock("../src/utils/notifyUtils", () => ({
    notifyAndLog: () => undefined,
}));

import App from "../src/App";
import {
    DOCKED_ACTIVITY_STRIP_WIDTH,
    DOCKED_DIVIDER_WIDTH,
    DOCKED_LAYOUT_BASE_WIDTH,
    DOCK_RATIO_DEFAULT,
    DOCKED_SIDEBAR_WIDTH,
    DOCKED_WINDOW_MIN_WIDTH,
} from "../src/components/viewer/viewerDocking";
import {useViewerStore} from "../src/components/viewer/viewerStore";

function createRect(width: number): DOMRect {
    return DOMRect.fromRect({x: 0, y: 0, width, height: 900});
}

function setWindowWidth(width: number) {
    Object.defineProperty(window, "innerWidth", {
        configurable: true,
        value: width,
        writable: true,
    });
}

describe("App docked integration", () => {
    let container: HTMLDivElement;
    let root: Root;
    let nextAnimationFrameID: number;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        nextAnimationFrameID = 1;
        mocked.addNotification.mockReset();
        mocked.getValidationRules.mockClear();
        mocked.killPane.mockClear();
        mocked.setActiveSession.mockClear();
        mocked.setPendingPrefixKillPaneId.mockClear();
        mocked.toggleViewerSidebarMode.mockClear();
        useViewerStore.setState({
            activeViewId: "test-view",
            dockRatio: DOCK_RATIO_DEFAULT,
            viewContext: null,
        });
        setWindowWidth(1200);
        vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback: FrameRequestCallback) => {
            callback(0);
            return nextAnimationFrameID++;
        });
        vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        useViewerStore.setState({
            activeViewId: null,
            dockRatio: DOCK_RATIO_DEFAULT,
            viewContext: null,
        });
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("keeps a viewport-width reserve for the portaled activity strip at 1200px and 980px", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const portalStrip = document.body.querySelector<HTMLDivElement>(".viewer-activity-strip");
        expect(appBody).not.toBeNull();
        expect(portalStrip).not.toBeNull();
        expect(container.querySelector(".viewer-activity-strip")).toBeNull();
        expect(container.querySelector(".docked-divider")).not.toBeNull();
        expect(container.querySelector(".viewer-overlay")).not.toBeNull();

        const scaledReserve1200 = Number.parseFloat(
            appBody!.style.getPropertyValue("--dock-activity-strip-reserved-width"),
        );
        const appScale1200 = 1200 / DOCKED_LAYOUT_BASE_WIDTH;
        expect(scaledReserve1200 * appScale1200).toBeCloseTo(DOCKED_ACTIVITY_STRIP_WIDTH);

        act(() => {
            setWindowWidth(980);
            window.dispatchEvent(new Event("resize"));
        });

        const scaledReserve980 = Number.parseFloat(
            appBody!.style.getPropertyValue("--dock-activity-strip-reserved-width"),
        );
        const appScale980 = 980 / DOCKED_LAYOUT_BASE_WIDTH;
        expect(scaledReserve980 * appScale980).toBeCloseTo(DOCKED_ACTIVITY_STRIP_WIDTH);
    });

    it("uses the real docked divider against the scaled viewport content width", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const divider = container.querySelector<HTMLDivElement>(".docked-divider");
        expect(appBody).not.toBeNull();
        expect(divider).not.toBeNull();

        vi.spyOn(appBody!, "getBoundingClientRect").mockReturnValue(createRect(1200));

        act(() => {
            divider!.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 360}));
        });

        // 1200px viewport: hand-calculated displayed content width is about 927.33px.
        expect(useViewerStore.getState().dockRatio).toBeCloseTo(0.6078, 3);
    });

    it("keeps the divider math aligned with App when transient widths dip below the runtime minimum", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const divider = container.querySelector<HTMLDivElement>(".docked-divider");
        expect(appBody).not.toBeNull();
        expect(divider).not.toBeNull();

        act(() => {
            setWindowWidth(600);
            window.dispatchEvent(new Event("resize"));
        });

        vi.spyOn(appBody!, "getBoundingClientRect").mockReturnValue(createRect(600));

        act(() => {
            divider!.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 360}));
        });

        const appScale = DOCKED_WINDOW_MIN_WIDTH / DOCKED_LAYOUT_BASE_WIDTH;
        const visibleContentWidth =
            DOCKED_WINDOW_MIN_WIDTH -
            (DOCKED_SIDEBAR_WIDTH * appScale) -
            (DOCKED_DIVIDER_WIDTH * appScale) -
            DOCKED_ACTIVITY_STRIP_WIDTH;
        const expectedRatio = 0.5 + (360 - 260) / visibleContentWidth;
        expect(useViewerStore.getState().dockRatio).toBeCloseTo(expectedRatio);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe(
            `${DOCKED_WINDOW_MIN_WIDTH / DOCKED_LAYOUT_BASE_WIDTH}`,
        );
    });

    it("routes docked layout variables into the real DOM and stylesheet consumers", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const mainContent = container.querySelector<HTMLElement>(".main-content");
        const viewerOverlay = container.querySelector<HTMLElement>(".viewer-overlay");
        const divider = container.querySelector<HTMLElement>(".docked-divider");
        const portalStrip = document.body.querySelector<HTMLElement>(".viewer-activity-strip");
        expect(appBody).not.toBeNull();
        expect(mainContent).not.toBeNull();
        expect(viewerOverlay).not.toBeNull();
        expect(divider).not.toBeNull();
        expect(portalStrip).not.toBeNull();
        expect(container.querySelector(".viewer-activity-strip")).toBeNull();

        const sharedCss = readFileSync(resolve(import.meta.dirname, "../src/styles/viewer/shared.css"), "utf8");

        expect(appBody!.style.getPropertyValue("--dock-main-width")).not.toBe("");
        expect(appBody!.style.getPropertyValue("--dock-viewer-width")).not.toBe("");
        expect(appBody!.style.getPropertyValue("--dock-activity-strip-reserved-width")).not.toBe("");
        expect(mainContent!.classList.contains("main-content")).toBe(true);
        expect(viewerOverlay!.classList.contains("viewer-overlay")).toBe(true);
        expect(divider!.classList.contains("docked-divider")).toBe(true);
        expect(sharedCss).toContain(".app-body--viewer-docked .viewer-overlay");
        expect(sharedCss).toContain("width: var(--dock-viewer-width, 50%)");
        expect(sharedCss).toContain("margin-right: var(--dock-activity-strip-reserved-width, var(--activity-strip-width, 36px));");
        expect(sharedCss).toContain(".app-body--viewer-docked .main-content");
        expect(sharedCss).toContain("width: var(--dock-main-width, 50%)");
        expect(sharedCss).toContain(".docked-divider");
        expect(sharedCss).toContain("width: var(--dock-divider-width, 4px)");
    });

    it("clears dock variables if a queued resize flushes after the view is no longer docked", () => {
        let queuedResizeCallback: FrameRequestCallback | null = null;
        vi.restoreAllMocks();
        vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback: FrameRequestCallback) => {
            queuedResizeCallback = callback;
            return nextAnimationFrameID++;
        });
        vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);

        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        expect(appBody).not.toBeNull();
        expect(appBody!.classList.contains("app-body--viewer-docked")).toBe(true);

        act(() => {
            setWindowWidth(980);
            window.dispatchEvent(new Event("resize"));
        });

        act(() => {
            useViewerStore.setState({activeViewId: null});
        });

        act(() => {
            queuedResizeCallback?.(0);
        });

        expect(appBody!.classList.contains("app-body--viewer-docked")).toBe(false);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe("");
        expect(appBody!.style.getPropertyValue("--dock-main-width")).toBe("");
    });
});
