import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const editorState = vi.hoisted(() => ({
    activeSession: "test-session" as string | null,
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

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: { closeView: () => void }) => unknown) =>
        selector({
            closeView: () => {
            }
        }),
}));

vi.mock("../src/components/viewer/views/editor/useEditor", () => ({
    useEditor: () => ({
        activeSession: editorState.activeSession,
        error: editorState.error,
        flatNodes: editorState.flatNodes,
        isRootLoading: editorState.isRootLoading,
        selectedPath: editorState.selectedPath,
        clearSelection: () => {
        },
        createDirectory: vi.fn(),
        createFile: vi.fn(),
        deleteItem: vi.fn(),
        loadRoot: vi.fn(),
        renameItem: vi.fn(),
        selectFile: vi.fn(),
        toggleDir: vi.fn(),
    }),
}));

vi.mock("../src/components/viewer/views/editor/useEditorFile", () => ({
    useEditorFile: () => ({
        clearFile: vi.fn(),
        currentPath: editorFileState.currentPath,
        detectedLanguage: editorFileState.detectedLanguage,
        error: editorFileState.error,
        fileSize: editorFileState.fileSize,
        handleChange: vi.fn(),
        handleEditorMount: vi.fn(),
        isModified: editorFileState.isModified,
        loadFile: vi.fn(),
        loadingState: editorFileState.loadingState,
        readOnly: editorFileState.readOnly,
        saveFile: vi.fn(),
        truncated: editorFileState.truncated,
    }),
}));

vi.mock("../src/components/viewer/views/editor/EditorFileTree", () => ({
    EditorFileTree: ({
                         isRefreshing,
                         selectedPath,
                     }: {
        isRefreshing: boolean;
        selectedPath: string | null;
    }) => (
        <div
            data-testid="editor-tree"
            data-refreshing={String(isRefreshing)}
            data-selected-path={selectedPath ?? ""}
        />
    ),
}));

vi.mock("../src/components/viewer/views/editor/EditorPane", () => ({
    EditorPane: () => <div data-testid="editor-pane">pane</div>,
}));

import {EditorView} from "../src/components/viewer/views/editor/EditorView";

describe("EditorView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        editorState.activeSession = "test-session";
        editorState.error = null;
        editorState.flatNodes = [];
        editorState.isRootLoading = false;
        editorState.selectedPath = null;
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
});
