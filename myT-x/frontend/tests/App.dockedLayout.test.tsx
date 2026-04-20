import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {MenuBarProps} from "../src/components/MenuBar";

const mocked = vi.hoisted(() => ({
    dockRatio: 0.5,
    getValidationRules: vi.fn(() => new Promise<null>(() => {})),
    isViewerDocked: true,
    killPane: vi.fn().mockResolvedValue(undefined),
    setActiveSession: vi.fn().mockResolvedValue(undefined),
    setPendingPrefixKillPaneId: vi.fn(),
}));

vi.mock("../src/api", () => ({
    api: {
        GetValidationRules: (...args: unknown[]) => mocked.getValidationRules(...args),
        KillPane: (...args: unknown[]) => mocked.killPane(...args),
        SetActiveSession: (...args: unknown[]) => mocked.setActiveSession(...args),
    },
}));

vi.mock("../src/components/ConfirmDialog", () => ({
    ConfirmDialog: () => null,
}));

vi.mock("../src/components/MenuBar", () => ({
    MenuBar: (_props: MenuBarProps) => <div className="menu-bar"/>,
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

vi.mock("../src/components/viewer", () => ({
    ViewerSystem: () => <div className="viewer-overlay"/>,
}));

vi.mock("../src/components/viewer/useIsViewerDocked", () => ({
    useIsViewerDocked: () => mocked.isViewerDocked,
}));

vi.mock("../src/components/viewer/viewerStore", () => {
    const useViewerStore = (selector: (state: {dockRatio: number}) => unknown) =>
        selector({dockRatio: mocked.dockRatio});
    return {useViewerStore};
});

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
        t: (_key: string, fallback: string) => fallback,
    }),
}));

