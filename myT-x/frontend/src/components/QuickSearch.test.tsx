import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../i18n";
import {QuickSearch} from "./QuickSearch";
import {QUICK_SEARCH_DIALOG_ID} from "./quickSearchShared";
import {useTmuxStore} from "../stores/tmuxStore";
import {useViewerStore} from "./viewer/viewerStore";

const setActiveSessionMock = vi.fn<(...args: [string]) => Promise<void>>();
const splitPaneMock = vi.fn<(...args: [string, boolean]) => Promise<void>>();
const toggleViewerSidebarModeMock = vi.fn<() => Promise<void>>();
const notifyOperationFailureMock = vi.fn();
const logFrontendEventSafeMock = vi.fn();
const registryListeners = new Set<() => void>();
let registeredViews = [
    {
        id: "mcp-manager",
        label: "MCP Manager",
        component: () => null,
        icon: () => null,
        shortcut: "Ctrl+Shift+M",
    },
];

function resetRegisteredViews() {
    registeredViews = [
        {
            id: "mcp-manager",
            label: "MCP Manager",
            component: () => null,
            icon: () => null,
            shortcut: "Ctrl+Shift+M",
        },
    ];
}

function updateRegisteredViews(nextViews: typeof registeredViews): void {
    registeredViews = nextViews;
    for (const listener of registryListeners) {
        listener();
    }
}

vi.mock("../api", () => ({
    api: {
        SetActiveSession: (sessionName: string) => setActiveSessionMock(sessionName),
        SplitPane: (paneId: string, vertical: boolean) => splitPaneMock(paneId, vertical),
        ToggleViewerSidebarMode: () => toggleViewerSidebarModeMock(),
    },
}));

vi.mock("../utils/notifyUtils", () => ({
    notifyOperationFailure: (...args: unknown[]) => notifyOperationFailureMock(...args),
}));

vi.mock("../utils/logFrontendEventSafe", () => ({
    logFrontendEventSafe: (...args: unknown[]) => logFrontendEventSafeMock(...args),
}));

vi.mock("./viewer/viewerRegistry", () => ({
    registerView: () => undefined,
    getRegisteredViews: () => registeredViews,
    subscribeRegistry: (listener: () => void) => {
        registryListeners.add(listener);
        return () => {
            registryListeners.delete(listener);
        };
    },
}));

function keyboardEvent(init: KeyboardEventInit & {keyCode?: number; isComposing?: boolean}): KeyboardEvent {
    const event = new KeyboardEvent("keydown", {...init, bubbles: true, cancelable: true});
    if (typeof init.keyCode === "number") {
        Object.defineProperty(event, "keyCode", {value: init.keyCode});
    }
    if (typeof init.isComposing === "boolean") {
        Object.defineProperty(event, "isComposing", {value: init.isComposing});
    }
    return event;
}

function setInputValue(input: HTMLInputElement, value: string): void {
    const valueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    valueSetter?.call(input, value);
    input.dispatchEvent(new Event("input", {bubbles: true}));
}

function buildSession(name: string, activePaneId: string) {
    return {
        id: name === "alpha" ? 1 : 2,
        name,
        created_at: "2026-04-19T00:00:00Z",
        is_idle: false,
        active_window_id: 1,
        windows: [{
            id: 1,
            name: "main",
            active_pane: 1,
            panes: activePaneId === ""
                ? []
                : [{
                    id: activePaneId,
                    index: 0,
                    title: `${name}-pane`,
                    active: true,
                    width: 80,
                    height: 24,
                }],
        }],
        worktree: {
            repo_path: `C:/repo/${name}`,
            branch_name: `${name}-branch`,
            is_detached: false,
        },
    };
}

function defaultConfig() {
    return {
        shell: "powershell.exe",
        prefix: "Ctrl+b",
        keys: {},
        quake_mode: true,
        global_hotkey: "Ctrl+Shift+F12",
        viewer_sidebar_mode: "overlay",
        default_session_dir: "",
        chat_overlay_percentage: 40,
        websocket_port: 0,
        worktree: {
            enabled: true,
            force_cleanup: false,
            setup_scripts: [],
            setup_script_timeout_seconds: 30,
            copy_files: [],
            copy_dirs: [],
        },
        task_scheduler: {
            pre_exec_reset_delay_s: 0,
            pre_exec_idle_timeout_s: 0,
            pre_exec_target_mode: "current_pane",
            message_templates: [{
                name: "Review Template",
                message: "Review the diff carefully.",
            }],
        },
        viewer_shortcuts: {},
    };
}

