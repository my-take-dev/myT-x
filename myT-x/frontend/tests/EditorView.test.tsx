import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const editorState = vi.hoisted(() => ({
    activeSession: "test-session" as string | null,
    activeSessionKey: "test-session:1",
    error: null as string | null,
    flatNodes: [] as readonly { path: string; name: string; isDir: boolean; depth: number; size: number }[],
    isRootLoading: false,
    selectedPath: null as string | null,
}));

const editorFileState = vi.hoisted(() => ({
    currentPath: "src/app.ts" as string | null,
    detectedLanguage: "typescript",
    error: null as string | null,
    fileSize: 128,
    isModified: false,
    loadingState: "loaded" as const,
    readOnly: false,
    truncated: false,
}));

const editorFileActions = vi.hoisted(() => ({
    clearFile: vi.fn(),
    handleChange: vi.fn(),
    handleEditorMount: vi.fn(),
    loadFile: vi.fn(),
    saveFile: vi.fn(),
}));

const editorActions = vi.hoisted(() => ({
    clearSelection: vi.fn(),
    createDirectory: vi.fn(),
    createFile: vi.fn(),
    deleteItem: vi.fn(),
    loadRoot: vi.fn(),
    renameItem: vi.fn(),
    selectFile: vi.fn(),
    toggleDir: vi.fn(),
}));

const viewerActions = vi.hoisted(() => ({
    closeView: vi.fn(),
}));

const fileSearchState = vi.hoisted(() => ({
    query: "",
    results: [] as readonly {path: string}[],
    isSearching: false,
    searchError: null as string | null,
}));

const fileSearchActions = vi.hoisted(() => ({
    setQuery: vi.fn(),
    clearSearch: vi.fn(),
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: { closeView: typeof viewerActions.closeView }) => unknown) =>
        selector({
            closeView: viewerActions.closeView,
        }),
}));

vi.mock("../src/components/viewer/views/editor/useEditor", () => ({
    useEditor: () => ({
        activeSession: editorState.activeSession,
        activeSessionKey: editorState.activeSessionKey,
        error: editorState.error,
        flatNodes: editorState.flatNodes,
        isRootLoading: editorState.isRootLoading,
        selectedPath: editorState.selectedPath,
        clearSelection: editorActions.clearSelection,
        createDirectory: editorActions.createDirectory,
        createFile: editorActions.createFile,
        deleteItem: editorActions.deleteItem,
        loadRoot: editorActions.loadRoot,
        renameItem: editorActions.renameItem,
        selectFile: editorActions.selectFile,
        toggleDir: editorActions.toggleDir,
    }),
}));

vi.mock("../src/components/viewer/views/editor/useEditorFile", () => ({
    useEditorFile: () => ({
        clearFile: editorFileActions.clearFile,
        currentPath: editorFileState.currentPath,
        detectedLanguage: editorFileState.detectedLanguage,
        error: editorFileState.error,
        fileSize: editorFileState.fileSize,
        handleChange: editorFileActions.handleChange,
        handleEditorMount: editorFileActions.handleEditorMount,
        isModified: editorFileState.isModified,
        loadFile: editorFileActions.loadFile,
        loadingState: editorFileState.loadingState,
        readOnly: editorFileState.readOnly,
        saveFile: editorFileActions.saveFile,
        truncated: editorFileState.truncated,
    }),
}));

vi.mock("../src/components/viewer/views/file-tree/useFileSearch", () => ({
    useFileSearch: () => ({
        query: fileSearchState.query,
        setQuery: fileSearchActions.setQuery,
        results: fileSearchState.results,
        isSearching: fileSearchState.isSearching,
        searchError: fileSearchState.searchError,
        clearSearch: fileSearchActions.clearSearch,
    }),
}));