vi.mock("../src/stores/tmuxStore", () => {
    const state = {
        activeSession: "session-1",
        config: {chat_overlay_percentage: 40},
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
    DOCKED_LAYOUT_BASE_WIDTH,
    DOCKED_LAYOUT_FIXED_CHROME_WIDTH,
    DOCKED_WINDOW_MIN_WIDTH,
} from "../src/components/viewer/viewerDocking";

function setWindowWidth(width: number) {
    Object.defineProperty(window, "innerWidth", {
        configurable: true,
        value: width,
        writable: true,
    });
}

describe("App docked layout", () => {
    let container: HTMLDivElement;
    let root: Root;
    let nextAnimationFrameID: number;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        nextAnimationFrameID = 1;
        mocked.dockRatio = 0.5;
        mocked.getValidationRules.mockClear();
        mocked.isViewerDocked = true;
        mocked.killPane.mockClear();
        mocked.setActiveSession.mockClear();
        mocked.setPendingPrefixKillPaneId.mockClear();
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
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("applies the scaled docked layout variables below the base-width threshold", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const appScale = 1200 / DOCKED_LAYOUT_BASE_WIDTH;
        const reservedWidth = DOCKED_ACTIVITY_STRIP_WIDTH / appScale;
        const contentWidth = DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - reservedWidth;
        expect(appBody).not.toBeNull();
        expect(appBody!.classList.contains("app-body--viewer-docked")).toBe(true);
        expect(appBody!.classList.contains("app-body--viewer-scaled")).toBe(true);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe(`${appScale}`);
        expect(appBody!.style.getPropertyValue("--dock-activity-strip-reserved-width")).toBe(`${reservedWidth}px`);
        expect(appBody!.style.getPropertyValue("--dock-app-unscaled-width")).toBe(`${(DOCKED_LAYOUT_BASE_WIDTH / 1200) * 100}%`);
        expect(appBody!.style.getPropertyValue("--dock-main-width")).toBe(`${contentWidth / 2}px`);
        expect(appBody!.style.getPropertyValue("--dock-viewer-width")).toBe(`${contentWidth / 2}px`);
    });

    it("renders app-body__inner as the scaled shell wrapper", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const appBodyInner = container.querySelector<HTMLDivElement>(".app-body__inner");
        expect(appBody).not.toBeNull();
        expect(appBodyInner).not.toBeNull();
        expect(appBody?.firstElementChild).toBe(appBodyInner);
        expect(appBodyInner?.querySelector(".sidebar")).not.toBeNull();
        expect(appBodyInner?.querySelector(".main-content")).not.toBeNull();
        expect(appBodyInner?.querySelector(".viewer-overlay")).not.toBeNull();
    });

    it("drops the scaled class once the window grows past the base-width threshold", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        expect(appBody).not.toBeNull();

        act(() => {
            setWindowWidth(1600);
            window.dispatchEvent(new Event("resize"));
        });

        expect(appBody!.classList.contains("app-body--viewer-docked")).toBe(true);
        expect(appBody!.classList.contains("app-body--viewer-scaled")).toBe(false);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe("1");
        expect(appBody!.style.getPropertyValue("--dock-activity-strip-reserved-width")).toBe(`${DOCKED_ACTIVITY_STRIP_WIDTH}px`);
        expect(appBody!.style.getPropertyValue("--dock-app-unscaled-width")).toBe("100%");
        expect(appBody!.style.getPropertyValue("--dock-main-width")).toBe(
            `${(1600 - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH) / 2}px`,
        );
        expect(appBody!.style.getPropertyValue("--dock-viewer-width")).toBe(
            `${(1600 - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH) / 2}px`,
        );
    });

    it("keeps dock-only variables unset outside docked mode", () => {
        mocked.isViewerDocked = false;

        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        expect(appBody).not.toBeNull();
        expect(appBody!.classList.contains("app-body--viewer-docked")).toBe(false);
        expect(appBody!.classList.contains("app-body--viewer-scaled")).toBe(false);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe("");
        expect(appBody!.style.getPropertyValue("--dock-activity-strip-reserved-width")).toBe("");
        expect(appBody!.style.getPropertyValue("--dock-main-width")).toBe("");
        expect(appBody!.style.getPropertyValue("--dock-viewer-width")).toBe("");
    });

    it("normalizes transient widths below the runtime minimum during resize", () => {
        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        expect(appBody).not.toBeNull();

        act(() => {
            setWindowWidth(600);
            window.dispatchEvent(new Event("resize"));
        });

        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe(`${DOCKED_WINDOW_MIN_WIDTH / DOCKED_LAYOUT_BASE_WIDTH}`);
    });

    it("clears dock-only variables before a queued resize flush can rerender overlay mode", () => {
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
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).not.toBe("");

        act(() => {
            setWindowWidth(980);
            window.dispatchEvent(new Event("resize"));
        });

        mocked.isViewerDocked = false;
        act(() => {
            root.render(<App/>);
        });

        act(() => {
            queuedResizeCallback?.(0);
        });

        expect(appBody!.classList.contains("app-body--viewer-docked")).toBe(false);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe("");
        expect(appBody!.style.getPropertyValue("--dock-viewer-width")).toBe("");
    });

    it("coalesces rapid resize events into one animation-frame update", () => {
        let queuedResizeCallback: FrameRequestCallback | null = null;
        vi.restoreAllMocks();
        const requestAnimationFrameSpy = vi.spyOn(window, "requestAnimationFrame").mockImplementation(
            (callback: FrameRequestCallback) => {
                queuedResizeCallback = callback;
                return nextAnimationFrameID++;
            },
        );
        vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);

        act(() => {
            root.render(<App/>);
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        expect(appBody).not.toBeNull();

        act(() => {
            setWindowWidth(1000);
            window.dispatchEvent(new Event("resize"));
            setWindowWidth(1010);
            window.dispatchEvent(new Event("resize"));
        });

        expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(1);
        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe(`${1200 / DOCKED_LAYOUT_BASE_WIDTH}`);

        act(() => {
            queuedResizeCallback?.(0);
        });

        expect(appBody!.style.getPropertyValue("--dock-app-scale")).toBe(`${1010 / DOCKED_LAYOUT_BASE_WIDTH}`);

        act(() => {
            setWindowWidth(1020);
            window.dispatchEvent(new Event("resize"));
        });

        expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(2);
    });

    it("removes the resize listener on unmount", () => {
        const addEventListenerSpy = vi.spyOn(window, "addEventListener");
        const removeEventListenerSpy = vi.spyOn(window, "removeEventListener");

        act(() => {
            root.render(<App/>);
        });

        const resizeHandler = addEventListenerSpy.mock.calls.find(([type]) => type === "resize")?.[1];
        expect(resizeHandler).toBeTypeOf("function");

        act(() => {
            root.unmount();
        });

        expect(removeEventListenerSpy).toHaveBeenCalledWith("resize", resizeHandler);
    });
});
