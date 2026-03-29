import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useAppImeRecovery} from "../src/hooks/useAppImeRecovery";
import {useTmuxStore} from "../src/stores/tmuxStore";
import {
    isTerminalTextEntryElement,
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

function appendTerminalInput(): HTMLTextAreaElement {
    const terminalElement = document.createElement("div");
    terminalElement.className = "xterm";
    const terminalInput = document.createElement("textarea");
    terminalElement.appendChild(terminalInput);
    document.body.appendChild(terminalElement);
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

    it("rejects malformed terminal IME recovery events", () => {
        const invalidEvent = new CustomEvent(TERMINAL_IME_RECOVERY_EVENT, {
            detail: {paneId: null, reason: "manual"},
        });

        expect(isTerminalImeRecoveryEvent(invalidEvent)).toBe(false);
    });
});
