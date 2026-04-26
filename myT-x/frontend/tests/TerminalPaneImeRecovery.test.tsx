import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {LayoutRenderer} from "../src/components/LayoutRenderer";
import {useAppImeRecovery} from "../src/hooks/useAppImeRecovery";
import type {LayoutNode, PaneSnapshot} from "../src/types/tmux";
import {
    __resetTerminalFocusSuppressionsForTest,
    isTerminalImeRecoveryEvent,
    TERMINAL_IME_RECOVERY_EVENT,
    type TerminalImeRecoveryDetail,
} from "../src/utils/imeRecovery";

const apiMock = vi.hoisted(() => ({
    RecoverIMEWindowFocus: vi.fn<() => Promise<void>>(),
}));

vi.mock("../src/api", () => ({
    api: {
        RecoverIMEWindowFocus: () => apiMock.RecoverIMEWindowFocus(),
    },
}));

vi.mock("../src/stores/autoEnterStore", () => ({
    startAutoEnter: vi.fn(),
    stopAutoEnter: vi.fn(),
    useAutoEnterStore: (selector: (state: {activeEntries: Record<string, unknown>}) => unknown) => {
        return selector({activeEntries: {}});
    },
}));

vi.mock("../src/hooks/useTerminalSetup", async () => {
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
                const textarea = document.createElement("textarea");
                xterm.appendChild(textarea);
                options.containerRef.current?.appendChild(xterm);
                options.terminalRef.current = {
                    focus: () => textarea.focus(),
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

vi.mock("../src/hooks/useTerminalEvents", () => ({
    useTerminalEvents: () => undefined,
}));

vi.mock("../src/hooks/useTerminalResize", () => ({
    useTerminalResize: () => undefined,
}));

vi.mock("../src/hooks/useTerminalFontSize", () => ({
    useTerminalFontSize: () => undefined,
}));

function AppImeRecoveryProbe({activePaneId}: {activePaneId: string | null}) {
    const recoverySurfaceRef = useAppImeRecovery({activePaneId});
    return (
        <textarea
            ref={recoverySurfaceRef}
            data-ime-recovery-surface="true"
            tabIndex={-1}
            readOnly
            aria-hidden="true"
        />
    );
}

const panes: PaneSnapshot[] = [
    {id: "%1", index: 0, title: "left", active: true, width: 100, height: 40},
    {id: "%2", index: 1, title: "right", active: false, width: 100, height: 40},
];

const layout: LayoutNode = {
    type: "split",
    direction: "horizontal",
    ratio: 0.5,
    pane_id: -1,
    children: [
        {type: "leaf", pane_id: 1},
        {type: "leaf", pane_id: 2},
    ],
};

describe("TerminalPane IME recovery integration", () => {
    let container: HTMLDivElement;
    let root: Root;
    let recoveryEvents: TerminalImeRecoveryDetail[];
    let recoveryListener: (event: Event) => void;

    beforeEach(() => {
        vi.useFakeTimers();
        apiMock.RecoverIMEWindowFocus.mockReset();
        apiMock.RecoverIMEWindowFocus.mockResolvedValue(undefined);
        recoveryEvents = [];
        recoveryListener = (event: Event) => {
            if (isTerminalImeRecoveryEvent(event)) {
                recoveryEvents.push(event.detail);
            }
        };
        window.addEventListener(TERMINAL_IME_RECOVERY_EVENT, recoveryListener);
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        window.removeEventListener(TERMINAL_IME_RECOVERY_EVENT, recoveryListener);
        container.remove();
        document.body.innerHTML = "";
        vi.useRealTimers();
        vi.restoreAllMocks();
        __resetTerminalFocusSuppressionsForTest();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("routes terminal focus recovery from the rendered layout pane attribute", async () => {
        act(() => {
            root.render(
                <>
                    <AppImeRecoveryProbe activePaneId="%1"/>
                    <LayoutRenderer
                        layout={layout}
                        panes={panes}
                        activePaneId="%1"
                        zoomPaneId={null}
                        onFocusPane={() => undefined}
                        onSplitVertical={() => undefined}
                        onSplitHorizontal={() => undefined}
                        onToggleZoom={() => undefined}
                        onKillPane={() => undefined}
                        onRenamePane={() => undefined}
                        onSwapPane={() => undefined}
                        onDetachSession={() => undefined}
                    />
                </>,
            );
        });

        const targetPane = container.querySelector<HTMLElement>('[data-terminal-pane-id="%2"]');
        const targetInput = targetPane?.querySelector<HTMLTextAreaElement>(".xterm textarea");
        expect(targetPane).not.toBeNull();
        expect(targetInput).not.toBeNull();

        act(() => {
            targetPane!.dispatchEvent(new MouseEvent("mousedown", {bubbles: true}));
        });
        await act(async () => {
            await Promise.resolve();
            await vi.runAllTimersAsync();
        });

        expect(document.activeElement).toBe(targetInput);
        expect(apiMock.RecoverIMEWindowFocus).not.toHaveBeenCalled();
        expect(recoveryEvents).toEqual([{paneId: "%2", reason: "terminal-focus"}]);
    });

    it("does not move backend focus on drag start before pane swap", () => {
        const onFocusPane = vi.fn();
        const onSwapPane = vi.fn();

        act(() => {
            root.render(
                <LayoutRenderer
                    layout={layout}
                    panes={panes}
                    activePaneId="%1"
                    zoomPaneId={null}
                    onFocusPane={onFocusPane}
                    onSplitVertical={() => undefined}
                    onSplitHorizontal={() => undefined}
                    onToggleZoom={() => undefined}
                    onKillPane={() => undefined}
                    onRenamePane={() => undefined}
                    onSwapPane={onSwapPane}
                    onDetachSession={() => undefined}
                />,
            );
        });

        const sourcePane = container.querySelector<HTMLElement>('[data-terminal-pane-id="%1"]');
        const targetPane = container.querySelector<HTMLElement>('[data-terminal-pane-id="%2"]');
        expect(sourcePane).not.toBeNull();
        expect(targetPane).not.toBeNull();

        const dataTransfer = createDataTransfer();
        act(() => {
            sourcePane!.dispatchEvent(new MouseEvent("mousedown", {bubbles: true}));
            dispatchDragEvent(sourcePane!, "dragstart", dataTransfer);
            dispatchDragEvent(targetPane!, "drop", dataTransfer);
        });

        expect(onFocusPane).not.toHaveBeenCalled();
        expect(onSwapPane).toHaveBeenCalledWith("%1", "%2");
    });
});

function createDataTransfer(): DataTransfer {
    const values = new Map<string, string>();
    return {
        files: [] as unknown as FileList,
        setData: (format: string, data: string) => {
            values.set(format, data);
        },
        getData: (format: string) => values.get(format) ?? "",
    } as DataTransfer;
}

function dispatchDragEvent(target: HTMLElement, type: string, dataTransfer: DataTransfer): void {
    const event = new Event(type, {bubbles: true, cancelable: true});
    Object.defineProperty(event, "dataTransfer", {
        configurable: true,
        value: dataTransfer,
    });
    target.dispatchEvent(event);
}
