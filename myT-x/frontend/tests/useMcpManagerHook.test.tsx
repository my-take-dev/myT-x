import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const apiMock = vi.hoisted(() => ({
    ListMCPServers: vi.fn<(sessionName: string) => Promise<unknown[]>>(),
}));

const notificationMock = vi.hoisted(() => ({
    addNotification: vi.fn(),
}));

let mockActiveSession = "session-a";
let mockSessions = [
    {id: 1, name: "session-a"},
    {id: 2, name: "session-b"},
];

const mcpStoreState = vi.hoisted(() => ({
    snapshots: {} as Record<string, unknown[]>,
    sessionStates: {} as Record<string, {loading?: boolean; error?: string | null}>,
    setSnapshots: vi.fn((sessionName: string, snapshots: unknown[]) => {
        mcpStoreState.snapshots[sessionName] = snapshots;
    }),
    beginSessionLoad: vi.fn((sessionName: string) => {
        const current = mcpStoreState.sessionStates[sessionName] ?? {};
        mcpStoreState.sessionStates[sessionName] = {...current, error: null, loading: true};
    }),
    setSessionLoading: vi.fn((sessionName: string, loading: boolean) => {
        const current = mcpStoreState.sessionStates[sessionName] ?? {};
        mcpStoreState.sessionStates[sessionName] = {...current, loading};
    }),
    setSessionError: vi.fn((sessionName: string, error: string | null) => {
        const current = mcpStoreState.sessionStates[sessionName] ?? {};
        mcpStoreState.sessionStates[sessionName] = {...current, error};
    }),
}));

vi.mock("../src/api", () => ({
    api: {
        ListMCPServers: (...args: unknown[]) => apiMock.ListMCPServers(...args),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {sessions: unknown[]; activeSession: string | null}) => unknown) => (
        selector({sessions: mockSessions, activeSession: mockActiveSession})
    ),
}));

vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: (selector: (state: {addNotification: typeof notificationMock.addNotification}) => unknown) => (
        selector({addNotification: notificationMock.addNotification})
    ),
}));

vi.mock("../src/stores/mcpStore", () => ({
    useMCPStore: (selector: (state: typeof mcpStoreState) => unknown) => selector(mcpStoreState),
}));

import {useMcpManager} from "../src/components/viewer/views/mcp-manager/useMcpManager";

let hookResult: ReturnType<typeof useMcpManager> | null = null;

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

function makeSnapshot(id: string) {
    return {
        id,
        name: id,
        description: "",
        enabled: true,
        status: "running",
    };
}

function Probe() {
    hookResult = useMcpManager();
    return <div />;
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("useMcpManager", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "session-a";
        mockSessions = [
            {id: 1, name: "session-a"},
            {id: 2, name: "session-b"},
        ];
        apiMock.ListMCPServers.mockReset();
        notificationMock.addNotification.mockReset();
        mcpStoreState.snapshots = {};
        mcpStoreState.sessionStates = {};
        mcpStoreState.setSnapshots.mockClear();
        mcpStoreState.beginSessionLoad.mockClear();
        mcpStoreState.setSessionLoading.mockClear();
        mcpStoreState.setSessionError.mockClear();
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("ignores stale same-session loads after retryLoad starts a newer request", async () => {
        const firstLoad = deferred<unknown[]>();
        const secondLoad = deferred<unknown[]>();
        apiMock.ListMCPServers
            .mockReturnValueOnce(firstLoad.promise)
            .mockReturnValueOnce(secondLoad.promise);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mcpStoreState.setSnapshots.mockClear();
        mcpStoreState.setSessionLoading.mockClear();

        await act(async () => {
            hookResult!.retryLoad();
            await Promise.resolve();
        });

        await act(async () => {
            firstLoad.resolve([makeSnapshot("stale")]);
            await firstLoad.promise;
        });
        await flushEffects();

        expect(mcpStoreState.setSnapshots).not.toHaveBeenCalled();
        expect(mcpStoreState.setSessionLoading).not.toHaveBeenCalledWith("session-a", false);

        await act(async () => {
            secondLoad.resolve([makeSnapshot("fresh")]);
            await secondLoad.promise;
        });
        await flushEffects();

        expect(mcpStoreState.setSnapshots).toHaveBeenCalledWith("session-a", [makeSnapshot("fresh")]);
        expect(mcpStoreState.setSessionLoading).toHaveBeenCalledWith("session-a", false);
    });

    it("ignores stale same-session load failures after retryLoad succeeds", async () => {
        const firstLoad = deferred<unknown[]>();
        const secondLoad = deferred<unknown[]>();
        apiMock.ListMCPServers
            .mockReturnValueOnce(firstLoad.promise)
            .mockReturnValueOnce(secondLoad.promise);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mcpStoreState.setSnapshots.mockClear();
        mcpStoreState.setSessionError.mockClear();
        mcpStoreState.setSessionLoading.mockClear();
        notificationMock.addNotification.mockClear();

        await act(async () => {
            hookResult!.retryLoad();
            await Promise.resolve();
        });

        await act(async () => {
            secondLoad.resolve([makeSnapshot("fresh")]);
            await secondLoad.promise;
        });
        await flushEffects();

        await act(async () => {
            firstLoad.reject(new Error("stale failure"));
            try {
                await firstLoad.promise;
            } catch {
                // The hook swallows stale failures after checking load token ordering.
            }
        });
        await flushEffects();

        expect(mcpStoreState.setSnapshots).toHaveBeenCalledWith("session-a", [makeSnapshot("fresh")]);
        expect(mcpStoreState.setSessionError).not.toHaveBeenCalled();
        expect(notificationMock.addNotification).not.toHaveBeenCalled();
    });
});
