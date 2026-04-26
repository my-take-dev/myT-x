import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../i18n";
import {useTmuxStore} from "../stores/tmuxStore";
import type {AppConfig, AppConfigAutoStartCommand} from "../types/tmux";
import {TerminalPane} from "./TerminalPane";

const apiMock = vi.hoisted(() => ({
    StartAutoStartCommand: vi.fn<(paneId: string, entry: AppConfigAutoStartCommand) => Promise<string>>(),
}));

vi.mock("../api", () => ({
    api: {
        StartAutoStartCommand: (paneId: string, entry: AppConfigAutoStartCommand) =>
            apiMock.StartAutoStartCommand(paneId, entry),
    },
}));

vi.mock("../stores/autoEnterStore", () => ({
    startAutoEnter: vi.fn<() => Promise<void>>(),
    stopAutoEnter: vi.fn<() => Promise<void>>(),
    useAutoEnterStore: (selector: (state: {activeEntries: Record<string, unknown>}) => unknown) =>
        selector({activeEntries: {}}),
}));

vi.mock("../hooks/useTerminalSetup", async () => {
    const React = await vi.importActual<typeof import("react")>("react");
    return {
        useTerminalSetup: (options: {
            containerRef: {current: HTMLDivElement | null};
            paneId: string;
            terminalRef: {current: unknown};
        }) => {
            React.useEffect(() => {
                const xterm = document.createElement("div");
                xterm.className = "xterm";
                options.containerRef.current?.appendChild(xterm);
                options.terminalRef.current = {
                    focus: vi.fn(),
                    scrollToBottom: vi.fn(),
                };
                return () => {
                    options.terminalRef.current = null;
                    xterm.remove();
                };
            }, [options.containerRef, options.paneId, options.terminalRef]);
        },
    };
});

vi.mock("../hooks/useTerminalEvents", () => ({
    useTerminalEvents: () => undefined,
}));

vi.mock("../hooks/useTerminalResize", () => ({
    useTerminalResize: () => undefined,
}));

vi.mock("../hooks/useTerminalFontSize", () => ({
    useTerminalFontSize: () => undefined,
}));

function createDeferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

function createConfig(autoStart: AppConfigAutoStartCommand[]): AppConfig {
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
            setup_script_timeout_seconds: 300,
            copy_files: [],
            copy_dirs: [],
        },
        auto_start: autoStart,
    };
}

describe("TerminalPane AutoStart", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        setLanguage("en");
        apiMock.StartAutoStartCommand.mockReset();
        useTmuxStore.setState({
            activeSession: "session-a",
            config: createConfig([{name: "Mini Codex", command: "codex", args: "--model gpt-5.4-mini"}]),
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        useTmuxStore.setState({activeSession: null, config: null});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("blocks duplicate AutoStart launches while the first launch is pending", async () => {
        const launch = createDeferred<string>();
        apiMock.StartAutoStartCommand.mockReturnValue(launch.promise);

        act(() => {
            root.render(
                <TerminalPane
                    paneId="%1"
                    paneTitle="Pane"
                    active
                    onFocus={vi.fn()}
                    onSplitVertical={vi.fn()}
                    onSplitHorizontal={vi.fn()}
                    onToggleZoom={vi.fn()}
                    onKillPane={vi.fn()}
                    onRenamePane={vi.fn()}
                    onSwapPane={vi.fn()}
                    onDetach={vi.fn()}
                />,
            );
        });

        const toolbarButton = container.querySelector(
            '[aria-label="Open AutoStart commands for pane %1"]',
        ) as HTMLButtonElement;
        expect(toolbarButton).not.toBeNull();

        act(() => {
            toolbarButton.click();
        });

        const commandButton = container.querySelector(".auto-start-command-btn") as HTMLButtonElement;
        expect(commandButton).not.toBeNull();

        act(() => {
            commandButton.click();
        });

        expect(commandButton.disabled).toBe(true);

        act(() => {
            commandButton.click();
        });

        expect(apiMock.StartAutoStartCommand).toHaveBeenCalledTimes(1);
        expect(apiMock.StartAutoStartCommand).toHaveBeenCalledWith("%1", {
            name: "Mini Codex",
            command: "codex",
            args: "--model gpt-5.4-mini",
        });

        await act(async () => {
            launch.resolve("%2");
            await launch.promise;
        });

        expect(container.querySelector(".auto-start-command-btn")).toBeNull();
    });
});
