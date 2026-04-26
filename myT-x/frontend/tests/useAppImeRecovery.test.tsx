import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useAppImeRecovery} from "../src/hooks/useAppImeRecovery";
import {useTmuxStore} from "../src/stores/tmuxStore";
import {
    IME_RECOVERY_AUTO_COOLDOWN_MS,
    __resetTerminalFocusSuppressionsForTest,
    isActiveTerminalTextEntryElement,
    isTerminalTextEntryElement,
    isTerminalImeRecoveryEvent,
    suppressNextTerminalFocusImeRecovery,
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

interface AppImeRecoveryProbeProps {
    activePaneId: string | null;
}

function AppImeRecoveryProbe({activePaneId}: AppImeRecoveryProbeProps) {
    const recoverySurfaceRef = useAppImeRecovery({activePaneId});

    return (
        <textarea
            ref={recoverySurfaceRef}
            className="ime-recovery-surface"
            data-ime-recovery-surface="true"
            tabIndex={-1}
            readOnly
            aria-hidden="true"
        />
    );
}

async function flushRecoveryTimers(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
        await vi.runAllTimersAsync();
    });
}

function appendTerminalInput(paneId?: string): HTMLTextAreaElement {
    const terminalPane = document.createElement("div");
    terminalPane.className = "terminal-pane";
    if (paneId !== undefined) {
        terminalPane.setAttribute("data-terminal-pane-id", paneId);
    }
    const terminalElement = document.createElement("div");
    terminalElement.className = "xterm";
    const terminalInput = document.createElement("textarea");
    terminalElement.appendChild(terminalInput);
    terminalPane.appendChild(terminalElement);
    document.body.appendChild(terminalPane);
    return terminalInput;
}

