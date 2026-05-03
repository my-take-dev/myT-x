import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useSessionMemoStore} from "../../../../stores/sessionMemoStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {useSessionMemo} from "./useSessionMemo";

const loadSessionMemoMock = vi.fn<(sessionName: string) => Promise<string>>();
const saveSessionMemoMock = vi.fn<(sessionName: string, memo: string) => Promise<void>>();

vi.mock("../../../../api", () => ({
    api: {
        LoadSessionMemo: (sessionName: string) => loadSessionMemoMock(sessionName),
        SaveSessionMemo: (sessionName: string, memo: string) => saveSessionMemoMock(sessionName, memo),
    },
}));

let latestHook: ReturnType<typeof useSessionMemo> | null = null;

function HookHarness() {
    latestHook = useSessionMemo();
    return null;
}

function createHookRoot() {
    const container = document.createElement("div");
    document.body.appendChild(container);
    return {
        container,
        root: createRoot(container),
    };
}

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

describe("useSessionMemo", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        const hookRoot = createHookRoot();
        container = hookRoot.container;
        root = hookRoot.root;
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        latestHook = null;
        loadSessionMemoMock.mockReset();
        saveSessionMemoMock.mockReset();
        useSessionMemoStore.setState({drafts: {}});
        useNotificationStore.setState({notifications: []});
        useTmuxStore.setState({
            config: null,
            sessions: [
                {id: 1, name: "alpha", created_at: "", is_idle: false, active_window_id: 0, windows: []},
                {id: 2, name: "beta", created_at: "", is_idle: false, active_window_id: 0, windows: []},
            ],
            sessionOrder: ["alpha", "beta"],
            activeSession: "alpha",
            activeWindowId: null,
            zoomPaneId: null,
            pendingPrefixKillPaneId: null,
            prefixMode: false,
            syncInputMode: false,
            fontSize: 13,
            imeResetSignal: 0,
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("loads a persisted memo once and keeps edits in memory", async () => {
        loadSessionMemoMock.mockResolvedValue("persisted");

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(latestHook?.content).toBe("persisted");

        act(() => {
            latestHook?.updateContent("draft");
        });
        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: "beta"}));
        });
        await act(async () => {
            await Promise.resolve();
        });
        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: "alpha"}));
        });

        expect(latestHook?.content).toBe("draft");
        expect(loadSessionMemoMock.mock.calls.filter(([sessionName]) => sessionName === "alpha")).toHaveLength(1);
    });

    it("saves the current session memo and marks it clean", async () => {
        loadSessionMemoMock.mockResolvedValue("");
        saveSessionMemoMock.mockResolvedValue();

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("meeting notes");
        });
        expect(latestHook?.isDirty).toBe(true);

        await act(async () => {
            await latestHook?.save();
        });

        expect(saveSessionMemoMock).toHaveBeenCalledWith("alpha", "meeting notes");
        expect(latestHook?.isDirty).toBe(false);
        expect(useNotificationStore.getState().notifications[0]?.message).toBe("セッションメモを保存しました。");
    });

    it("does not discard text typed while save is in flight", async () => {
        const saveRequest = deferred<void>();
        loadSessionMemoMock.mockResolvedValue("");
        saveSessionMemoMock.mockReturnValue(saveRequest.promise);

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("abc");
        });
        let savePromise: Promise<void> | undefined;
        act(() => {
            savePromise = latestHook?.save();
        });
        act(() => {
            latestHook?.updateContent("abcdef");
        });
        await act(async () => {
            saveRequest.resolve();
            await savePromise;
        });

        expect(saveSessionMemoMock).toHaveBeenCalledWith("alpha", "abc");
        expect(latestHook?.content).toBe("abcdef");
        expect(latestHook?.isDirty).toBe(true);
    });

    it("shows an inline error without notification when loading fails", async () => {
        loadSessionMemoMock.mockRejectedValue(new Error("boom"));

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(latestHook?.error).toContain("boom");
        expect(latestHook?.isDirty).toBe(false);
        expect(useNotificationStore.getState().notifications).toHaveLength(0);
    });

    it("keeps the dirty draft and shows an inline error when saving fails", async () => {
        loadSessionMemoMock.mockResolvedValue("");
        saveSessionMemoMock.mockRejectedValue(new Error("disk full"));

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("meeting notes");
        });
        await act(async () => {
            await latestHook?.save();
        });

        expect(latestHook?.content).toBe("meeting notes");
        expect(latestHook?.isDirty).toBe(true);
        expect(latestHook?.error).toContain("disk full");
        expect(useNotificationStore.getState().notifications[0]?.message).toContain("disk full");
    });

    it("adds a notification when a manual refresh fails", async () => {
        loadSessionMemoMock
            .mockResolvedValueOnce("")
            .mockRejectedValueOnce(new Error("reload failed"));

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        await act(async () => {
            await expect(latestHook?.refresh(true)).rejects.toThrow("reload failed");
        });

        expect(latestHook?.error).toContain("reload failed");
        expect(useNotificationStore.getState().notifications[0]?.message).toBe("Failed to reload the session memo.");
    });

    it("ignores stale load responses after the active session changes", async () => {
        const alphaLoad = deferred<string>();
        const betaLoad = deferred<string>();
        loadSessionMemoMock
            .mockReturnValueOnce(alphaLoad.promise)
            .mockReturnValueOnce(betaLoad.promise);

        await act(async () => {
            root.render(<HookHarness/>);
        });

        act(() => {
            useTmuxStore.setState((state) => ({...state, activeSession: "beta"}));
        });
        await act(async () => {
            betaLoad.resolve("beta memo");
            await Promise.resolve();
        });
        await act(async () => {
            alphaLoad.resolve("alpha stale");
            await Promise.resolve();
        });

        expect(latestHook?.activeSession).toBe("beta");
        expect(latestHook?.content).toBe("beta memo");
        expect(useSessionMemoStore.getState().drafts["alpha:1"]).toBeUndefined();
    });

    it("keeps the active session draft when the session is renamed", async () => {
        loadSessionMemoMock.mockResolvedValue("persisted");

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("renamed draft");
        });
        act(() => {
            useTmuxStore.setState((state) => ({
                ...state,
                activeSession: "renamed-alpha",
                sessions: [
                    {id: 1, name: "renamed-alpha", created_at: "", is_idle: false, active_window_id: 0, windows: []},
                    {id: 2, name: "beta", created_at: "", is_idle: false, active_window_id: 0, windows: []},
                ],
                sessionOrder: ["renamed-alpha", "beta"],
            }));
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(latestHook?.activeSession).toBe("renamed-alpha");
        expect(latestHook?.content).toBe("renamed draft");
        expect(useSessionMemoStore.getState().drafts["alpha:1"]).toBeUndefined();
        expect(useSessionMemoStore.getState().drafts["renamed-alpha:1"]?.content).toBe("renamed draft");
    });

    it("keeps the active draft when active session changes before the renamed snapshot arrives", async () => {
        loadSessionMemoMock.mockResolvedValue("persisted");

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("active-first renamed draft");
        });
        act(() => {
            useTmuxStore.setState((state) => ({
                ...state,
                activeSession: "renamed-alpha",
            }));
        });
        await act(async () => {
            await Promise.resolve();
        });
        act(() => {
            useTmuxStore.setState((state) => ({
                ...state,
                sessions: [
                    {id: 1, name: "renamed-alpha", created_at: "", is_idle: false, active_window_id: 0, windows: []},
                    {id: 2, name: "beta", created_at: "", is_idle: false, active_window_id: 0, windows: []},
                ],
                sessionOrder: ["renamed-alpha", "beta"],
            }));
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(latestHook?.activeSession).toBe("renamed-alpha");
        expect(latestHook?.content).toBe("active-first renamed draft");
        expect(useSessionMemoStore.getState().drafts["alpha:1"]).toBeUndefined();
        expect(useSessionMemoStore.getState().drafts["renamed-alpha:1"]?.content).toBe("active-first renamed draft");
    });

    it("ignores an in-flight refresh after save starts", async () => {
        const refreshRequest = deferred<string>();
        loadSessionMemoMock
            .mockResolvedValueOnce("")
            .mockReturnValueOnce(refreshRequest.promise);
        saveSessionMemoMock.mockResolvedValue();

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("new memo");
        });
        let refreshPromise: Promise<void> | undefined;
        act(() => {
            refreshPromise = latestHook?.refresh(true);
        });
        await act(async () => {
            await latestHook?.save();
        });
        await act(async () => {
            refreshRequest.resolve("stale memo");
            await refreshPromise;
        });

        expect(latestHook?.content).toBe("new memo");
        expect(latestHook?.isDirty).toBe(false);
    });

    it("rejects content that exceeds the backend UTF-8 byte limit", async () => {
        loadSessionMemoMock.mockResolvedValue("");

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        act(() => {
            latestHook?.updateContent("界".repeat(349_526));
        });

        expect(latestHook?.content).toBe("");
        expect(latestHook?.error).toBe("Session memo must be 1 MiB or smaller.");
    });
});
