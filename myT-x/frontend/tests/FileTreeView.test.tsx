import {act, isValidElement, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {FileContentViewerProps} from "../src/components/viewer/views/file-tree/FileContentViewer";
import type {DocumentKind} from "../src/components/viewer/views/file-tree/documentTypes";
import type {FileContentResult, FileNode, FlatNode} from "../src/components/viewer/views/file-tree/fileTreeTypes";
import {RendererSurface} from "../src/components/viewer/views/file-tree/renderers/RendererSurface";

const closeViewMock = vi.fn();
const fileContentViewerMock = vi.fn<(props: FileContentViewerProps) => ReactNode>();
const fileSearchPanelMock = vi.fn<() => ReactNode>();
const fileTreeSidebarMock = vi.fn<(props: {flatNodes: readonly FlatNode[]}) => ReactNode>();
const setQueryMock = vi.fn();
const clearSearchMock = vi.fn();
const tmuxState = {
    config: {
        viewer_shortcuts: null as Record<string, string> | null,
    },
};

const viewState = {
    tree: [] as readonly FileNode[],
    expandedPaths: new Set<string>(),
    loadingPaths: new Set<string>(),
    selectedPath: "docs/diagram.drawio.svg",
    fileContent: null as FileContentResult | null,
    isLoadingContent: false,
    isRootLoading: false,
    error: null as string | null,
    contentError: null as string | null,
    dirError: null as string | null,
    toggleDir: vi.fn(),
    selectFile: vi.fn(),
    loadRoot: vi.fn(),
    activeSession: "session-a",
    activeSessionKey: "session-a:1",
};

vi.mock("../src/components/viewer/views/file-tree/useFileTree", () => ({
    useFileTree: () => viewState,
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: { closeView: typeof closeViewMock }) => unknown) => selector({closeView: closeViewMock}),
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: typeof tmuxState) => unknown) => selector(tmuxState),
}));

vi.mock("../src/components/viewer/views/file-tree/useFileSearch", () => ({
    useFileSearch: () => ({
        query: "",
        setQuery: setQueryMock,
        results: [],
        isSearching: false,
        searchError: null,
        clearSearch: clearSearchMock,
    }),
}));

vi.mock("../src/components/viewer/views/shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({children}: { children?: ReactNode }) => <div>{children}</div>,
}));

vi.mock("../src/components/viewer/views/file-tree/FileTreeSidebar", () => ({
    FileTreeSidebar: (props: {flatNodes: readonly FlatNode[]}) => {
        fileTreeSidebarMock(props);
        return <div/>;
    },
}));

vi.mock("../src/components/viewer/views/file-tree/FileSearchPanel", () => ({
    FileSearchPanel: () => {
        fileSearchPanelMock();
        return <div data-testid="file-search-panel"/>;
    },
}));

vi.mock("../src/components/viewer/views/file-tree/FileContentViewer", () => ({
    FileContentViewer: (props: FileContentViewerProps) => {
        fileContentViewerMock(props);
        return (
            <div
                data-testid="file-content-viewer"
                data-can-preview={String(props.canPreview ?? false)}
                data-render-mode={props.renderMode ?? "uncontrolled"}
            />
        );
    },
}));

vi.mock("../src/components/viewer/views/file-tree/renderers/DrawioRenderer", () => ({
    DrawioRenderer: (props: Record<string, unknown>) => (
        <div data-testid="drawio-renderer" data-kind={String(props.kind)}/>
    ),
}));

import {FileTreeView} from "../src/components/viewer/views/file-tree/FileTreeView";

