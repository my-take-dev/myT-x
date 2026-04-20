import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useCanvasTaskSync} from "../src/hooks/useCanvasTaskSync";
import {useCanvasStore} from "../src/stores/canvasStore";

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

const apiMock = vi.hoisted(() => ({
    ListOrchestratorTasks: vi.fn<() => Promise<unknown[]>>(),
    ListOrchestratorAgents: vi.fn<() => Promise<unknown[]>>(),
    GetPaneProcessStatus: vi.fn<() => Promise<unknown[]>>(),
}));

vi.mock("../wailsjs/runtime/runtime", () => runtimeMock);

vi.mock("../src/api", () => ({
    api: {
        ListOrchestratorTasks: (sessionName: string) => apiMock.ListOrchestratorTasks(sessionName),
        ListOrchestratorAgents: (sessionName: string) => apiMock.ListOrchestratorAgents(sessionName),
        GetPaneProcessStatus: (sessionName: string) => apiMock.GetPaneProcessStatus(sessionName),
    },
}));

function CanvasTaskSyncProbe({sessionName}: {sessionName: string | null}) {
    useCanvasTaskSync(sessionName);
    return null;
}

function createDeferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
    });
}

describe("useCanvasTaskSync", () => {
    let container: HTMLDivElement;
    let root: Root;
    let eventHandlers: Map<string, (payload: unknown) => void>;

    beforeEach(() => {
        eventHandlers = new Map<string, (payload: unknown) => void>();
        runtimeMock.EventsOn.mockImplementation((eventName: string, handler: (payload: unknown) => void) => {
            eventHandlers.set(eventName, handler);
            return () => {
                eventHandlers.delete(eventName);
            };
        });

        apiMock.ListOrchestratorTasks.mockReset();
        apiMock.ListOrchestratorAgents.mockReset();
        apiMock.GetPaneProcessStatus.mockReset();
        apiMock.ListOrchestratorTasks.mockResolvedValue([]);
        apiMock.ListOrchestratorAgents.mockResolvedValue([]);
        apiMock.GetPaneProcessStatus.mockResolvedValue([]);

        useCanvasStore.setState((state) => ({
            ...state,
            mode: "canvas",
            activeSessionName: null,
            nodePositions: {},
            nodeSizes: {},
            taskEdgeMap: {},
            agentMap: {},
            processStatusMap: {},
            rootPaneId: null,
            sessionDataMap: {},
        }));

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
        vi.useRealTimers();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("polls immediately when orchestrator agents are updated for the active session", async () => {
        apiMock.ListOrchestratorAgents
            .mockResolvedValueOnce([])
            .mockResolvedValueOnce([{name: "worker", pane_id: "%2", role: "developer"}]);

        act(() => {
            root.render(<CanvasTaskSyncProbe sessionName="alpha"/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");
        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);

        act(() => {
            handler?.({sessionName: "alpha"});
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(2);
        expect(useCanvasStore.getState().agentMap["%2"]?.name).toBe("worker");
    });

    it("ignores orchestrator agent updates for other sessions", async () => {
        act(() => {
            root.render(<CanvasTaskSyncProbe sessionName="alpha"/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");
        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);

        act(() => {
            handler?.({sessionName: "beta"});
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);
    });

    it("ignores orchestrator agent updates without a session payload", async () => {
        act(() => {
            root.render(<CanvasTaskSyncProbe sessionName="alpha"/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");
        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);

        act(() => {
            handler?.({});
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);
    });

    it("does not create an extra polling loop after an event-driven refresh", async () => {
        vi.useFakeTimers();

        act(() => {
            root.render(<CanvasTaskSyncProbe sessionName="alpha"/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");
        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);

        act(() => {
            handler?.({sessionName: "alpha"});
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(2);

        await act(async () => {
            await vi.advanceTimersByTimeAsync(3000);
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(3);
    });

    it("queues refreshes triggered during an in-flight poll instead of starting concurrent polls", async () => {
        vi.useFakeTimers();

        const firstAgentPoll = createDeferred<unknown[]>();
        const queuedAgentPoll = createDeferred<unknown[]>();
        apiMock.ListOrchestratorAgents
            .mockImplementationOnce(() => firstAgentPoll.promise)
            .mockImplementationOnce(() => queuedAgentPoll.promise);

        act(() => {
            root.render(<CanvasTaskSyncProbe sessionName="alpha"/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");
        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);

        act(() => {
            handler?.({sessionName: "alpha"});
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(1);

        await act(async () => {
            firstAgentPoll.resolve([]);
            await Promise.resolve();
            await Promise.resolve();
        });

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0);
        });
        await flushEffects();

        expect(apiMock.ListOrchestratorAgents).toHaveBeenCalledTimes(2);

        await act(async () => {
            queuedAgentPoll.resolve([{name: "worker", pane_id: "%2", role: "developer"}]);
            await Promise.resolve();
            await Promise.resolve();
        });
        await flushEffects();

        expect(useCanvasStore.getState().agentMap["%2"]?.name).toBe("worker");
    });

    it("clears stale agent roles when the agents poll fails but other canvas polls succeed", async () => {
        apiMock.ListOrchestratorAgents
            .mockResolvedValueOnce([{name: "worker", pane_id: "%2", role: "orchestrator"}])
            .mockRejectedValueOnce(new Error("agents endpoint unavailable"));

        act(() => {
            root.render(<CanvasTaskSyncProbe sessionName="alpha"/>);
        });
        await flushEffects();

        expect(useCanvasStore.getState().agentMap["%2"]?.role).toBe("orchestrator");

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.({sessionName: "alpha"});
        });
        await flushEffects();

        expect(useCanvasStore.getState().agentMap).toEqual({});
    });
});
