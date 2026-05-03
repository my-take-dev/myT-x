import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {Terminal} from "@xterm/xterm";
import {setupPaneDataStream} from "../src/hooks/useTerminalPaneData";
import type {TerminalEventShared} from "../src/hooks/useTerminalKeyHandler";

const registerPaneHandlerMock = vi.hoisted(() => vi.fn());
const registerReconnectCallbackMock = vi.hoisted(() => vi.fn());
const isConnectedMock = vi.hoisted(() => vi.fn(() => false));
const eventsOnMock = vi.hoisted(() => vi.fn());
const unregisterPaneMock = vi.hoisted(() => vi.fn());
const unregisterReconnectMock = vi.hoisted(() => vi.fn());
const cancelIpcListenerMock = vi.hoisted(() => vi.fn());
const paneHandlers = vi.hoisted(() => new Map<string, (rawData: Uint8Array) => void>());
const ipcHandlers = vi.hoisted(() => new Map<string, (data: unknown) => void>());
const reconnectCallbacks = vi.hoisted(() => [] as (() => void)[]);

vi.mock("../src/services/paneDataStream", () => ({
    registerPaneHandler: (paneId: string, handler: (rawData: Uint8Array) => void) => {
        registerPaneHandlerMock(paneId, handler);
        paneHandlers.set(paneId, handler);
        return unregisterPaneMock;
    },
    isConnected: () => isConnectedMock(),
    registerReconnectCallback: (callback: () => void, owner?: string) => {
        registerReconnectCallbackMock(callback, owner);
        reconnectCallbacks.push(callback);
        return unregisterReconnectMock;
    },
}));

vi.mock("../wailsjs/runtime/runtime", () => ({
    EventsOn: (eventName: string, handler: (data: unknown) => void) => {
        eventsOnMock(eventName, handler);
        ipcHandlers.set(eventName, handler);
        return cancelIpcListenerMock;
    },
}));

function createShared(): TerminalEventShared {
    return {
        disposed: false,
        // Forces synchronous term.write(), avoiding RAF scheduling in most unit tests.
        pageHidden: true,
        isComposing: false,
        composingOutput: [],
    };
}

function encodeText(value: string): Uint8Array {
    return new TextEncoder().encode(value);
}

function terminalWrite(term: Terminal): ReturnType<typeof vi.fn> {
    return (term as unknown as {write: ReturnType<typeof vi.fn>}).write;
}

describe("setupPaneDataStream output streaming", () => {
    beforeEach(() => {
        registerPaneHandlerMock.mockClear();
        registerReconnectCallbackMock.mockClear();
        isConnectedMock.mockReset();
        isConnectedMock.mockReturnValue(false);
        eventsOnMock.mockClear();
        unregisterPaneMock.mockClear();
        unregisterReconnectMock.mockClear();
        cancelIpcListenerMock.mockClear();
        paneHandlers.clear();
        ipcHandlers.clear();
        reconnectCallbacks.length = 0;
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it("preserves live WebSocket scrollback purge data", () => {
        const term = {write: vi.fn()} as unknown as Terminal;
        const cleanup = setupPaneDataStream({term, shared: createShared(), paneId: "%1"});

        try {
            paneHandlers.get("%1")?.(encodeText("before\x1b[3Jafter"));

            expect(terminalWrite(term)).toHaveBeenCalledWith("before\x1b[3Jafter");
        } finally {
            cleanup();
        }
    });

    it("preserves split live purge sequences across WebSocket chunks", () => {
        const term = {write: vi.fn()} as unknown as Terminal;
        const cleanup = setupPaneDataStream({term, shared: createShared(), paneId: "%1"});

        try {
            paneHandlers.get("%1")?.(encodeText("before\x1b["));
            paneHandlers.get("%1")?.(encodeText("?3Jafter"));

            const write = terminalWrite(term);
            expect(write).toHaveBeenNthCalledWith(1, "before\x1b[");
            expect(write).toHaveBeenNthCalledWith(2, "?3Jafter");
        } finally {
            cleanup();
        }
    });

    it("preserves live IPC fallback scrollback purge data", () => {
        const term = {write: vi.fn()} as unknown as Terminal;
        const cleanup = setupPaneDataStream({term, shared: createShared(), paneId: "%1"});

        try {
            ipcHandlers.get("pane:data:%1")?.("before\x1b[3Jafter");

            expect(terminalWrite(term)).toHaveBeenCalledWith("before\x1b[3Jafter");
        } finally {
            cleanup();
        }
    });

    it("writes foreground WebSocket data through RAF without sanitizing terminal controls", () => {
        const term = {write: vi.fn()} as unknown as Terminal;
        const shared = createShared();
        shared.pageHidden = false;
        const requestAnimationFrameSpy = vi
            .spyOn(window, "requestAnimationFrame")
            .mockImplementation((callback: FrameRequestCallback) => {
                callback(0);
                return 1;
            });
        const cancelAnimationFrameSpy = vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
        const cleanup = setupPaneDataStream({term, shared, paneId: "%1"});

        try {
            paneHandlers.get("%1")?.(encodeText("before\x1b[3Jafter"));

            expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(1);
            expect(terminalWrite(term)).toHaveBeenCalledWith("before\x1b[3Jafter");
        } finally {
            cleanup();
            cancelAnimationFrameSpy.mockRestore();
            requestAnimationFrameSpy.mockRestore();
        }
    });

    it("runs every mounted pane reconnect callback", () => {
        const firstTerm = {write: vi.fn()} as unknown as Terminal;
        const secondTerm = {write: vi.fn()} as unknown as Terminal;
        const firstCleanup = setupPaneDataStream({term: firstTerm, shared: createShared(), paneId: "%1"});
        const secondCleanup = setupPaneDataStream({term: secondTerm, shared: createShared(), paneId: "%2"});

        try {
            paneHandlers.get("%1")?.(encodeText("first"));
            paneHandlers.get("%2")?.(encodeText("second"));

            reconnectCallbacks.forEach((callback) => callback());
            isConnectedMock.mockReturnValue(true);
            ipcHandlers.get("pane:data:%1")?.("ipc-1");
            ipcHandlers.get("pane:data:%2")?.("ipc-2");

            expect(terminalWrite(firstTerm)).toHaveBeenLastCalledWith("ipc-1");
            expect(terminalWrite(secondTerm)).toHaveBeenLastCalledWith("ipc-2");
        } finally {
            secondCleanup();
            firstCleanup();
        }
    });

    it("unregisters only the current pane reconnect callback during cleanup", () => {
        const term = {write: vi.fn()} as unknown as Terminal;
        const cleanup = setupPaneDataStream({term, shared: createShared(), paneId: "%1"});

        try {
            expect(registerReconnectCallbackMock).toHaveBeenCalledWith(expect.any(Function), "pane:%1");
        } finally {
            cleanup();
        }

        expect(unregisterReconnectMock).toHaveBeenCalledTimes(1);
    });
});