describe("useAppImeRecovery", () => {
    let container: HTMLDivElement;
    let root: Root;
    let recoveryEvents: TerminalImeRecoveryDetail[];
    let recoveryListener: (event: Event) => void;

    beforeEach(() => {
        vi.useFakeTimers();
        apiMock.RecoverIMEWindowFocus.mockReset();
        apiMock.RecoverIMEWindowFocus.mockResolvedValue(undefined);
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        useTmuxStore.setState({
            imeResetSignal: 0,
            activeSession: null,
            sessions: [],
        });

        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        recoveryEvents = [];
        recoveryListener = (event: Event) => {
            if (isTerminalImeRecoveryEvent(event)) {
                recoveryEvents.push(event.detail);
            }
        };
        window.addEventListener(TERMINAL_IME_RECOVERY_EVENT, recoveryListener);
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

    it("calls native recovery and dispatches terminal reset on manual reset", async () => {
        const terminalInput = appendTerminalInput();

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });

        act(() => {
            useTmuxStore.getState().triggerImeReset();
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).toHaveBeenCalledTimes(1);
        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "manual"}]);
    });

    it("runs soft recovery on window focus without calling the backend", async () => {
        const terminalInput = appendTerminalInput();

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
            terminalInput.dispatchEvent(new FocusEvent("focusout", {bubbles: true, relatedTarget: null}));
            window.dispatchEvent(new Event("blur"));
        });

        act(() => {
            window.dispatchEvent(new Event("focus"));
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).not.toHaveBeenCalled();
        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "window-focus"}]);
    });

    it("runs soft recovery on active terminal focus without calling the backend", async () => {
        const terminalInput = appendTerminalInput("pane-1");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).not.toHaveBeenCalled();
        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "terminal-focus"}]);
    });

    it("does not repeat terminal focus recovery during cooldown", async () => {
        const terminalInput = appendTerminalInput("pane-1");
        const otherInput = document.createElement("input");
        document.body.appendChild(otherInput);

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();

        act(() => {
            otherInput.focus();
            terminalInput.focus();
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).not.toHaveBeenCalled();
        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "terminal-focus"}]);
    });

    it("uses the focused terminal pane id even before active pane state catches up", async () => {
        const terminalInput = appendTerminalInput("pane-2");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).not.toHaveBeenCalled();
        expect(recoveryEvents).toEqual([{paneId: "pane-2", reason: "terminal-focus"}]);
    });

    it("dispatches terminal focus recovery to the pane captured before the surface cycle", async () => {
        const terminalInput = appendTerminalInput("pane-1");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-2"/>);
        });
        await flushRecoveryTimers();

        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "terminal-focus"}]);
    });

    it("dispatches manual recovery to the focused terminal captured before the async cycle", async () => {
        const terminalInput = appendTerminalInput("pane-1");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();
        recoveryEvents = [];

        act(() => {
            useTmuxStore.getState().triggerImeReset();
        });
        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-2"/>);
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).toHaveBeenCalledTimes(1);
        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "manual"}]);
    });

    it("dispatches window focus recovery to the terminal captured before active pane changes", async () => {
        const terminalInput = appendTerminalInput("pane-1");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();
        recoveryEvents = [];

        await act(async () => {
            await vi.advanceTimersByTimeAsync(IME_RECOVERY_AUTO_COOLDOWN_MS);
        });
        act(() => {
            terminalInput.dispatchEvent(new FocusEvent("focusout", {bubbles: true, relatedTarget: null}));
            window.dispatchEvent(new Event("blur"));
            window.dispatchEvent(new Event("focus"));
        });
        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-2"/>);
        });
        await flushRecoveryTimers();

        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "window-focus"}]);
    });

    it("keeps terminal focus cooldown scoped to each pane", async () => {
        const firstTerminalInput = appendTerminalInput("pane-1");
        const secondTerminalInput = appendTerminalInput("pane-2");
        const otherInput = document.createElement("input");
        document.body.appendChild(otherInput);

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            firstTerminalInput.focus();
        });
        await flushRecoveryTimers();

        act(() => {
            otherInput.focus();
            secondTerminalInput.focus();
        });
        await flushRecoveryTimers();

        expect(recoveryEvents).toEqual([
            {paneId: "pane-1", reason: "terminal-focus"},
            {paneId: "pane-2", reason: "terminal-focus"},
        ]);
    });

    it("skips the suppressed initial terminal focus recovery", async () => {
        const terminalInput = appendTerminalInput("pane-1");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        suppressNextTerminalFocusImeRecovery("pane-1");
        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();

        expect(recoveryEvents).toEqual([]);
    });

    it("does not dispatch terminal focus recovery during IME composition", async () => {
        const terminalInput = appendTerminalInput("pane-1");

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.dispatchEvent(new Event("compositionstart", {bubbles: true}));
            terminalInput.focus();
        });
        await flushRecoveryTimers();

        expect(recoveryEvents).toEqual([]);
    });

    it("restores a generic text input without dispatching terminal recovery", async () => {
        const input = document.createElement("input");
        document.body.appendChild(input);

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            input.focus();
        });

        act(() => {
            useTmuxStore.getState().triggerImeReset();
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).toHaveBeenCalledTimes(1);
        expect(document.activeElement).toBe(input);
        expect(recoveryEvents).toEqual([]);
    });

    it("does not steal focus from non-terminal text inputs during automatic recovery", async () => {
        const terminalInput = appendTerminalInput("pane-1");
        const chatInput = document.createElement("textarea");
        document.body.appendChild(chatInput);

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });
        await flushRecoveryTimers();
        recoveryEvents = [];

        await act(async () => {
            await vi.advanceTimersByTimeAsync(IME_RECOVERY_AUTO_COOLDOWN_MS);
        });
        // The first terminal focus consumes the terminal-focus cooldown; this
        // re-entry path verifies generic text inputs are still preserved after it.
        act(() => {
            terminalInput.dispatchEvent(new FocusEvent("focusout", {bubbles: true, relatedTarget: chatInput}));
            chatInput.focus();
        });
        await flushRecoveryTimers();

        expect(document.activeElement).toBe(chatInput);
        expect(recoveryEvents).toEqual([]);
    });

    it("accepts terminal focus recovery events", () => {
        const validEvent = new CustomEvent(TERMINAL_IME_RECOVERY_EVENT, {
            detail: {paneId: "pane-1", reason: "terminal-focus"},
        });

        expect(isTerminalImeRecoveryEvent(validEvent)).toBe(true);
    });

    it("continues manual recovery when native focus recovery rejects", async () => {
        const terminalInput = appendTerminalInput();

        apiMock.RecoverIMEWindowFocus.mockRejectedValueOnce(new Error("runtime context is nil"));

        act(() => {
            root.render(<AppImeRecoveryProbe activePaneId="pane-1"/>);
        });

        act(() => {
            terminalInput.focus();
        });

        act(() => {
            useTmuxStore.getState().triggerImeReset();
        });
        await flushRecoveryTimers();

        expect(apiMock.RecoverIMEWindowFocus).toHaveBeenCalledTimes(1);
        expect(recoveryEvents).toEqual([{paneId: "pane-1", reason: "manual"}]);
    });

    it("treats a null element as a non-terminal text entry target", () => {
        expect(isTerminalTextEntryElement(null)).toBe(false);
    });

    it("identifies terminal text entry elements inside the active pane", () => {
        const activeTerminalInput = appendTerminalInput("pane-1");
        const inactiveTerminalInput = appendTerminalInput("pane-2");
        const untaggedTerminalInput = appendTerminalInput();
        const genericInput = document.createElement("input");
        document.body.appendChild(genericInput);

        expect(isActiveTerminalTextEntryElement(activeTerminalInput, "pane-1")).toBe(true);
        expect(isActiveTerminalTextEntryElement(inactiveTerminalInput, "pane-1")).toBe(false);
        expect(isActiveTerminalTextEntryElement(untaggedTerminalInput, "pane-1")).toBe(false);
        expect(isActiveTerminalTextEntryElement(genericInput, "pane-1")).toBe(false);
        expect(isActiveTerminalTextEntryElement(activeTerminalInput, null)).toBe(false);
    });

    it("rejects malformed terminal IME recovery events", () => {
        const invalidEvent = new CustomEvent(TERMINAL_IME_RECOVERY_EVENT, {
            detail: {paneId: null, reason: "manual"},
        });

        expect(isTerminalImeRecoveryEvent(invalidEvent)).toBe(false);
    });
});
