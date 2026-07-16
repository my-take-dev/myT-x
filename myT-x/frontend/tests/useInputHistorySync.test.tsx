import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useInputHistorySync} from "../src/hooks/sync/useInputHistorySync";
import {useInputHistoryStore} from "../src/stores/inputHistoryStore";
import {useTmuxStore} from "../src/stores/tmuxStore";

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

const apiMock = vi.hoisted(() => ({
    GetInputHistoryForSession: vi.fn<(sessionName: string) => Promise<unknown>>(),
}));

vi.mock("../wailsjs/runtime/runtime", () => runtimeMock);

vi.mock("../src/api", () => ({
    api: {
        GetInputHistoryForSession: (sessionName: string) => apiMock.GetInputHistoryForSession(sessionName),
    },
}));

vi.mock("../src/utils/logFrontendEventSafe", () => ({
    logFrontendEventSafe: vi.fn(),
}));

function InputHistorySyncProbe() {
    useInputHistorySync();
    return null;
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

function snapshot(scopeKey: string, input: string) {
    return {
        scope_key: scopeKey,
        entries: [{
            seq: 1,
            ts: "20260516120000",
            pane_id: "%1",
            input,
            source: "chat",
            session: "session-a",
        }],
    };
}

describe("useInputHistorySync", () => {
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
        apiMock.GetInputHistoryForSession.mockReset();
        useInputHistoryStore.setState({
            scopeKey: "",
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
            readSeqByScope: {},
        });
        useTmuxStore.setState((state) => ({
            ...state,
            activeSession: "session-a",
            sessions: [],
            sessionOrder: [],
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
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("fetches a new snapshot when the active session changes", async () => {
        apiMock.GetInputHistoryForSession
            .mockResolvedValueOnce(snapshot("scope-a", "from a"))
            .mockResolvedValueOnce(snapshot("scope-b", "from b"));

        act(() => {
            root.render(<InputHistorySyncProbe/>);
        });
        await flushEffects();

        expect(apiMock.GetInputHistoryForSession).toHaveBeenCalledWith("session-a");
        expect(useInputHistoryStore.getState().scopeKey).toBe("scope-a");

        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: "session-b"}));
        });
        await flushEffects();

        const state = useInputHistoryStore.getState();
        expect(apiMock.GetInputHistoryForSession).toHaveBeenCalledWith("session-b");
        expect(state.scopeKey).toBe("scope-b");
        expect(state.entries[0]?.input).toBe("from b");
        expect(state.unreadCount).toBe(0);
    });

    it("ignores a stale fetch result after active session changes", async () => {
        let resolveSessionA: (value: unknown) => void = () => undefined;
        const sessionAPromise = new Promise<unknown>((resolve) => {
            resolveSessionA = resolve;
        });
        apiMock.GetInputHistoryForSession
            .mockReturnValueOnce(sessionAPromise)
            .mockResolvedValueOnce(snapshot("scope-b", "from b"));

        act(() => {
            root.render(<InputHistorySyncProbe/>);
        });
        await flushEffects();

        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: "session-b"}));
        });
        await flushEffects();

        await act(async () => {
            resolveSessionA(snapshot("scope-a", "stale a"));
            await Promise.resolve();
        });

        const state = useInputHistoryStore.getState();
        expect(state.scopeKey).toBe("scope-b");
        expect(state.entries[0]?.input).toBe("from b");
    });

    it("ignores an older fetch when the same session name becomes active again", async () => {
        let resolveFirstSessionA: (value: unknown) => void = () => undefined;
        const firstSessionAPromise = new Promise<unknown>((resolve) => {
            resolveFirstSessionA = resolve;
        });
        apiMock.GetInputHistoryForSession
            .mockReturnValueOnce(firstSessionAPromise)
            .mockResolvedValueOnce(snapshot("scope-a", "fresh a"));

        act(() => {
            root.render(<InputHistorySyncProbe/>);
        });
        await flushEffects();

        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: ""}));
        });
        await flushEffects();
        expect(useInputHistoryStore.getState().entries).toHaveLength(0);

        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: "session-a"}));
        });
        await flushEffects();
        expect(useInputHistoryStore.getState().entries[0]?.input).toBe("fresh a");

        await act(async () => {
            resolveFirstSessionA(snapshot("scope-a", "stale a"));
            await Promise.resolve();
        });

        const state = useInputHistoryStore.getState();
        expect(state.scopeKey).toBe("scope-a");
        expect(state.entries[0]?.input).toBe("fresh a");
    });
});
