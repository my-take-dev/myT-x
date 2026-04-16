import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const apiMock = vi.hoisted(() => ({
    DevPanelListDir: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelReadFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelStartWatcher: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelStopWatcher: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

vi.mock("../src/api", () => ({
    api: {
        DevPanelListDir: (...args: unknown[]) => apiMock.DevPanelListDir(...args),
        DevPanelReadFile: (...args: unknown[]) => apiMock.DevPanelReadFile(...args),
        DevPanelStartWatcher: (...args: unknown[]) => apiMock.DevPanelStartWatcher(...args),
        DevPanelStopWatcher: (...args: unknown[]) => apiMock.DevPanelStopWatcher(...args),
    },
}));

vi.mock("../wailsjs/runtime", () => runtimeMock);

import {createFileTreeStore} from "../src/stores/fileTreeStore";
import {useFileTreeActions} from "../src/hooks/useFileTreeActions";

let mockActiveSession: string | null = "session-a";
let mockActiveSessionKey = "session-a:1";
let probeActions: ReturnType<typeof useFileTreeActions> | null = null;

function makeDirEntry(name: string, hasChildren: boolean = false, path: string = name) {
    return {name, path, is_dir: true, size: 0, has_children: hasChildren};
}

function makeFileEntry(name: string, path: string = name) {
    return {name, path, is_dir: false, size: 100, has_children: false};
}

function Probe() {
    probeActions = useFileTreeActions(probeStore, {
        activeSession: mockActiveSession,
        activeSessionKey: mockActiveSessionKey,
        loadFileContent: false,
    });
    return null;
}

let probeStore = createFileTreeStore();

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("useFileTreeActions", () => {
    let container: HTMLDivElement;
    let root: Root;
    let eventHandlers: Map<string, (payload: unknown) => void>;

    beforeEach(() => {
        probeStore = createFileTreeStore();
        mockActiveSession = "session-a";
        mockActiveSessionKey = "session-a:1";
        probeActions = null;
        eventHandlers = new Map<string, (payload: unknown) => void>();
        runtimeMock.EventsOn.mockReset();
        runtimeMock.EventsOn.mockImplementation((eventName: string, handler: (payload: unknown) => void) => {
            eventHandlers.set(eventName, handler);
            return () => {
                eventHandlers.delete(eventName);
            };
        });
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockResolvedValue([makeDirEntry("src")]);
        apiMock.DevPanelReadFile.mockReset();
        apiMock.DevPanelStartWatcher.mockReset();
        apiMock.DevPanelStartWatcher.mockResolvedValue(undefined);
        apiMock.DevPanelStopWatcher.mockReset();
        apiMock.DevPanelStopWatcher.mockResolvedValue(undefined);
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        vi.spyOn(console, "error").mockImplementation(() => undefined);

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

    it("starts and stops the watcher when the active session changes", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-a");
        expect(probeStore.getState().tree.length).toBe(1);

        mockActiveSession = "session-b";
        apiMock.DevPanelListDir.mockResolvedValueOnce([]);
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a");
        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-b");
    });

    it("keeps the tree empty when the initial root load fails", async () => {
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockRejectedValueOnce(new Error("initial load failed"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(probeStore.getState().tree).toEqual([]);
        expect(probeStore.getState().selectedPath).toBeNull();
        expect(probeStore.getState().fileContent).toBeNull();
        expect(probeStore.getState().error).toBe("initial load failed");
        expect(probeStore.getState().isRootLoading).toBe(false);
    });

    it("keeps the loaded tree visible when watcher startup fails", async () => {
        apiMock.DevPanelStartWatcher.mockRejectedValueOnce(new Error("watcher bootstrap failed"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();
        await flushEffects();

        expect(probeStore.getState().tree.map((node) => node.path)).toEqual(["src"]);
        expect(probeStore.getState().watcherError).toBe(
            "Automatic refresh is unavailable. Reload the directory manually if needed.",
        );
    });

    it("restarts the watcher when the active session is recreated with the same name", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const staleInvalidationHandler = eventHandlers.get("devpanel:tree-invalidated");
        mockActiveSessionKey = "session-a:99";
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a");
        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledTimes(2);

        act(() => {
            staleInvalidationHandler?.({session_name: "session-a", paths: [""]});
        });
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenCalledTimes(2);
    });

    it("rebinds watcher ownership after a session rename handoff", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("tmux:session-renamed")?.({oldName: "session-a", newName: "session-renamed"});
        });
        mockActiveSession = "session-renamed";
        mockActiveSessionKey = "session-renamed:1";

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-renamed");
        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a");
    });

    it("continues refreshing from invalidation events after a session rename handoff", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("tmux:session-renamed")?.({oldName: "session-a", newName: "session-renamed"});
        });
        mockActiveSession = "session-renamed";
        mockActiveSessionKey = "session-renamed:1";

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("devpanel:tree-invalidated")?.({session_name: "session-renamed", paths: [""]});
        });
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenCalledWith("session-renamed", "");
    });

    it("releases the renamed watcher when the file tree unmounts after a rename handoff", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("tmux:session-renamed")?.({oldName: "session-a", newName: "session-renamed"});
        });
        mockActiveSession = "session-renamed";
        mockActiveSessionKey = "session-renamed:1";

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            root.render(<></>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a");
        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-renamed");
        expect(
            apiMock.DevPanelStopWatcher.mock.calls.filter(([sessionName]) => sessionName === "session-renamed"),
        ).toHaveLength(1);
    });

    it("logs every watcher cleanup failure during a rename handoff teardown", async () => {
        apiMock.DevPanelStopWatcher
            .mockRejectedValueOnce(new Error("stop old failed"))
            .mockRejectedValueOnce(new Error("stop renamed failed"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("tmux:session-renamed")?.({oldName: "session-a", newName: "session-renamed"});
        });
        mockActiveSession = "session-renamed";
        mockActiveSessionKey = "session-renamed:1";

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            root.render(<></>);
        });
        await flushEffects();
        await flushEffects();

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a");
        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-renamed");
        expect(console.warn).toHaveBeenCalledWith("[file-tree] DevPanelStopWatcher failed", expect.objectContaining({
            session: "session-a",
        }));
        expect(console.warn).toHaveBeenCalledWith("[file-tree] DevPanelStopWatcher failed", expect.objectContaining({
            session: "session-renamed",
        }));
    });

    it("starts a fresh watcher when the session key changes without a rename event", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mockActiveSession = "session-renamed";
        mockActiveSessionKey = "session-renamed:1";

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a");
        expect(apiMock.DevPanelStopWatcher).not.toHaveBeenCalledWith("session-renamed");
        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-renamed");
    });

    it("ignores other-session and invalid invalidation events", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const invalidationHandler = eventHandlers.get("devpanel:tree-invalidated");
        expect(invalidationHandler).toBeTypeOf("function");
        expect(apiMock.DevPanelListDir).toHaveBeenCalledTimes(1);

        act(() => {
            invalidationHandler?.({session_name: "session-b", paths: [""]});
        });
        await flushEffects();
        expect(apiMock.DevPanelListDir).toHaveBeenCalledTimes(1);

        act(() => {
            invalidationHandler?.({invalid: true});
        });
        await flushEffects();
        expect(apiMock.DevPanelListDir).toHaveBeenCalledTimes(1);

        act(() => {
            invalidationHandler?.({session_name: "session-a", paths: [""]});
        });
        await flushEffects();
        expect(apiMock.DevPanelListDir).toHaveBeenCalledTimes(2);
    });

    it("refreshes collapsed directories when watcher invalidates them", async () => {
        apiMock.DevPanelListDir
            .mockReset()
            .mockResolvedValueOnce([makeDirEntry("src", false)])
            .mockResolvedValueOnce([makeFileEntry("new.ts", "src/new.ts")]);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const invalidationHandler = eventHandlers.get("devpanel:tree-invalidated");
        act(() => {
            invalidationHandler?.({session_name: "session-a", paths: ["src"]});
        });
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenNthCalledWith(2, "session-a", "src");
        const srcNode = probeStore.getState().tree[0];
        expect(srcNode?.path).toBe("src");
        expect(srcNode?.hasChildren).toBe(true);
        expect(srcNode?.children?.[0]?.path).toBe("src/new.ts");
        expect(probeStore.getState().expandedPaths.has("src")).toBe(false);
    });

    it("surfaces automatic refresh failures in watcherError", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        apiMock.DevPanelListDir.mockRejectedValueOnce(new Error("refresh failed"));

        const invalidationHandler = eventHandlers.get("devpanel:tree-invalidated");
        act(() => {
            invalidationHandler?.({session_name: "session-a", paths: [""]});
        });
        await flushEffects();

        expect(probeStore.getState().watcherError).toBe("refresh failed");
        expect(probeStore.getState().dirError).toBeNull();
    });

    it("surfaces watcher failure events in watcherError for the active session", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const watcherFailedHandler = eventHandlers.get("devpanel:watcher-failed");
        act(() => {
            watcherFailedHandler?.({session_name: "session-a", message: "watcher stopped"});
        });

        expect(probeStore.getState().watcherError).toBe("watcher stopped");
        expect(probeStore.getState().dirError).toBeNull();
    });

    it("keeps watcherError visible across unrelated manual actions", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const watcherFailedHandler = eventHandlers.get("devpanel:watcher-failed");
        act(() => {
            watcherFailedHandler?.({session_name: "session-a", message: "watcher stopped"});
        });

        expect(probeStore.getState().watcherError).toBe("watcher stopped");

        await act(async () => {
            probeActions?.loadRoot();
        });
        await flushEffects();
        expect(probeStore.getState().watcherError).toBe("watcher stopped");

        act(() => {
            probeStore.getState().setDirError("temporary directory error");
        });
        await act(async () => {
            probeActions?.selectFile("README.md");
        });
        expect(probeStore.getState().dirError).toBeNull();
        expect(probeStore.getState().watcherError).toBe("watcher stopped");
    });

    it("preserves the current tree and file selection when a root refresh fails", async () => {
        apiMock.DevPanelListDir
            .mockReset()
            .mockResolvedValueOnce([makeFileEntry("README.md")])
            .mockRejectedValueOnce(new Error("refresh failed"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeStore.getState().setSelectedPath("README.md");
            probeStore.getState().setFileContent({
                path: "README.md",
                content: "existing content",
                line_count: 1,
                size: 16,
                truncated: false,
                binary: false,
            });
        });

        await act(async () => {
            probeActions?.loadRoot();
        });
        await flushEffects();

        expect(probeStore.getState().tree.map((node) => node.path)).toEqual(["README.md"]);
        expect(probeStore.getState().selectedPath).toBe("README.md");
        expect(probeStore.getState().fileContent?.path).toBe("README.md");
        expect(probeStore.getState().fileContent?.content).toBe("existing content");
        expect(probeStore.getState().error).toBe("refresh failed");
        expect(probeStore.getState().isRootLoading).toBe(false);
    });

    it("preserves has_children=false for root and child directory loads", async () => {
        apiMock.DevPanelListDir
            .mockReset()
            .mockResolvedValueOnce([makeDirEntry("src", true)])
            .mockResolvedValueOnce([makeDirEntry("empty", false, "src/empty")]);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const rootNode = probeStore.getState().tree[0];
        expect(rootNode?.path).toBe("src");
        expect(rootNode?.hasChildren).toBe(true);

        await act(async () => {
            probeActions?.toggleDir("src");
            await Promise.resolve();
        });
        await flushEffects();

        const childNode = probeStore.getState().tree[0]?.children?.[0];
        expect(childNode?.path).toBe("src/empty");
        expect(childNode?.hasChildren).toBe(false);
    });
});