vi.mock("../src/components/viewer/views/editor/EditorFileTree", () => ({
    EditorFileTree: ({
                         onRequestCreateFile,
                         isRefreshing,
                         onRequestDelete,
                         onRequestRename,
                         selectedPath,
                     }: {
        onRequestCreateFile: (parentDir: string) => void;
        isRefreshing: boolean;
        onRequestDelete: (node: { path: string; name: string; isDir: boolean }) => void;
        onRequestRename: (node: { path: string; name: string; isDir: boolean }) => void;
        selectedPath: string | null;
    }) => (
        <div>
            <div
                data-testid="editor-tree"
                data-refreshing={String(isRefreshing)}
                data-selected-path={selectedPath ?? ""}
            />
            <button
                type="button"
                data-testid="request-create-file"
                onClick={() => onRequestCreateFile("")}
            >
                Create File
            </button>
            <button
                type="button"
                data-testid="request-rename-parent"
                onClick={() => onRequestRename({path: "src", name: "src", isDir: true})}
            >
                Rename Parent
            </button>
            <button
                type="button"
                data-testid="request-delete-parent"
                onClick={() => onRequestDelete({path: "src", name: "src", isDir: true})}
            >
                Delete Parent
            </button>
        </div>
    ),
}));

vi.mock("../src/components/viewer/views/editor/EditorPane", () => ({
    EditorPane: () => <div data-testid="editor-pane">pane</div>,
}));

import {EditorView} from "../src/components/viewer/views/editor/EditorView";

