import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useEditor, type UseEditorResult} from "../src/components/viewer/views/editor/useEditor";
import type {FileNode} from "../src/components/viewer/views/file-tree/fileTreeTypes";
import type {FileTreeStore} from "../src/stores/fileTreeStore";

const apiMock = vi.hoisted(() => ({
    DevPanelCreateDirectory: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelCreateFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelDeleteFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelRenameFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

const fileTreeActionsMock = vi.hoisted(() => ({
    loadRoot: vi.fn(),
    refreshDirectory: vi.fn<(...args: unknown[]) => Promise<void>>(),
    selectFile: vi.fn(),
    toggleDir: vi.fn(),
}));

let mockActiveSession: string | null = "session-a";
let mockSessions: {id: number; name: string; windows: never[]}[] | undefined = [
    {
        id: 1,
        name: "session-a",
        windows: [],
    },
];
let latestStore: FileTreeStore | null = null;
let latestResult: UseEditorResult | null = null;

vi.mock("../src/api", () => ({
    api: {
        DevPanelCreateDirectory: (...args: unknown[]) => apiMock.DevPanelCreateDirectory(...args),
        DevPanelCreateFile: (...args: unknown[]) => apiMock.DevPanelCreateFile(...args),
        DevPanelDeleteFile: (...args: unknown[]) => apiMock.DevPanelDeleteFile(...args),
        DevPanelRenameFile: (...args: unknown[]) => apiMock.DevPanelRenameFile(...args),
    },
}));

vi.mock("../src/hooks/useFileTreeActions", () => ({
    useFileTreeActions: (store: FileTreeStore) => {
        latestStore = store;
        return fileTreeActionsMock;
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {activeSession: string | null; sessions?: typeof mockSessions}) => unknown) =>
        selector({activeSession: mockActiveSession, sessions: mockSessions}),
}));

function createDeferred<T>() {
    let resolve!: (value: T | PromiseLike<T>) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

function Probe() {
    latestResult = useEditor();
    return <div data-testid="node-count">{String(latestResult.flatNodes.length)}</div>;
}

function seedTree(nodes: readonly FileNode[]) {
    act(() => {
        latestStore?.getState().setRootNodes(nodes);
    });
}

describe("useEditor", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        latestStore = null;
        latestResult = null;
        mockActiveSession = "session-a";
        mockSessions = [{id: 1, name: "session-a", windows: []}];
        apiMock.DevPanelCreateDirectory.mockReset();
        apiMock.DevPanelCreateDirectory.mockResolvedValue(undefined);
        apiMock.DevPanelCreateFile.mockReset();
        apiMock.DevPanelCreateFile.mockResolvedValue(undefined);
        apiMock.DevPanelDeleteFile.mockReset();
        apiMock.DevPanelDeleteFile.mockResolvedValue(undefined);
        apiMock.DevPanelRenameFile.mockReset();
        apiMock.DevPanelRenameFile.mockResolvedValue(undefined);
        fileTreeActionsMock.loadRoot.mockReset();
        fileTreeActionsMock.refreshDirectory.mockReset();
        fileTreeActionsMock.selectFile.mockReset();
        fileTreeActionsMock.toggleDir.mockReset();
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;

        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        act(() => {
            root.render(<Probe/>);
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("returns a degraded create-file result when the tree refresh fails", async () => {
        fileTreeActionsMock.refreshDirectory.mockRejectedValueOnce(new Error("refresh failed"));
        expect(latestResult).not.toBeNull();

        let result: Awaited<ReturnType<UseEditorResult["createFile"]>> | null = null;
        await act(async () => {
            result = await latestResult!.createFile("", "new.txt");
        });

        expect(apiMock.DevPanelCreateFile).toHaveBeenCalledWith("session-a:1", "new.txt");
        expect(fileTreeActionsMock.refreshDirectory).toHaveBeenCalledWith("", {expandOnSuccess: false});
        expect(result).toEqual({
            result: "new.txt",
            refreshError: "Created the item. refresh failed",
        });
    });

    it("returns a degraded create-directory result when the tree refresh fails", async () => {
        fileTreeActionsMock.refreshDirectory.mockRejectedValueOnce(new Error("refresh failed"));
        expect(latestResult).not.toBeNull();

        let result: Awaited<ReturnType<UseEditorResult["createDirectory"]>> | null = null;
        await act(async () => {
            result = await latestResult!.createDirectory("src", "nested");
        });

        expect(apiMock.DevPanelCreateDirectory).toHaveBeenCalledWith("session-a:1", "src/nested");
        expect(fileTreeActionsMock.refreshDirectory).toHaveBeenCalledWith("src", {expandOnSuccess: true});
        expect(result).toEqual({
            result: true,
            refreshError: "Created the item. refresh failed",
        });
    });

    it("returns a degraded rename result when the follow-up refresh fails", async () => {
        seedTree([{name: "old.txt", path: "old.txt", isDir: false, hasChildren: false, size: 12}]);
        fileTreeActionsMock.refreshDirectory.mockRejectedValueOnce(new Error("refresh failed"));
        expect(latestResult).not.toBeNull();

        let result: Awaited<ReturnType<UseEditorResult["renameItem"]>> | null = null;
        await act(async () => {
            result = await latestResult!.renameItem("old.txt", "new.txt");
        });

        expect(apiMock.DevPanelRenameFile).toHaveBeenCalledWith("session-a:1", "old.txt", "new.txt");
        expect(fileTreeActionsMock.refreshDirectory).toHaveBeenCalledWith("", {expandOnSuccess: false});
        expect(latestStore?.getState().tree).toEqual([
            {name: "old.txt", path: "old.txt", isDir: false, hasChildren: false, size: 12},
        ]);
        expect(result).toEqual({
            result: "new.txt",
            refreshError: "Renamed the item. refresh failed",
        });
    });

    it("returns a degraded delete result when the follow-up refresh fails", async () => {
        seedTree([{name: "old.txt", path: "old.txt", isDir: false, hasChildren: false, size: 12}]);
        fileTreeActionsMock.refreshDirectory.mockRejectedValueOnce(new Error("refresh failed"));
        expect(latestResult).not.toBeNull();

        let result: Awaited<ReturnType<UseEditorResult["deleteItem"]>> | null = null;
        await act(async () => {
            result = await latestResult!.deleteItem("old.txt");
        });

        expect(apiMock.DevPanelDeleteFile).toHaveBeenCalledWith("session-a:1", "old.txt");
        expect(fileTreeActionsMock.refreshDirectory).toHaveBeenCalledWith("", {expandOnSuccess: false});
        expect(latestStore?.getState().tree).toEqual([
            {name: "old.txt", path: "old.txt", isDir: false, hasChildren: false, size: 12},
        ]);
        expect(result).toEqual({
            result: true,
            refreshError: "Deleted the item. refresh failed",
        });
    });

    it("treats a missing sessions array as an empty snapshot", () => {
        mockSessions = undefined;

        act(() => {
            root.render(<Probe/>);
        });

        expect(latestResult?.activeSession).toBe("session-a");
        expect(latestResult?.activeSessionKey).toBe("");
        expect(latestResult?.flatNodes).toEqual([]);
    });

    it("ignores stale create results after a session switch", async () => {
        const pendingCreate = createDeferred<void>();
        apiMock.DevPanelCreateFile.mockReturnValueOnce(pendingCreate.promise);
        expect(latestResult).not.toBeNull();

        const createPromise = latestResult!.createFile("", "new.txt");

        mockActiveSession = "session-b";
        mockSessions = [{id: 2, name: "session-b", windows: []}];
        act(() => {
            root.render(<Probe/>);
        });

        let createdPath: Awaited<ReturnType<UseEditorResult["createFile"]>> | null = {result: "unexpected", refreshError: null};
        await act(async () => {
            pendingCreate.resolve(undefined);
            createdPath = await createPromise;
        });

        expect(createdPath).toEqual({result: null, refreshError: null});
        expect(fileTreeActionsMock.refreshDirectory).not.toHaveBeenCalled();
    });

    it("ignores stale rename results after a session switch", async () => {
        const pendingRename = createDeferred<void>();
        apiMock.DevPanelRenameFile.mockReturnValueOnce(pendingRename.promise);
        expect(latestResult).not.toBeNull();

        const renamePromise = latestResult!.renameItem("old.txt", "new.txt");

        mockActiveSession = "session-b";
        mockSessions = [{id: 2, name: "session-b", windows: []}];
        act(() => {
            root.render(<Probe/>);
        });

        let renamedPath: Awaited<ReturnType<UseEditorResult["renameItem"]>> | null = {result: "unexpected", refreshError: null};
        await act(async () => {
            pendingRename.resolve(undefined);
            renamedPath = await renamePromise;
        });

        expect(renamedPath).toEqual({result: null, refreshError: null});
        expect(fileTreeActionsMock.refreshDirectory).not.toHaveBeenCalled();
    });

    it("ignores stale delete results after a session switch", async () => {
        const pendingDelete = createDeferred<void>();
        apiMock.DevPanelDeleteFile.mockReturnValueOnce(pendingDelete.promise);
        expect(latestResult).not.toBeNull();

        const deletePromise = latestResult!.deleteItem("old.txt");

        mockActiveSession = "session-b";
        mockSessions = [{id: 2, name: "session-b", windows: []}];
        act(() => {
            root.render(<Probe/>);
        });

        let deleted: Awaited<ReturnType<UseEditorResult["deleteItem"]>> | null = {result: true, refreshError: null};
        await act(async () => {
            pendingDelete.resolve(undefined);
            deleted = await deletePromise;
        });

        expect(deleted).toEqual({result: false, refreshError: null});
        expect(fileTreeActionsMock.refreshDirectory).not.toHaveBeenCalled();
    });

    it("keeps clearSelection stable across rerenders", () => {
        expect(latestResult).not.toBeNull();
        const initialClearSelection = latestResult!.clearSelection;

        act(() => {
            root.render(<Probe/>);
        });

        expect(latestResult!.clearSelection).toBe(initialClearSelection);
    });
});