describe("FileTreeView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        closeViewMock.mockReset();
        fileContentViewerMock.mockReset();
        fileSearchPanelMock.mockReset();
        fileTreeSidebarMock.mockReset();
        setQueryMock.mockReset();
        clearSearchMock.mockReset();
        viewState.toggleDir.mockReset();
        viewState.selectFile.mockReset();
        viewState.loadRoot.mockReset();
        tmuxState.config.viewer_shortcuts = null;
        viewState.tree = [];
        viewState.expandedPaths = new Set<string>();
        viewState.loadingPaths = new Set<string>();
        viewState.selectedPath = "docs/diagram.drawio.svg";
        viewState.fileContent = {
            path: "docs/diagram.drawio.svg",
            content: "<svg/>",
            line_count: 1,
            size: 7,
            truncated: false,
            binary: false,
        };
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    function renderView(): void {
        act(() => {
            root.render(<FileTreeView/>);
        });
    }

    function getRenderedMode(): string | null {
        return container.querySelector<HTMLElement>("[data-testid='file-content-viewer']")?.dataset.renderMode ?? null;
    }

    function dispatchShortcut(eventInit: KeyboardEventInit): void {
        act(() => {
            document.dispatchEvent(new KeyboardEvent("keydown", {
                bubbles: true,
                cancelable: true,
                ...eventInit,
            }));
        });
    }

    function dispatchImeShortcut(eventInit: KeyboardEventInit & { keyCode?: number }): void {
        const event = new KeyboardEvent("keydown", {
            bubbles: true,
            cancelable: true,
            ...eventInit,
        });
        if (typeof eventInit.keyCode === "number") {
            Object.defineProperty(event, "keyCode", {value: eventInit.keyCode});
        }
        act(() => {
            document.dispatchEvent(event);
        });
    }

    it("uses the current file classification immediately after a file switch", () => {
        renderView();

        expect(fileContentViewerMock.mock.lastCall?.[0]).toEqual(expect.objectContaining({
            documentKind: "drawio-svg",
        }));

        viewState.selectedPath = "docs/notes.txt";
        viewState.fileContent = {
            path: "docs/notes.txt",
            content: "plain text",
            line_count: 1,
            size: 10,
            truncated: false,
            binary: false,
        };

        renderView();

        expect(fileContentViewerMock.mock.lastCall?.[0]).toEqual(expect.objectContaining({
            documentKind: "yaml-json-raw",
            content: expect.objectContaining({
                path: "docs/notes.txt",
            }),
        }));
    });

    it("refreshes the current file from the file content header", () => {
        renderView();

        fileContentViewerMock.mock.lastCall?.[0].onRefresh?.();

        expect(viewState.selectFile).toHaveBeenCalledWith("docs/diagram.drawio.svg");
        expect(viewState.loadRoot).not.toHaveBeenCalled();
    });

    it("passes document-filtered flat nodes to the sidebar", () => {
        viewState.tree = [
            {
                name: "src",
                path: "src",
                isDir: true,
                hasChildren: true,
                hasViewTarget: false,
            },
            {
                name: "docs",
                path: "docs",
                isDir: true,
                hasChildren: true,
                hasViewTarget: true,
            },
            {
                name: "README.md",
                path: "README.md",
                isDir: false,
                hasChildren: false,
                hasViewTarget: true,
                size: 128,
            },
            {
                name: "main.go",
                path: "main.go",
                isDir: false,
                hasChildren: false,
                hasViewTarget: false,
                size: 64,
            },
        ] satisfies readonly FileNode[];

        renderView();

        expect(fileTreeSidebarMock.mock.lastCall?.[0].flatNodes.map((node) => node.path)).toEqual([
            "docs",
            "README.md",
        ]);
    });

    it.each([
        ["graphviz", "Graphviz プレビューを読み込み中...", {code: "digraph { a -> b }"}],
        ["markmap", "Markmap プレビューを読み込み中...", {code: "# Root"}],
        ["wavedrom", "WaveDrom プレビューを読み込み中...", {code: "{\"signal\":[]}"}],
        ["vega-lite", "Vega-Lite プレビューを読み込み中...", {code: "{\"mark\":\"bar\"}", kind: "vega-lite"}],
        ["vega", "Vega プレビューを読み込み中...", {code: "{\"signals\":[]}", kind: "vega"}],
    ] satisfies readonly [DocumentKind, string, Record<string, unknown>][])(
        "routes %s documents to the expected lazy preview renderer",
        (kind, loadingMessage, expectedProps) => {
            renderView();

            const previewRenderer = fileContentViewerMock.mock.lastCall?.[0].previewRenderer;
            expect(previewRenderer).toBeTypeOf("function");

            const rendered = previewRenderer?.({
                path: `docs/${kind}.txt`,
                content: String(expectedProps.code),
                line_count: 1,
                size: 1,
                truncated: false,
                binary: false,
            }, kind);

            expect(isValidElement(rendered)).toBe(true);
            const surface = rendered as {type: unknown; props: {children: ReactNode; loadingMessage: string}};
            expect(surface.type).toBe(RendererSurface);
            expect(surface.props.loadingMessage).toBe(loadingMessage);

            const lazyChild = surface.props.children;
            expect(isValidElement(lazyChild)).toBe(true);
            expect((lazyChild as {props: Record<string, unknown>}).props).toEqual(expect.objectContaining(expectedProps));
        },
    );

    it.each([
        ["drawio-svg", "docs/diagram.drawio.svg", "<svg/>"],
        ["drawio-xml", "docs/diagram.drawio", "<mxfile/>"],
    ] satisfies readonly [DocumentKind, string, string][])(
        "routes %s documents to the draw.io renderer with session context",
        (kind, path, content) => {
            renderView();

            const previewRenderer = fileContentViewerMock.mock.lastCall?.[0].previewRenderer;
            expect(previewRenderer).toBeTypeOf("function");

            const rendered = previewRenderer?.({
                path,
                content,
                line_count: 1,
                size: content.length,
                truncated: false,
                binary: false,
            }, kind);

            expect(isValidElement(rendered)).toBe(true);
            expect((rendered as {props: Record<string, unknown>}).props).toEqual(expect.objectContaining({
                kind,
                content,
                filePath: path,
                sessionKey: "session-a:1",
                sessionName: "session-a",
            }));
        },
    );

    it("toggles preview mode with Ctrl+Shift+V", () => {
        renderView();

        expect(getRenderedMode()).toBe("preview");

        dispatchShortcut({key: "V", ctrlKey: true, shiftKey: true});
        expect(getRenderedMode()).toBe("raw");

        dispatchShortcut({key: "V", ctrlKey: true, shiftKey: true});
        expect(getRenderedMode()).toBe("preview");
    });

    it("does not steal Ctrl+Shift+V when a viewer shortcut already owns it", () => {
        tmuxState.config.viewer_shortcuts = {
            "git-graph": "Ctrl+Shift+V",
        };

        renderView();

        dispatchShortcut({key: "V", ctrlKey: true, shiftKey: true});

        expect(getRenderedMode()).toBe("preview");
    });

    it("keeps the Ctrl+F file search shortcut working", () => {
        renderView();

        dispatchShortcut({key: "f", ctrlKey: true});

        expect(container.querySelector("[data-testid='file-search-panel']")).not.toBeNull();
        expect(fileSearchPanelMock).toHaveBeenCalled();
    });

    it("ignores Ctrl+F while IME composition is transitional", () => {
        renderView();

        dispatchImeShortcut({key: "Process", ctrlKey: true});

        expect(container.querySelector("[data-testid='file-search-panel']")).toBeNull();
        expect(fileSearchPanelMock).not.toHaveBeenCalled();
    });

    it("ignores Ctrl+Shift+V while IME composition is transitional", () => {
        renderView();

        dispatchImeShortcut({key: "Unidentified", ctrlKey: true, shiftKey: true, keyCode: 229});

        expect(getRenderedMode()).toBe("preview");
    });

    it.each([
        {
            name: "input",
            createTarget: () => {
                const input = document.createElement("input");
                document.body.appendChild(input);
                input.focus();
                return {element: input};
            },
        },
        {
            name: "textarea",
            createTarget: () => {
                const textarea = document.createElement("textarea");
                document.body.appendChild(textarea);
                textarea.focus();
                return {element: textarea};
            },
        },
        {
            name: "contenteditable",
            createTarget: () => {
                const editable = document.createElement("div");
                editable.contentEditable = "true";
                editable.setAttribute("contenteditable", "true");
                editable.tabIndex = 0;
                document.body.appendChild(editable);
                editable.focus();
                const activeElementDescriptor = Object.getOwnPropertyDescriptor(document, "activeElement");
                Object.defineProperty(document, "activeElement", {
                    configurable: true,
                    get: () => editable,
                });
                return {
                    element: editable,
                    restore: () => {
                        if (activeElementDescriptor) {
                            Object.defineProperty(document, "activeElement", activeElementDescriptor);
                            return;
                        }
                        delete (document as Document & { activeElement?: Element }).activeElement;
                    },
                };
            },
        },
        {
            name: "Monaco",
            createTarget: () => {
                const monaco = document.createElement("div");
                monaco.className = "monaco-editor";
                monaco.tabIndex = -1;
                document.body.appendChild(monaco);
                monaco.focus();
                return {element: monaco};
            },
        },
        {
            name: "xterm",
            createTarget: () => {
                const terminal = document.createElement("div");
                terminal.className = "xterm";
                terminal.tabIndex = -1;
                document.body.appendChild(terminal);
                terminal.focus();
                return {element: terminal};
            },
        },
    ])("does not toggle preview while focus is inside $name", ({createTarget}) => {
        renderView();

        const {element, restore} = createTarget();
        try {
            dispatchShortcut({key: "V", ctrlKey: true, shiftKey: true});
            expect(getRenderedMode()).toBe("preview");
        } finally {
            restore?.();
            element.remove();
        }
    });

    it("resets previewable files back to preview mode after a file switch", () => {
        renderView();
        dispatchShortcut({key: "V", ctrlKey: true, shiftKey: true});
        expect(getRenderedMode()).toBe("raw");

        viewState.selectedPath = "docs/readme.md";
        viewState.fileContent = {
            path: "docs/readme.md",
            content: "# Readme",
            line_count: 1,
            size: 8,
            truncated: false,
            binary: false,
        };

        renderView();

        expect(getRenderedMode()).toBe("preview");
        expect(fileContentViewerMock.mock.lastCall?.[0]).toEqual(expect.objectContaining({
            documentKind: "markdown",
            renderMode: "preview",
        }));
    });
});