describe("QuickSearch", () => {
    let container: HTMLDivElement;
    let root: Root;
    let scrollIntoViewMock: ReturnType<typeof vi.spyOn>;
    let restoreScrollIntoView: (() => void) | null;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
        restoreScrollIntoView = null;
        if (!("scrollIntoView" in HTMLElement.prototype)) {
            Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
                configurable: true,
                writable: true,
                value: () => undefined,
            });
            restoreScrollIntoView = () => {
                Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
            };
        }
        scrollIntoViewMock = vi.spyOn(HTMLElement.prototype, "scrollIntoView").mockImplementation(() => {});

        setActiveSessionMock.mockReset().mockResolvedValue(undefined);
        splitPaneMock.mockReset().mockResolvedValue(undefined);
        toggleViewerSidebarModeMock.mockReset().mockResolvedValue(undefined);
        notifyOperationFailureMock.mockReset();
        logFrontendEventSafeMock.mockReset();
        registryListeners.clear();
        resetRegisteredViews();
        setLanguage("ja");

        useTmuxStore.setState({
            sessions: [buildSession("alpha", "%1"), buildSession("beta", "%2")] as never,
            activeSession: "beta",
            config: defaultConfig() as never,
        });
        useViewerStore.setState({activeViewId: null, viewContext: null});
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        scrollIntoViewMock.mockRestore();
        restoreScrollIntoView?.();
        container.remove();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("switches the active session from the command palette", async () => {
        const onClose = vi.fn();

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={onClose}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "alpha");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(setActiveSessionMock).toHaveBeenCalledWith("alpha");
        expect(useTmuxStore.getState().activeSession).toBe("alpha");
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("anchors the dropdown mode to the menu bar trigger", async () => {
        const anchor = document.createElement("button");
        Object.defineProperty(anchor, "getBoundingClientRect", {
            value: () => ({
                width: 220,
                height: 32,
                top: 12,
                right: 330,
                bottom: 44,
                left: 110,
                x: 110,
                y: 12,
                toJSON: () => ({}),
            }),
        });
        document.body.appendChild(anchor);
        const dropdownAnchorRef = {current: anchor as HTMLButtonElement | null};

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                    triggerMode="dropdown"
                    dropdownAnchorRef={dropdownAnchorRef}
                />,
            );
        });

        const dropdownShell = container.querySelector(".quick-search-dropdown-shell");
        const dropdownPanel = container.querySelector<HTMLElement>(".quick-search-panel--dropdown");

        expect(dropdownShell).not.toBeNull();
        expect(container.querySelector(".quick-search-overlay")).toBeNull();
        expect(dropdownPanel?.style.top).toBe("52px");
        expect(dropdownPanel?.style.left).toBe("60px");
        expect(dropdownPanel?.style.width).toBe("320px");
        expect(dropdownPanel?.style.maxHeight).toBe("420px");

        anchor.remove();
    });

    it("closes the anchored dropdown when clicking outside the panel and trigger", async () => {
        const onClose = vi.fn();
        const anchor = document.createElement("button");
        Object.defineProperty(anchor, "getBoundingClientRect", {
            value: () => ({
                width: 240,
                height: 32,
                top: 12,
                right: 360,
                bottom: 44,
                left: 120,
                x: 120,
                y: 12,
                toJSON: () => ({}),
            }),
        });
        document.body.appendChild(anchor);
        const dropdownAnchorRef = {current: anchor as HTMLButtonElement | null};

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={onClose}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                    triggerMode="dropdown"
                    dropdownAnchorRef={dropdownAnchorRef}
                />,
            );
        });

        await act(async () => {
            document.body.dispatchEvent(new MouseEvent("mousedown", {bubbles: true}));
        });

        expect(onClose).toHaveBeenCalledTimes(1);

        anchor.remove();
    });

    it("keeps palette mode on the modal overlay and exposes the dialog id", async () => {
        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        expect(container.querySelector(".quick-search-dropdown-shell")).toBeNull();
        expect(container.querySelector(".quick-search-overlay")).not.toBeNull();
        expect(container.querySelector(`#${QUICK_SEARCH_DIALOG_ID}`)).not.toBeNull();
    });

    it("opens viewer commands from the command palette", async () => {
        const onClose = vi.fn();

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={onClose}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "mcp manager");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(useViewerStore.getState().activeViewId).toBe("mcp-manager");
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("shows viewer shortcut metadata in the result list", async () => {
        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        expect(container.textContent).toContain("Ctrl+Shift+M");
    });

    it("hides viewer shortcut metadata when the configured shortcut is reserved", async () => {
        useTmuxStore.setState({
            config: {
                ...defaultConfig(),
                viewer_shortcuts: {
                    "mcp-manager": "Ctrl+P",
                },
            } as never,
        });

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        expect(container.textContent).not.toContain("Ctrl+P");
    });

    it("opens the task scheduler with template context", async () => {
        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "review template");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(useViewerStore.getState().activeViewId).toBe("task-scheduler");
        expect(useViewerStore.getState().viewContext).toMatchObject({
            kind: "task-scheduler-template",
            name: "Review Template",
            message: "Review the diff carefully.",
            targetPaneID: "%2",
        });
    });

    it("runs common commands from the command palette", async () => {
        const onOpenSettings = vi.fn();

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={onOpenSettings}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "settings");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(onOpenSettings).toHaveBeenCalledTimes(1);
    });

    it("opens the new-session flow from the command palette", async () => {
        const onOpenNewSession = vi.fn();

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={onOpenNewSession}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "new session");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(onOpenNewSession).toHaveBeenCalledTimes(1);
    });

    it("runs split-pane commands from the command palette", async () => {
        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "split pane vertical");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(splitPaneMock).toHaveBeenCalledWith("%2", true);
    });

    it("scrolls the selected result into view during keyboard navigation", async () => {
        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        scrollIntoViewMock.mockClear();

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "ArrowDown"}));
        });

        expect(scrollIntoViewMock).toHaveBeenCalledTimes(1);
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
            block: "nearest",
            inline: "nearest",
        });
    });

    it("stays stable when config omits task scheduler templates", async () => {
        useTmuxStore.setState({
            config: {
                ...defaultConfig(),
                task_scheduler: undefined,
            } as never,
        });

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        expect(container.querySelector("input")).not.toBeNull();
        expect(container.querySelectorAll(".quick-search-item").length).toBeGreaterThan(0);
        expect(container.textContent).not.toContain("Review Template");
    });

    it("updates viewer entries after runtime registry changes", async () => {
        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        expect(container.textContent).not.toContain("Usage Dashboard");

        await act(async () => {
            updateRegisteredViews([
                ...registeredViews,
                {
                    id: "usage-dashboard",
                    label: "Usage Dashboard",
                    component: () => null,
                    icon: () => null,
                    shortcut: "Ctrl+Shift+U",
                },
            ]);
            await Promise.resolve();
        });

        expect(container.textContent).toContain("Usage Dashboard");
        expect(container.textContent).toContain("Ctrl+Shift+U");
    });

    it("does not expose split pane commands when the active pane is unavailable", async () => {
        useTmuxStore.setState({
            sessions: [buildSession("alpha", ""), buildSession("beta", "")] as never,
            activeSession: "beta",
        });

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "split pane");
        });

        expect(container.querySelectorAll(".quick-search-item")).toHaveLength(0);
    });

    it("closes on Escape and ignores Enter while IME composition is active", async () => {
        const onClose = vi.fn();

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={onClose}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "alpha");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter", isComposing: true}));
            await Promise.resolve();
        });

        expect(setActiveSessionMock).not.toHaveBeenCalled();

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Escape"}));
        });

        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("keeps Escape available while a command is still executing", async () => {
        const onClose = vi.fn();
        let resolveSessionSwitch: (() => void) | null = null;
        setActiveSessionMock.mockImplementationOnce(() => new Promise<void>((resolve) => {
            resolveSessionSwitch = resolve;
        }));

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={onClose}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "alpha");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(input.readOnly).toBe(true);

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Escape"}));
        });

        expect(onClose).toHaveBeenCalledTimes(1);

        await act(async () => {
            resolveSessionSwitch?.();
            await Promise.resolve();
        });
    });

    it("uses the updated English command palette labels", async () => {
        setLanguage("en");

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={vi.fn()}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const dialog = container.querySelector("[role='dialog']");
        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        expect(dialog?.getAttribute("aria-label")).toBe("Command Palette");
        expect(input.placeholder).toBe("Search sessions, viewers, commands, or templates...");
    });

    it("logs failures with stable English labels while leaving the palette open", async () => {
        const onClose = vi.fn();
        setActiveSessionMock.mockRejectedValueOnce(new Error("boom"));

        await act(async () => {
            root.render(
                <QuickSearch
                    open
                    onClose={onClose}
                    onOpenNewSession={vi.fn()}
                    onOpenSettings={vi.fn()}
                />,
            );
        });

        const input = container.querySelector("input");
        if (!(input instanceof HTMLInputElement)) {
            throw new Error("expected command palette input");
        }

        await act(async () => {
            setInputValue(input, "alpha");
        });

        await act(async () => {
            input.dispatchEvent(keyboardEvent({key: "Enter"}));
            await Promise.resolve();
        });

        expect(onClose).not.toHaveBeenCalled();
        expect(notifyOperationFailureMock).toHaveBeenCalledTimes(1);
        expect(logFrontendEventSafeMock).toHaveBeenCalledWith(
            "warn",
            expect.stringContaining("Activate session failed"),
            "QuickSearch",
        );
    });
});
