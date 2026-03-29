import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useSnapshotSync} from "../src/hooks/sync/useSnapshotSync";
import {useNotificationStore} from "../src/stores/notificationStore";
import {useTmuxStore} from "../src/stores/tmuxStore";

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

const paneDataStreamMock = vi.hoisted(() => ({
    connect: vi.fn(),
    disconnect: vi.fn(),
}));

const apiMock = vi.hoisted(() => ({
    GetActiveSession: vi.fn<() => Promise<string>>(),
    GetWebSocketURL: vi.fn<() => Promise<string>>(),
    ListSessions: vi.fn<() => Promise<unknown[]>>(),
}));

vi.mock("../wailsjs/runtime/runtime", () => runtimeMock);

vi.mock("../src/api", () => ({
    api: {
        GetActiveSession: () => apiMock.GetActiveSession(),
        GetWebSocketURL: () => apiMock.GetWebSocketURL(),
        ListSessions: () => apiMock.ListSessions(),
    },
}));

vi.mock("../src/i18n", () => ({
    getLanguage: vi.fn(() => "en"),
    translate: vi.fn((_key: string, _jaText: string, enText: string) => enText),
}));

vi.mock("../src/services/paneDataStream", () => ({
    connect: (...args: unknown[]) => paneDataStreamMock.connect(...args),
    disconnect: () => paneDataStreamMock.disconnect(),
}));

function SnapshotSyncProbe() {
    useSnapshotSync();
    return null;
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("useSnapshotSync", () => {
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

        apiMock.ListSessions.mockReset();
        apiMock.ListSessions.mockResolvedValue([]);
        apiMock.GetActiveSession.mockReset();
        apiMock.GetActiveSession.mockResolvedValue("");
        apiMock.GetWebSocketURL.mockReset();
        apiMock.GetWebSocketURL.mockResolvedValue("");
        paneDataStreamMock.connect.mockReset();
        paneDataStreamMock.disconnect.mockReset();
        vi.spyOn(console, "warn").mockImplementation(() => undefined);

        useNotificationStore.setState((state) => ({...state, notifications: []}));
        useTmuxStore.setState((state) => ({
            ...state,
            activeSession: null,
            activeWindowId: null,
            imeResetSignal: 0,
            sessionOrder: [],
            sessions: [],
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
        useNotificationStore.setState((state) => ({...state, notifications: []}));
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("adds a warning notification when worktree pull fails", async () => {
        act(() => {
            root.render(<SnapshotSyncProbe/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("worktree:pull-failed");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.({
                sessionName: "feature-ime",
                message: "pull failed, worktree created from local state",
                error: "fatal: remote update rejected",
            });
        });

        const [notification] = useNotificationStore.getState().notifications;
        expect(notification?.level).toBe("warn");
        expect(notification?.message).toContain("feature-ime");
        expect(notification?.message).toContain("local checkout state");
        expect(notification?.message).toContain("fatal: remote update rejected");
    });

    it("ignores malformed worktree pull failure payloads", async () => {
        act(() => {
            root.render(<SnapshotSyncProbe/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("worktree:pull-failed");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.(null);
        });

        expect(useNotificationStore.getState().notifications).toEqual([]);
    });
});
