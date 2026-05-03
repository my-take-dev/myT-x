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
let probeLoadFileContent = false;
let probeAutoRefreshExternalChanges: boolean | undefined;

function makeDirEntry(name: string, hasChildren: boolean = false, path: string = name) {
    return {name, path, is_dir: true, size: 0, has_children: hasChildren, has_view_target: true};
}

function makeFileEntry(name: string, path: string = name) {
    return {name, path, is_dir: false, size: 100, has_children: false, has_view_target: true};
}

function makeFileContent(path: string, content: string) {
    return {
        path,
        content,
        line_count: 1,
        size: content.length,
        truncated: false,
        binary: false,
    };
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

function Probe() {
    probeActions = useFileTreeActions(probeStore, {
        activeSession: mockActiveSession,
        activeSessionKey: mockActiveSessionKey,
        loadFileContent: probeLoadFileContent,
        autoRefreshExternalChanges: probeAutoRefreshExternalChanges,
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
        probeLoadFileContent = false;
        probeAutoRefreshExternalChanges = undefined;
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

        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-a:1");
        expect(probeStore.getState().tree.length).toBe(1);

        mockActiveSession = "session-b";
        mockActiveSessionKey = "session-b:1";
        apiMock.DevPanelListDir.mockResolvedValueOnce([]);
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a:1");
        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-b:1");
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

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a:1");
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

        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-renamed:1");
        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a:1");
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

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a:1");
        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-renamed:1");
        expect(
            apiMock.DevPanelStopWatcher.mock.calls.filter(([sessionKey]) => sessionKey === "session-renamed:1"),
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

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a:1");
        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-renamed:1");
        expect(console.warn).toHaveBeenCalledWith("[file-tree] DevPanelStopWatcher failed", expect.objectContaining({
            sessionKey: "session-a:1",
        }));
        expect(console.warn).toHaveBeenCalledWith("[file-tree] DevPanelStopWatcher failed", expect.objectContaining({
            sessionKey: "session-renamed:1",
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

        expect(apiMock.DevPanelStopWatcher).toHaveBeenCalledWith("session-a:1");
        expect(apiMock.DevPanelStopWatcher).not.toHaveBeenCalledWith("session-renamed:1");
        expect(apiMock.DevPanelStartWatcher).toHaveBeenCalledWith("session-renamed:1");
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

    it("refreshes root and known ancestors from watcher invalidation events", async () => {
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockImplementation((_, dirPath) => {
            if (dirPath === "src") {
                return Promise.resolve([makeFileEntry("guide.md", "src/guide.md")]);
            }
            return Promise.resolve([makeDirEntry("src", true)]);
        });

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const invalidationHandler = eventHandlers.get("devpanel:tree-invalidated");
        act(() => {
            invalidationHandler?.({session_name: "session-a", paths: ["src/guide.md", "src", ""]});
        });
        await flushEffects();
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenCalledWith("session-a", "");
        expect(apiMock.DevPanelListDir).toHaveBeenCalledWith("session-a", "src");
        expect(probeStore.getState().tree[0]?.children?.[0]?.path).toBe("src/guide.md");
    });

    it("rediscovers a previously hidden directory when watcher invalidates root", async () => {
        apiMock.DevPanelListDir
            .mockReset()
            .mockResolvedValueOnce([makeDirEntry("src", false)])
            .mockResolvedValueOnce([makeDirEntry("src", false), makeDirEntry("docs", true)]);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const invalidationHandler = eventHandlers.get("devpanel:tree-invalidated");
        act(() => {
            invalidationHandler?.({session_name: "session-a", paths: ["docs/guide.md", "docs", ""]});
        });
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenNthCalledWith(2, "session-a", "");
        expect(probeStore.getState().tree.map((node) => node.path)).toEqual(["src", "docs"]);
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

    it("does not start watcher or subscribe to invalidation events when external auto refresh is disabled", async () => {
        probeAutoRefreshExternalChanges = false;

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenCalledWith("session-a", "");
        expect(apiMock.DevPanelStartWatcher).not.toHaveBeenCalled();
        expect(runtimeMock.EventsOn).not.toHaveBeenCalled();
    });

    it("manually refreshes root, expanded directories, and selected file content", async () => {
        probeLoadFileContent = true;
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockImplementation((_, dirPath) => {
            if (dirPath === "src") {
                return Promise.resolve([makeFileEntry("index.ts", "src/index.ts")]);
            }
            return Promise.resolve([makeDirEntry("src", true)]);
        });
        apiMock.DevPanelReadFile.mockReset();
        apiMock.DevPanelReadFile.mockResolvedValue({
            path: "src/index.ts",
            content: "fresh content",
            line_count: 1,
            size: 13,
            truncated: false,
            binary: false,
        });

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeActions?.toggleDir("src");
        });
        await flushEffects();

        act(() => {
            probeActions?.selectFile("src/index.ts");
        });
        await flushEffects();

        apiMock.DevPanelListDir.mockClear();
        apiMock.DevPanelReadFile.mockClear();

        act(() => {
            probeActions?.loadRoot();
        });
        await flushEffects();
        await flushEffects();
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenCalledWith("session-a", "");
        expect(apiMock.DevPanelListDir).toHaveBeenCalledWith("session-a", "src");
        expect(apiMock.DevPanelReadFile).toHaveBeenCalledWith("session-a", "src/index.ts");
        expect(probeStore.getState().fileContent?.content).toBe("fresh content");
    });

    it("re-fetches expanded directories by depth during manual root refresh", async () => {
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockImplementation((_, dirPath) => {
            if (dirPath === "src/docs") {
                return Promise.resolve([makeFileEntry("guide.md", "src/docs/guide.md")]);
            }
            if (dirPath === "src") {
                return Promise.resolve([makeDirEntry("docs", true, "src/docs")]);
            }
            return Promise.resolve([makeDirEntry("src", true)]);
        });

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeActions?.toggleDir("src");
        });
        await flushEffects();

        act(() => {
            probeActions?.toggleDir("src/docs");
        });
        await flushEffects();

        apiMock.DevPanelListDir.mockClear();

        act(() => {
            probeActions?.loadRoot();
        });
        await flushEffects();
        await flushEffects();
        await flushEffects();

        expect(apiMock.DevPanelListDir.mock.calls.map(([, dirPath]) => dirPath)).toEqual(["", "src", "src/docs"]);
    });

    it("does not reload selected content during manual root refresh when file content loading is disabled", async () => {
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockResolvedValue([makeFileEntry("README.md")]);
        apiMock.DevPanelReadFile.mockReset();

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeActions?.selectFile("README.md");
            probeActions?.loadRoot();
        });
        await flushEffects();
        await flushEffects();

        expect(apiMock.DevPanelReadFile).not.toHaveBeenCalled();
    });

    it("does not let manual root refresh reload a stale selected file after the user selects another file", async () => {
        probeLoadFileContent = true;
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockResolvedValueOnce([makeFileEntry("a.md"), makeFileEntry("b.md")]);
        apiMock.DevPanelReadFile.mockReset();
        apiMock.DevPanelReadFile.mockResolvedValueOnce(makeFileContent("a.md", "old a"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeActions?.selectFile("a.md");
        });
        await flushEffects();
        expect(probeStore.getState().fileContent?.content).toBe("old a");

        const rootRefresh = createDeferred<ReturnType<typeof makeFileEntry>[]>();
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockReturnValueOnce(rootRefresh.promise);
        apiMock.DevPanelReadFile.mockReset();
        apiMock.DevPanelReadFile.mockResolvedValueOnce(makeFileContent("b.md", "new b"));

        act(() => {
            probeActions?.loadRoot();
        });

        act(() => {
            rootRefresh.resolve([makeFileEntry("a.md"), makeFileEntry("b.md")]);
            probeActions?.selectFile("b.md");
        });
        await flushEffects();
        await flushEffects();

        expect(apiMock.DevPanelReadFile).toHaveBeenCalledTimes(1);
        expect(apiMock.DevPanelReadFile).toHaveBeenCalledWith("session-a", "b.md");
        expect(probeStore.getState().selectedPath).toBe("b.md");
        expect(probeStore.getState().fileContent?.content).toBe("new b");
    });

    it("keeps selected file content visible when an expanded directory refresh fails during manual root refresh", async () => {
        probeLoadFileContent = true;
        let srcLoadCount = 0;
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelListDir.mockImplementation((_, dirPath) => {
            if (dirPath === "src") {
                srcLoadCount += 1;
                if (srcLoadCount === 1) {
                    return Promise.resolve([makeFileEntry("index.ts", "src/index.ts")]);
                }
                return Promise.reject(new Error("directory refresh failed"));
            }
            return Promise.resolve([makeDirEntry("src", true), makeFileEntry("README.md")]);
        });
        apiMock.DevPanelReadFile.mockReset();
        apiMock.DevPanelReadFile.mockResolvedValue(makeFileContent("README.md", "fresh content"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeActions?.toggleDir("src");
        });
        await flushEffects();

        act(() => {
            probeActions?.selectFile("README.md");
        });
        await flushEffects();

        act(() => {
            probeActions?.loadRoot();
        });
        await flushEffects();
        await flushEffects();

        expect(probeStore.getState().dirError).toBeNull();
        expect(probeStore.getState().contentError).toBeNull();
        expect(probeStore.getState().fileContent?.content).toBe("fresh content");
    });

    it("reloads a cached collapsed directory after manual root refresh", async () => {
        apiMock.DevPanelListDir
            .mockReset()
            .mockResolvedValueOnce([makeDirEntry("src", true)])
            .mockResolvedValueOnce([makeFileEntry("old.ts", "src/old.ts")])
            .mockResolvedValueOnce([makeDirEntry("src", true)])
            .mockResolvedValueOnce([makeFileEntry("new.ts", "src/new.ts")]);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeActions?.toggleDir("src");
        });
        await flushEffects();

        expect(probeStore.getState().tree[0]?.children?.[0]?.path).toBe("src/old.ts");

        act(() => {
            probeActions?.toggleDir("src");
        });
        expect(probeStore.getState().expandedPaths.has("src")).toBe(false);

        act(() => {
            probeActions?.loadRoot();
        });
        await flushEffects();
        await flushEffects();

        expect(probeStore.getState().tree[0]?.children).toBeUndefined();

        act(() => {
            probeActions?.toggleDir("src");
        });
        await flushEffects();

        expect(apiMock.DevPanelListDir).toHaveBeenNthCalledWith(4, "session-a", "src");
        expect(probeStore.getState().tree[0]?.children?.[0]?.path).toBe("src/new.ts");
    });

    it("keeps a search-selected descendant when its ancestors are not loaded during root refresh", async () => {
        probeLoadFileContent = true;
        apiMock.DevPanelListDir
            .mockReset()
            .mockResolvedValueOnce([makeDirEntry("docs", true)])
            .mockResolvedValueOnce([makeDirEntry("docs", true)]);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            probeStore.getState().setSelectedPath("docs/specs/deep.md");
            probeStore.getState().setFileContent({
                path: "docs/specs/deep.md",
                content: "existing content",
                line_count: 1,
                size: 16,
                truncated: false,
                binary: false,
            });
        });

        act(() => {
            probeActions?.loadRoot();
        });
        await flushEffects();
        await flushEffects();

        expect(probeStore.getState().selectedPath).toBe("docs/specs/deep.md");
        expect(probeStore.getState().fileContent?.content).toBe("existing content");
    });
});
