import {act, useRef} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {FitAddon} from "@xterm/addon-fit";
import type {SearchAddon} from "@xterm/addon-search";
import type {Terminal} from "@xterm/xterm";
import {__resetTerminalWebglProbeForTest, useTerminalSetup} from "../src/hooks/useTerminalSetup";

interface FakeTerminalInstance {
    cols: number;
    dispose: ReturnType<typeof vi.fn>;
    focus: ReturnType<typeof vi.fn>;
    loadAddon: ReturnType<typeof vi.fn>;
    open: ReturnType<typeof vi.fn>;
    rows: number;
    write: ReturnType<typeof vi.fn>;
}

const terminalInstances = vi.hoisted(() => [] as FakeTerminalInstance[]);
const fitCalls = vi.hoisted(() => [] as string[]);
const resizePaneMock = vi.hoisted(() => vi.fn<() => Promise<void>>());
const getPaneReplayMock = vi.hoisted(() => vi.fn<() => Promise<string>>());
const fontSizeMock = vi.hoisted(() => vi.fn(() => 17));

vi.mock("@xterm/xterm", () => ({
    Terminal: class {
        cols = 80;
        rows = 24;
        dispose = vi.fn();
        focus = vi.fn();
        loadAddon = vi.fn();
        open = vi.fn();
        write = vi.fn();

        constructor() {
            terminalInstances.push(this);
        }
    },
}));

vi.mock("@xterm/addon-fit", () => ({
    FitAddon: class {
        fit = vi.fn(() => {
            fitCalls.push("fit");
        });
    },
}));

vi.mock("@xterm/addon-search", () => ({
    SearchAddon: class {},
}));

vi.mock("@xterm/addon-web-links", () => ({
    WebLinksAddon: class {
        constructor(_handler: unknown) {}
    },
}));

vi.mock("@xterm/addon-webgl", () => ({
    WebglAddon: class {
        dispose = vi.fn();
        onContextLoss = vi.fn();
    },
}));

vi.mock("../../wailsjs/runtime/runtime", () => ({
    BrowserOpenURL: vi.fn(),
}));

vi.mock("../src/api", () => ({
    api: {
        ResizePane: (..._args: unknown[]) => resizePaneMock(),
        GetPaneReplay: (..._args: unknown[]) => getPaneReplayMock(),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: {
        getState: () => ({fontSize: fontSizeMock()}),
    },
}));

interface SetupProbeProps {
    focusOnOpen: boolean;
    paneId: string;
}

function SetupProbe({focusOnOpen, paneId}: SetupProbeProps) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const terminalRef = useRef<Terminal | null>(null);
    const searchAddonRef = useRef<SearchAddon | null>(null);
    const fitAddonRef = useRef<FitAddon | null>(null);

    useTerminalSetup({
        paneId,
        focusOnOpen,
        containerRef,
        terminalRef,
        searchAddonRef,
        fitAddonRef,
    });

    return <div ref={containerRef}/>;
}

describe("useTerminalSetup focus-on-open contract", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        terminalInstances.length = 0;
        fitCalls.length = 0;
        resizePaneMock.mockReset();
        resizePaneMock.mockResolvedValue(undefined);
        getPaneReplayMock.mockReset();
        getPaneReplayMock.mockResolvedValue("");
        fontSizeMock.mockReset();
        fontSizeMock.mockReturnValue(17);
        __resetTerminalWebglProbeForTest();
        vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(() => null);
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
        vi.restoreAllMocks();
        __resetTerminalWebglProbeForTest();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("does not focus when a pane opens without the focus hint", () => {
        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={false}/>);
        });

        expect(terminalInstances).toHaveLength(1);
        expect(terminalInstances[0]?.open).toHaveBeenCalledTimes(1);
        expect(terminalInstances[0]?.focus).not.toHaveBeenCalled();
        expect(fitCalls).toEqual(["fit"]);
    });

    it("focuses exactly once when a pane opens with the focus hint", () => {
        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={true}/>);
        });

        expect(terminalInstances).toHaveLength(1);
        expect(terminalInstances[0]?.focus).toHaveBeenCalledTimes(1);
    });

    it("does not treat same-pane focusOnOpen changes as live focus changes", () => {
        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={false}/>);
        });
        const firstTerminal = terminalInstances[0];

        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={true}/>);
        });

        expect(terminalInstances).toHaveLength(1);
        expect(firstTerminal?.focus).not.toHaveBeenCalled();
    });

    it("samples focusOnOpen again when paneId recreates the terminal", () => {
        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={false}/>);
        });
        const firstTerminal = terminalInstances[0];

        act(() => {
            root.render(<SetupProbe paneId="pane-2" focusOnOpen={true}/>);
        });

        expect(firstTerminal?.dispose).toHaveBeenCalledTimes(1);
        expect(terminalInstances).toHaveLength(2);
        expect(terminalInstances[1]?.focus).toHaveBeenCalledTimes(1);
    });

    it("probes WebGL support only once per module lifetime", () => {
        const previousWebGL = globalThis.WebGLRenderingContext;
        const previousWebGL2 = globalThis.WebGL2RenderingContext;
        Object.defineProperty(globalThis, "WebGLRenderingContext", {
            configurable: true,
            value: function WebGLRenderingContext() {},
        });
        Object.defineProperty(globalThis, "WebGL2RenderingContext", {
            configurable: true,
            value: undefined,
        });
        const contextSpy = vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(() => null);
        __resetTerminalWebglProbeForTest();

        try {
            act(() => {
                root.render(<SetupProbe paneId="pane-1" focusOnOpen={false}/>);
            });
            act(() => {
                root.render(<SetupProbe paneId="pane-2" focusOnOpen={false}/>);
            });

            expect(contextSpy).toHaveBeenCalledTimes(2);
        } finally {
            Object.defineProperty(globalThis, "WebGLRenderingContext", {
                configurable: true,
                value: previousWebGL,
            });
            Object.defineProperty(globalThis, "WebGL2RenderingContext", {
                configurable: true,
                value: previousWebGL2,
            });
        }
    });

    it("logs initial resize failures outside development mode", async () => {
        const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);
        resizePaneMock.mockRejectedValueOnce(new Error("resize failed"));

        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={false}/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(consoleWarnSpy).toHaveBeenCalledWith(
            "[terminal] initial ResizePane failed for pane=pane-1",
            expect.any(Error),
        );
    });

    it("logs replay load failures outside development mode", async () => {
        const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);
        getPaneReplayMock.mockRejectedValueOnce(new Error("replay failed"));

        act(() => {
            root.render(<SetupProbe paneId="pane-1" focusOnOpen={false}/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(consoleWarnSpy).toHaveBeenCalledWith(
            "[terminal] replay load failed for pane=pane-1",
            expect.any(Error),
        );
    });

});
