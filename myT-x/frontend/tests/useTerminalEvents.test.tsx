import {act, useRef} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {Terminal} from "@xterm/xterm";
import {useTerminalEvents} from "../src/hooks/useTerminalEvents";
import type {TerminalEventShared} from "../src/hooks/useTerminalKeyHandler";
import {TERMINAL_IME_RECOVERY_EVENT} from "../src/utils/imeRecovery";

const setPrefixModeMock = vi.hoisted(() => vi.fn());
const setupKeyHandlerMock = vi.hoisted(() => vi.fn(() => () => undefined));
const setupPaneDataStreamMock = vi.hoisted(() => vi.fn(() => () => undefined));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {setPrefixMode: typeof setPrefixModeMock}) => unknown) => {
        return selector({setPrefixMode: setPrefixModeMock});
    },
}));

vi.mock("../src/hooks/useTerminalKeyHandler", async (importOriginal) => {
    const actual = await importOriginal<typeof import("../src/hooks/useTerminalKeyHandler")>();
    return {
        ...actual,
        setupKeyHandler: (...args: unknown[]) => setupKeyHandlerMock(...args),
    };
});

vi.mock("../src/hooks/useTerminalPaneData", () => ({
    setupPaneDataStream: (...args: unknown[]) => setupPaneDataStreamMock(...args),
}));

interface FakeTerminal {
    blur: ReturnType<typeof vi.fn>;
    buffer: {active: {baseY: number; viewportY: number}};
    focus: ReturnType<typeof vi.fn>;
    onScroll: ReturnType<typeof vi.fn>;
    onWriteParsed: ReturnType<typeof vi.fn>;
    textarea: HTMLTextAreaElement;
}

function createFakeTerminal(): FakeTerminal {
    return {
        blur: vi.fn(),
        buffer: {active: {baseY: 0, viewportY: 0}},
        focus: vi.fn(),
        onScroll: vi.fn(() => ({dispose: vi.fn()})),
        onWriteParsed: vi.fn(() => ({dispose: vi.fn()})),
        textarea: document.createElement("textarea"),
    };
}

interface TerminalEventsProbeProps {
    paneId: string;
    term: FakeTerminal;
}

function TerminalEventsProbe({paneId, term}: TerminalEventsProbeProps) {
    const terminalRef = useRef(term as unknown as Terminal);
    const syncInputModeRef = useRef(false);
    const isComposingRef = useRef(false);

    useTerminalEvents({
        paneId,
        terminalRef,
        syncInputModeRef,
        setSearchOpen: () => undefined,
        setScrollAtBottom: () => undefined,
        isComposingRef,
    });

    return null;
}

function dispatchRecovery(paneId: string, reason: "terminal-focus" | "manual" = "terminal-focus"): void {
    window.dispatchEvent(new CustomEvent(TERMINAL_IME_RECOVERY_EVENT, {detail: {paneId, reason}}));
}

describe("useTerminalEvents IME recovery", () => {
    let container: HTMLDivElement;
    let root: Root;
    let term: FakeTerminal;

    beforeEach(() => {
        vi.useFakeTimers();
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        vi.spyOn(console, "debug").mockImplementation(() => undefined);
        setPrefixModeMock.mockReset();
        setupKeyHandlerMock.mockClear();
        setupPaneDataStreamMock.mockClear();
        term = createFakeTerminal();
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
        document.body.innerHTML = "";
        vi.useRealTimers();
        vi.restoreAllMocks();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("resets IME only for the matching pane", async () => {
        act(() => {
            root.render(<TerminalEventsProbe paneId="pane-1" term={term}/>);
        });

        act(() => {
            dispatchRecovery("pane-2");
        });
        expect(term.blur).not.toHaveBeenCalled();

        act(() => {
            dispatchRecovery("pane-1");
        });
        expect(term.blur).toHaveBeenCalledTimes(1);
        expect(term.focus).not.toHaveBeenCalled();

        await act(async () => {
            await vi.advanceTimersByTimeAsync(100);
        });
        expect(term.focus).toHaveBeenCalledTimes(1);
    });

    it("skips terminal-focus reset while composition is active", () => {
        act(() => {
            root.render(<TerminalEventsProbe paneId="pane-1" term={term}/>);
        });
        const setupArg = setupKeyHandlerMock.mock.calls[0]?.[0] as {shared: TerminalEventShared} | undefined;
        expect(setupArg).toBeDefined();
        setupArg!.shared.isComposing = true;

        act(() => {
            dispatchRecovery("pane-1", "terminal-focus");
        });

        expect(term.blur).not.toHaveBeenCalled();
        expect(term.focus).not.toHaveBeenCalled();
    });
});