function createDeferred<T>() {
    let resolve!: (value: T | PromiseLike<T>) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

describe("EditorView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        editorState.activeSession = "test-session";
        editorState.activeSessionKey = "test-session:1";
        editorState.error = null;
        editorState.flatNodes = [];
        editorState.isRootLoading = false;
        editorState.selectedPath = null;
        editorFileState.currentPath = null;
        editorFileState.isModified = false;
        editorFileActions.clearFile.mockReset();
        editorFileActions.handleChange.mockReset();
        editorFileActions.handleEditorMount.mockReset();
        editorFileActions.loadFile.mockReset();
        editorFileActions.saveFile.mockReset();
        editorActions.createDirectory.mockReset();
        editorActions.clearSelection.mockReset();
        editorActions.createFile.mockReset();
        editorActions.createDirectory.mockResolvedValue({result: true, refreshError: null});
        editorActions.createFile.mockResolvedValue({result: "created.txt", refreshError: null});
        editorActions.deleteItem.mockReset();
        editorActions.deleteItem.mockResolvedValue({result: true, refreshError: null});
        editorActions.loadRoot.mockReset();
        editorActions.renameItem.mockReset();
        editorActions.renameItem.mockResolvedValue({result: "renamed-src", refreshError: null});
        editorActions.selectFile.mockReset();
        editorActions.toggleDir.mockReset();
        viewerActions.closeView.mockReset();
        fileSearchState.query = "";
        fileSearchState.results = [];
        fileSearchState.isSearching = false;
        fileSearchState.searchError = null;
        fileSearchActions.setQuery.mockReset();
        fileSearchActions.clearSearch.mockReset();
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

    it("shows the blocking loading state only before the first root payload arrives", () => {
        editorState.isRootLoading = true;
        editorState.flatNodes = [];

        act(() => {
            root.render(<EditorView/>);
        });

        expect(container.textContent).toContain("Loading editor tree...");
        expect(container.querySelector("[data-testid='editor-tree']")).toBeNull();
        expect(container.querySelector("[data-testid='editor-pane']")).toBeNull();
    });

    it("keeps the tree and editor pane mounted during background refresh", () => {
        editorState.isRootLoading = true;
        editorState.selectedPath = "src/app.ts";
        editorState.flatNodes = [{
            path: "src/app.ts",
            name: "app.ts",
            isDir: false,
            depth: 1,
            size: 128,
        }];

        act(() => {
            root.render(<EditorView/>);
        });

        expect(container.textContent).not.toContain("Loading editor tree...");
        const tree = container.querySelector("[data-testid='editor-tree']");
        expect(tree).not.toBeNull();
        expect(tree?.getAttribute("data-refreshing")).toBe("true");
        expect(tree?.getAttribute("data-selected-path")).toBe("src/app.ts");
        expect(container.querySelector("[data-testid='editor-pane']")).not.toBeNull();
    });

    it("does not select a stale file created before a session switch", async () => {
        const pendingCreate = createDeferred<{result: string | null; refreshError: string | null}>();
        editorActions.createFile.mockReturnValueOnce(pendingCreate.promise);

        act(() => {
            root.render(<EditorView/>);
        });

        const createRequest = container.querySelector("[data-testid='request-create-file']") as HTMLButtonElement;
        await act(async () => {
            createRequest.click();
            await Promise.resolve();
        });

        const input = container.querySelector("input") as HTMLInputElement;
        await act(async () => {
            input.value = "created.txt";
            input.dispatchEvent(new Event("input", {bubbles: true}));
            await Promise.resolve();
        });

        const createButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Create");
        await act(async () => {
            createButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        editorState.activeSession = "next-session";
        editorState.activeSessionKey = "next-session:2";
        act(() => {
            root.render(<EditorView/>);
        });

        await act(async () => {
            pendingCreate.resolve({result: null, refreshError: null});
            await Promise.resolve();
        });

        expect(editorActions.selectFile).not.toHaveBeenCalled();
        expect(container.querySelector("[role='dialog'][aria-label='Create file']")).toBeNull();
    });

    it("does not reset editor session state when only activeSession changes", () => {
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";

        act(() => {
            root.render(<EditorView/>);
        });

        fileSearchActions.clearSearch.mockClear();
        editorFileActions.clearFile.mockClear();

        editorState.activeSession = "display-name-only-change";
        act(() => {
            root.render(<EditorView/>);
        });

        expect(fileSearchActions.clearSearch).not.toHaveBeenCalled();
        expect(editorFileActions.clearFile).not.toHaveBeenCalled();
    });

    it("accepts file names that contain repeated dots within a single path segment", async () => {
        act(() => {
            root.render(<EditorView/>);
        });

        const createRequest = container.querySelector("[data-testid='request-create-file']") as HTMLButtonElement;
        await act(async () => {
            createRequest.click();
            await Promise.resolve();
        });

        const input = container.querySelector("input") as HTMLInputElement;
        await act(async () => {
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
            nativeInputValueSetter?.call(input, "app..backup.ts");
            input.dispatchEvent(new Event("change", {bubbles: true}));
            await Promise.resolve();
        });

        const createButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Create");
        await act(async () => {
            createButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorActions.createFile).toHaveBeenCalledWith("", "app..backup.ts");
        expect(container.textContent).not.toContain("Name cannot be");
    });

    it("requires discard confirmation before renaming a parent directory of the open file", async () => {
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;
        editorActions.renameItem.mockResolvedValue({result: "renamed-src", refreshError: null});

        act(() => {
            root.render(<EditorView/>);
        });
        editorFileActions.clearFile.mockClear();

        const renameRequest = container.querySelector("[data-testid='request-rename-parent']") as HTMLButtonElement;
        await act(async () => {
            renameRequest.click();
            await Promise.resolve();
        });

        const renameButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Rename");
        expect(renameButton).toBeDefined();
        await act(async () => {
            renameButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(container.textContent).toContain("Discard unsaved changes?");
        expect(editorActions.renameItem).not.toHaveBeenCalled();

        const discardButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Discard");
        await act(async () => {
            discardButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorActions.renameItem).toHaveBeenCalledWith("src", "src");
        expect(editorFileActions.clearFile).toHaveBeenCalledTimes(1);
        expect(editorActions.selectFile).toHaveBeenCalledWith("renamed-src/app.ts");
    });

    it("keeps the rename dialog open when Escape cancels the discard prompt", async () => {
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;

        act(() => {
            root.render(<EditorView/>);
        });

        const renameRequest = container.querySelector("[data-testid='request-rename-parent']") as HTMLButtonElement;
        await act(async () => {
            renameRequest.click();
            await Promise.resolve();
        });

        const renameButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Rename");
        await act(async () => {
            renameButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(container.querySelector("[role='dialog'][aria-label='Rename item']")).toBeNull();
        expect(container.querySelector("[role='dialog'][aria-label='Discard unsaved changes']")).not.toBeNull();

        await act(async () => {
            document.dispatchEvent(new KeyboardEvent("keydown", {bubbles: true, key: "Escape"}));
            await Promise.resolve();
        });

        expect(container.querySelector("[role='dialog'][aria-label='Discard unsaved changes']")).toBeNull();
        expect(container.querySelector("[role='dialog'][aria-label='Rename item']")).not.toBeNull();
        expect(editorActions.renameItem).not.toHaveBeenCalled();
    });

    it("does not apply a stale rename after a session switch", async () => {
        const pendingRename = createDeferred<{result: string | null; refreshError: string | null}>();
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = false;
        editorActions.renameItem.mockReturnValueOnce(pendingRename.promise);

        act(() => {
            root.render(<EditorView/>);
        });

        const renameRequest = container.querySelector("[data-testid='request-rename-parent']") as HTMLButtonElement;
        await act(async () => {
            renameRequest.click();
            await Promise.resolve();
        });

        const renameButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Rename");
        await act(async () => {
            renameButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        editorState.activeSession = "next-session";
        editorState.activeSessionKey = "next-session:2";
        editorState.selectedPath = null;
        act(() => {
            root.render(<EditorView/>);
        });

        await act(async () => {
            pendingRename.resolve({result: null, refreshError: null});
            await Promise.resolve();
        });

        expect(editorFileActions.clearFile).toHaveBeenCalledTimes(1);
        expect(editorActions.selectFile).not.toHaveBeenCalled();
        expect(container.querySelector("[role='dialog'][aria-label='Rename item']")).toBeNull();
    });

    it("closes the create dialog and shows a degraded refresh warning after create succeeds", async () => {
        editorActions.createFile.mockResolvedValue({
            result: "created.txt",
            refreshError: "Created the item, but refresh failed",
        });

        act(() => {
            root.render(<EditorView/>);
        });

        const createRequest = container.querySelector("[data-testid='request-create-file']") as HTMLButtonElement;
        await act(async () => {
            createRequest.click();
            await Promise.resolve();
        });

        const input = container.querySelector("input") as HTMLInputElement;
        await act(async () => {
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
            nativeInputValueSetter?.call(input, "created.txt");
            input.dispatchEvent(new Event("change", {bubbles: true}));
            await Promise.resolve();
        });

        const createButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Create");
        await act(async () => {
            createButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorActions.selectFile).toHaveBeenCalledWith("created.txt");
        expect(container.querySelector("[role='dialog'][aria-label='Create file']")).toBeNull();
        expect(container.textContent).toContain("Created the item, but refresh failed");
    });

    it("shows the rename-specific degraded refresh warning after rename succeeds", async () => {
        editorState.selectedPath = "src";
        editorActions.renameItem.mockResolvedValue({
            result: "new.txt",
            refreshError: "Renamed the item, but refresh failed",
        });

        act(() => {
            root.render(<EditorView/>);
        });

        const renameRequest = container.querySelector("[data-testid='request-rename-parent']") as HTMLButtonElement;
        await act(async () => {
            renameRequest.click();
            await Promise.resolve();
        });

        const input = container.querySelector("input") as HTMLInputElement;
        await act(async () => {
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
            nativeInputValueSetter?.call(input, "new.txt");
            input.dispatchEvent(new Event("change", {bubbles: true}));
            await Promise.resolve();
        });

        const renameButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Rename");
        await act(async () => {
            renameButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(container.querySelector("[role='dialog'][aria-label='Rename item']")).toBeNull();
        expect(container.textContent).toContain("Renamed the item, but refresh failed");
    });

    it("requires discard confirmation before deleting a parent directory of the open file", async () => {
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;
        editorActions.deleteItem.mockResolvedValue({result: true, refreshError: null});

        act(() => {
            root.render(<EditorView/>);
        });
        editorFileActions.clearFile.mockClear();

        const deleteRequest = container.querySelector("[data-testid='request-delete-parent']") as HTMLButtonElement;
        await act(async () => {
            deleteRequest.click();
            await Promise.resolve();
        });

        const deleteButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Delete");
        expect(deleteButton).toBeDefined();
        await act(async () => {
            deleteButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(container.textContent).toContain("Discard unsaved changes?");
        expect(editorActions.deleteItem).not.toHaveBeenCalled();

        const discardButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Discard");
        await act(async () => {
            discardButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorActions.deleteItem).toHaveBeenCalledWith("src");
        expect(editorFileActions.clearFile).toHaveBeenCalledTimes(1);
    });

    it("requires discard confirmation before closing the panel with unsaved changes", async () => {
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;

        act(() => {
            root.render(<EditorView/>);
        });

        const closeButton = container.querySelector("button[aria-label='Close']") as HTMLButtonElement;
        act(() => {
            closeButton.click();
        });

        expect(container.textContent).toContain("Discard unsaved changes?");
        expect(viewerActions.closeView).not.toHaveBeenCalled();

        const discardButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Discard");
        act(() => {
            discardButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(viewerActions.closeView).toHaveBeenCalledTimes(1);
    });

    it("does not clear the editor for a stale delete after a session switch", async () => {
        const pendingDelete = createDeferred<boolean>();
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = false;
        editorActions.deleteItem.mockReturnValueOnce(pendingDelete.promise);

        act(() => {
            root.render(<EditorView/>);
        });

        const deleteRequest = container.querySelector("[data-testid='request-delete-parent']") as HTMLButtonElement;
        await act(async () => {
            deleteRequest.click();
            await Promise.resolve();
        });

        const deleteButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Delete");
        await act(async () => {
            deleteButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        editorState.activeSession = "next-session";
        editorState.activeSessionKey = "next-session:2";
        editorState.selectedPath = null;
        act(() => {
            root.render(<EditorView/>);
        });

        await act(async () => {
            pendingDelete.resolve(false);
            await Promise.resolve();
        });

        expect(editorFileActions.clearFile).toHaveBeenCalledTimes(1);
        expect(container.querySelector("[role='dialog'][aria-label='Delete item']")).toBeNull();
    });

    it("requires discard confirmation before clearing an externally invalidated open file", async () => {
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;

        act(() => {
            root.render(<EditorView/>);
        });
        editorFileActions.clearFile.mockClear();

        editorState.selectedPath = null;
        act(() => {
            root.render(<EditorView/>);
        });

        expect(container.textContent).toContain("Discard unsaved changes?");
        expect(editorFileActions.clearFile).not.toHaveBeenCalled();

        const cancelButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Cancel");
        await act(async () => {
            cancelButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });
        expect(editorFileActions.clearFile).not.toHaveBeenCalled();

        act(() => {
            editorState.selectedPath = "src/app.ts";
            root.render(<EditorView/>);
        });
        expect(container.textContent).not.toContain("Discard unsaved changes?");

        act(() => {
            editorState.selectedPath = null;
            root.render(<EditorView/>);
        });

        const discardButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Discard");
        await act(async () => {
            discardButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorFileActions.clearFile).toHaveBeenCalledTimes(1);
    });

    it("requires discard confirmation before loading an externally selected file", async () => {
        editorState.selectedPath = "src/other.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;

        act(() => {
            root.render(<EditorView/>);
        });

        expect(container.textContent).toContain("Discard unsaved changes?");
        expect(editorFileActions.loadFile).not.toHaveBeenCalled();
        expect(editorActions.selectFile).toHaveBeenCalledWith("src/app.ts");
        expect(editorActions.clearSelection).not.toHaveBeenCalled();

        const discardButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Discard");
        await act(async () => {
            discardButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorActions.selectFile).toHaveBeenCalledWith("src/other.ts");
        expect(editorFileActions.loadFile).not.toHaveBeenCalled();
    });

    it("confirms before clearing unsaved edits when the active session changes", async () => {
        editorState.activeSession = "test-session";
        editorState.activeSessionKey = "test-session:1";
        editorState.selectedPath = "src/app.ts";
        editorFileState.currentPath = "src/app.ts";
        editorFileState.isModified = true;

        act(() => {
            root.render(<EditorView/>);
        });
        editorFileActions.clearFile.mockClear();

        editorState.activeSession = "next-session";
        editorState.activeSessionKey = "next-session:2";
        editorState.selectedPath = null;
        act(() => {
            root.render(<EditorView/>);
        });

        expect(editorFileActions.clearFile).not.toHaveBeenCalled();
        expect(container.querySelector("[role='dialog'][aria-label='Discard unsaved changes']")).not.toBeNull();

        const discardButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Discard");
        await act(async () => {
            discardButton!.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(editorFileActions.clearFile).toHaveBeenCalledTimes(1);
    });
});
