import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useSnapshotSync} from "../src/hooks/sync/useSnapshotSync";
import {useCanvasStore} from "../src/stores/canvasStore";
import {useDiffReviewStore} from "../src/stores/diffReviewStore";
import {useMCPStore} from "../src/stores/mcpStore";
import {useNotificationStore} from "../src/stores/notificationStore";
import {useTmuxStore} from "../src/stores/tmuxStore";
import {buildDiffReviewDraftKey, buildDiffReviewSessionKey} from "../src/components/viewer/views/diff-view/diffReviewKeys";

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
    translate: vi.fn((_: string, defaultText: string, params?: Record<string, string | number>) =>
        defaultText.replace(/\{(\w+)}/g, (_, key: string) => String(params?.[key] ?? "")),
    ),
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
        useCanvasStore.setState((state) => ({
            ...state,
            activeSessionName: null,
            nodePositions: {},
            nodeSizes: {},
            taskEdgeMap: {},
            agentMap: {},
            processStatusMap: {},
            rootPaneId: null,
            sessionDataMap: {},
        }));
        useMCPStore.setState({
            snapshots: {},
            sessionStates: {},
        });
        useDiffReviewStore.setState((state) => ({
            ...state,
            comments: [],
            drafts: {},
            activeCommentLineKey: null,
        }));
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

    it("adds a warning notification when session cleanup is degraded", async () => {
        act(() => {
            root.render(<SnapshotSyncProbe/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("session:cleanup-degraded");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.({
                component: "devpanel",
                session_name: "feature-ime",
                message: "cleanup timed out",
            });
        });

        const [notification] = useNotificationStore.getState().notifications;
        expect(notification?.level).toBe("warn");
        expect(notification?.message).toContain("feature-ime");
        expect(notification?.message).toContain("devpanel");
        expect(notification?.message).toContain("cleanup timed out");
    });

    it.each([
        null,
        {},
        {component: "devpanel"},
        {component: "devpanel", session_name: "feature-ime"},
        {component: "devpanel", session_name: "feature-ime", message: ""},
    ])("ignores malformed session cleanup payloads: %o", async (payload) => {
        act(() => {
            root.render(<SnapshotSyncProbe/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("session:cleanup-degraded");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.(payload);
        });

        expect(useNotificationStore.getState().notifications).toEqual([]);
        expect(console.warn).toHaveBeenCalledWith(expect.stringContaining("session:cleanup-degraded"));
    });

    it("migrates session-scoped frontend state on tmux:session-renamed", async () => {
        useCanvasStore.setState((state) => ({
            ...state,
            activeSessionName: "old-session",
            nodePositions: {"%1": {x: 10, y: 20}},
            rootPaneId: "%1",
            sessionDataMap: {
                parked: {
                    nodePositions: {"%9": {x: 90, y: 100}},
                    nodeSizes: {},
                    taskEdgeMap: {},
                    agentMap: {},
                    processStatusMap: {},
                    rootPaneId: "%9",
                },
            },
        }));
        useMCPStore.setState({
            snapshots: {
                "old-session": [{id: "mcp-1", name: "MCP 1", description: "", enabled: true, status: "running"}],
            },
            sessionStates: {
                "old-session": {loading: false, error: "stale"},
            },
        });

        act(() => {
            root.render(<SnapshotSyncProbe/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("tmux:session-renamed");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.({oldName: "old-session", newName: "renamed-session"});
        });

        expect(useCanvasStore.getState().activeSessionName).toBe("renamed-session");
        expect(useCanvasStore.getState().rootPaneId).toBe("%1");
        expect(useMCPStore.getState().snapshots["old-session"]).toBeUndefined();
        expect(useMCPStore.getState().snapshots["renamed-session"]?.[0]?.id).toBe("mcp-1");
        expect(useMCPStore.getState().sessionStates["renamed-session"]?.error).toBe("stale");
    });

    it("clears diff review state when tmux:session-destroyed arrives", async () => {
        apiMock.ListSessions.mockResolvedValueOnce([
            {id: 7, name: "old-session", created_at: "", is_idle: false, active_window_id: 1, windows: []},
        ]);
        useTmuxStore.setState((state) => ({
            ...state,
            sessions: [{id: 7, name: "old-session", created_at: "", is_idle: false, active_window_id: 1, windows: []}],
            sessionOrder: ["old-session"],
            activeSession: "old-session",
            activeWindowId: "1",
        }));
        useDiffReviewStore.getState().addComment({
            sessionKey: buildDiffReviewSessionKey(7),
            filePath: "a.ts",
            startLineNum: 1,
            startLineType: "added",
            endLineNum: 1,
            endLineType: "added",
            lineContent: "const a = 1;",
            commentText: "remove me",
        });
        useDiffReviewStore.getState().setDraft(
            buildDiffReviewDraftKey(buildDiffReviewSessionKey(7), "a.ts", "hunk:1:1:0"),
            "draft text",
        );

        act(() => {
            root.render(<SnapshotSyncProbe/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("tmux:session-destroyed");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.({name: "old-session"});
        });

        expect(useDiffReviewStore.getState().comments).toEqual([]);
        expect(useDiffReviewStore.getState().drafts).toEqual({});
        expect(useDiffReviewStore.getState().activeCommentLineKey).toBeNull();
    });
});
