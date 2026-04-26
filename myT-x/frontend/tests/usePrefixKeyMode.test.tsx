import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {usePrefixKeyMode} from "../src/hooks/usePrefixKeyMode";
import {useTmuxStore} from "../src/stores/tmuxStore";
import type {SessionSnapshot} from "../src/types/tmux";
import {
    isTerminalImeRecoveryEvent,
    TERMINAL_IME_RECOVERY_EVENT,
    TERMINAL_PANE_ID_ATTRIBUTE,
    type TerminalImeRecoveryDetail,
} from "../src/utils/imeRecovery";

const apiMock = vi.hoisted(() => ({
    FocusPane: vi.fn<() => Promise<void>>(),
}));

vi.mock("../src/api", () => ({
    api: {
        FocusPane: (..._args: unknown[]) => apiMock.FocusPane(),
    },
}));

function PrefixProbe({activePaneId}: {readonly activePaneId: string | null}) {
    usePrefixKeyMode({activePaneId});
    return null;
}

function sessionSnapshot(): SessionSnapshot {
    return {
        id: 1,
        name: "session-1",
        created_at: "2026-04-15T10:00:00Z",
        is_idle: false,
        active_window_id: 1,
        windows: [
            {
                id: 1,
                name: "main",
                active_pane: 1,
                panes: [
                    {id: "%1", index: 0, title: "left", active: true, width: 80, height: 24},
                    {id: "%2", index: 1, title: "right", active: false, width: 80, height: 24},
                ],
            },
        ],
    };
}

function appendTerminalPane(paneId: string): HTMLTextAreaElement {
    const pane = document.createElement("div");
    pane.setAttribute(TERMINAL_PANE_ID_ATTRIBUTE, paneId);
    const xterm = document.createElement("div");
    xterm.className = "xterm";
    const textarea = document.createElement("textarea");
    xterm.appendChild(textarea);
    pane.appendChild(xterm);
    document.body.appendChild(pane);
    return textarea;
}

describe("usePrefixKeyMode terminal text focus recovery", () => {
    let container: HTMLDivElement;
    let root: Root;
    let recoveryEvents: TerminalImeRecoveryDetail[];
    let recoveryListener: (event: Event) => void;
    let requestAnimationFrameSpy: ReturnType<typeof vi.spyOn>;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        apiMock.FocusPane.mockReset();
        apiMock.FocusPane.mockResolvedValue(undefined);
        useTmuxStore.setState({
            sessions: [sessionSnapshot()],
            activeSession: "session-1",
            activeWindowId: "1",
            prefixMode: false,
            zoomPaneId: null,
            pendingPrefixKillPaneId: null,
            syncInputMode: false,
        });
        recoveryEvents = [];
        recoveryListener = (event: Event) => {
            if (isTerminalImeRecoveryEvent(event)) {
                recoveryEvents.push(event.detail);
            }
        };
        window.addEventListener(TERMINAL_IME_RECOVERY_EVENT, recoveryListener);
        requestAnimationFrameSpy = vi.spyOn(window, "requestAnimationFrame")
            .mockImplementation((callback: FrameRequestCallback) => {
                callback(0);
                return 1;
            });
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);
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
        requestAnimationFrameSpy.mockRestore();
        consoleWarnSpy.mockRestore();
        container.remove();
        document.body.innerHTML = "";
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("does not dispatch IME recovery when prefix pane navigation focuses text entry", async () => {
        appendTerminalPane("%1");
        const targetInput = appendTerminalPane("%2");
        act(() => {
            root.render(<PrefixProbe activePaneId="%1"/>);
        });

        await act(async () => {
            window.dispatchEvent(new KeyboardEvent("keydown", {key: "b", ctrlKey: true, bubbles: true}));
            window.dispatchEvent(new KeyboardEvent("keydown", {key: "ArrowRight", bubbles: true}));
            await Promise.resolve();
        });

        expect(apiMock.FocusPane).toHaveBeenCalledTimes(1);
        expect(document.activeElement).toBe(targetInput);
        expect(recoveryEvents).toEqual([]);
    });

    it("dispatches pane-targeted IME recovery after backend focus when text entry focus fails", async () => {
        appendTerminalPane("%1");
        appendTerminalPane("%2");
        const focusSpy = vi.spyOn(HTMLTextAreaElement.prototype, "focus").mockImplementation(() => undefined);
        act(() => {
            root.render(<PrefixProbe activePaneId="%1"/>);
        });

        await act(async () => {
            window.dispatchEvent(new KeyboardEvent("keydown", {key: "b", ctrlKey: true, bubbles: true}));
            window.dispatchEvent(new KeyboardEvent("keydown", {key: "ArrowRight", bubbles: true}));
            await Promise.resolve();
        });

        expect(apiMock.FocusPane).toHaveBeenCalledTimes(1);
        expect(focusSpy).toHaveBeenCalled();
        expect(recoveryEvents).toEqual([{paneId: "%2", reason: "terminal-focus"}]);
        expect(consoleWarnSpy).toHaveBeenCalledWith("[prefix] Focus pane text entry focus failed", {paneId: "%2"});
    });
});
