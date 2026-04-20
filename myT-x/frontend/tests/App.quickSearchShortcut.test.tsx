import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {MenuBarProps} from "../src/components/MenuBar";
import type {QuickSearchTriggerMode} from "../src/components/quickSearchShared";

const isImeTransitionalEventMock = vi.fn(() => false);

vi.mock("../src/api", () => ({
    api: {
        GetValidationRules: async () => null,
        KillPane: async () => undefined,
        SetActiveSession: async () => undefined,
    },
}));

vi.mock("../src/components/ConfirmDialog", () => ({
    ConfirmDialog: () => null,
}));

vi.mock("../src/components/MenuBar", () => ({
    MenuBar: ({onOpenQuickSearch}: MenuBarProps) => (
        <button data-testid="menu-open-quick-search" onClick={onOpenQuickSearch}>menu</button>
    ),
}));

vi.mock("../src/components/QuickSearch", () => ({
    QuickSearch: ({
        open,
        onClose,
        triggerMode,
    }: {
        open: boolean;
        onClose: () => void;
        triggerMode?: QuickSearchTriggerMode;
    }) => (
        <>
            <div data-testid="quick-search-state">{`${open ? "open" : "closed"}:${triggerMode ?? "palette"}`}</div>
            {open && <button data-testid="quick-search-close" onClick={onClose}>close</button>}
        </>
    ),
}));

vi.mock("../src/components/SessionView", () => ({
    SessionView: () => <div>session-view</div>,
}));

vi.mock("../src/components/SettingsModal", () => ({
    SettingsModal: () => null,
}));

vi.mock("../src/components/Sidebar", () => ({
    Sidebar: () => <div>sidebar</div>,
}));

vi.mock("../src/components/ChatLayout", () => ({
    ChatLayout: ({children}: {children: ReactNode}) => <div>{children}</div>,
}));

vi.mock("../src/components/StatusBar", () => ({
    StatusBar: () => <div>status</div>,
}));

vi.mock("../src/components/ToastContainer", () => ({
    ToastContainer: () => null,
}));

vi.mock("../src/components/viewer", () => ({
    ViewerSystem: () => <div>viewer-system</div>,
}));

vi.mock("../src/components/viewer/viewerDocking", () => ({
    buildDockedCssVariables: () => ({}),
    buildDockedLayout: () => null,
    normalizeDockedViewportWidth: (width: number) => width,
}));

vi.mock("../src/components/viewer/useIsViewerDocked", () => ({
    useIsViewerDocked: () => false,
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {dockRatio: number}) => unknown) => selector({dockRatio: 0.5}),
}));

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
        sessions: [{name: "alpha", active_window_id: "", windows: []}],
        activeSession: "alpha",
        pendingPrefixKillPaneId: null,
        setPendingPrefixKillPaneId: () => undefined,
        config: null,
    };
    return {
        useTmuxStore: (selector: (store: typeof state) => unknown) => selector(state),
    };
});

vi.mock("../src/utils/ime", () => ({
    isImeTransitionalEvent: (event: KeyboardEvent) => isImeTransitionalEventMock(event),
}));

vi.mock("../src/utils/notifyUtils", () => ({
    notifyAndLog: () => undefined,
}));

import App from "../src/App";

describe("App quick search shortcut", () => {
    let container: HTMLDivElement;
    let root: Root;

    async function renderApp() {
        await act(async () => {
            root.render(<App/>);
            await Promise.resolve();
        });
    }

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
        isImeTransitionalEventMock.mockReset().mockReturnValue(false);
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("opens the palette on Ctrl+P", async () => {
        await renderApp();

        act(() => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ctrlKey: true,
                key: "p",
            }));
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:palette");
    });

    it("ignores Ctrl+Shift+P", async () => {
        await renderApp();

        act(() => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ctrlKey: true,
                shiftKey: true,
                key: "P",
            }));
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("closed:palette");
    });

    it("ignores already-handled shortcuts", async () => {
        await renderApp();

        const event = new KeyboardEvent("keydown", {
            bubbles: true,
            cancelable: true,
            ctrlKey: true,
            key: "p",
        });
        event.preventDefault();

        act(() => {
            window.dispatchEvent(event);
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("closed:palette");
    });

    it("ignores Ctrl+P during IME transitional input", async () => {
        await renderApp();
        isImeTransitionalEventMock.mockReturnValueOnce(true);

        act(() => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ctrlKey: true,
                key: "p",
            }));
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("closed:palette");
    });

    it("opens the quick search as a dropdown from the menu bar trigger", async () => {
        await renderApp();

        const button = container.querySelector<HTMLButtonElement>("[data-testid='menu-open-quick-search']");
        if (button === null) {
            throw new Error("expected menu quick search trigger");
        }

        act(() => {
            button.click();
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:dropdown");
    });

    it("switches a menu-opened quick search into palette mode on Ctrl+P", async () => {
        await renderApp();

        const button = container.querySelector<HTMLButtonElement>("[data-testid='menu-open-quick-search']");
        if (button === null) {
            throw new Error("expected menu quick search trigger");
        }

        act(() => {
            button.click();
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:dropdown");

        act(() => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ctrlKey: true,
                key: "p",
            }));
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:palette");
    });

    it("reopens the palette after a menu-opened quick search is closed", async () => {
        await renderApp();

        const button = container.querySelector<HTMLButtonElement>("[data-testid='menu-open-quick-search']");
        const closeButtonSelector = "[data-testid='quick-search-close']";
        if (button === null) {
            throw new Error("expected menu quick search trigger");
        }

        act(() => {
            button.click();
        });

        const closeButton = container.querySelector<HTMLButtonElement>(closeButtonSelector);
        if (closeButton === null) {
            throw new Error("expected quick search close trigger");
        }

        act(() => {
            closeButton.click();
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("closed:dropdown");

        act(() => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ctrlKey: true,
                key: "p",
            }));
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:palette");
    });

    it("switches an open palette into dropdown mode from the menu trigger", async () => {
        await renderApp();

        const button = container.querySelector<HTMLButtonElement>("[data-testid='menu-open-quick-search']");
        if (button === null) {
            throw new Error("expected menu quick search trigger");
        }

        act(() => {
            window.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ctrlKey: true,
                key: "p",
            }));
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:palette");

        act(() => {
            button.click();
        });

        expect(container.querySelector("[data-testid='quick-search-state']")?.textContent).toBe("open:dropdown");
    });
});
