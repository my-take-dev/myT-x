import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {FileContentViewerProps} from "../src/components/viewer/views/file-tree/FileContentViewer";
import type {FileContentResult} from "../src/components/viewer/views/file-tree/fileTreeTypes";

const closeViewMock = vi.fn();
const fileContentViewerMock = vi.fn<(props: FileContentViewerProps) => null>();

const viewState = {
    tree: [],
    expandedPaths: new Set<string>(),
    loadingPaths: new Set<string>(),
    selectedPath: "docs/diagram.drawio.svg",
    fileContent: null as FileContentResult | null,
    isLoadingContent: false,
    isRootLoading: false,
    error: null as string | null,
    contentError: null as string | null,
    dirError: null as string | null,
    watcherError: null as string | null,
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

vi.mock("../src/components/viewer/views/file-tree/useFileSearch", () => ({
    useFileSearch: () => ({
        query: "",
        setQuery: vi.fn(),
        results: [],
        isSearching: false,
        searchError: null,
        clearSearch: vi.fn(),
    }),
}));

vi.mock("../src/components/viewer/views/shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({children}: { children?: ReactNode }) => <div>{children}</div>,
}));

vi.mock("../src/components/viewer/views/file-tree/FileTreeSidebar", () => ({
    FileTreeSidebar: () => <div/>,
}));

vi.mock("../src/components/viewer/views/file-tree/FileSearchPanel", () => ({
    FileSearchPanel: () => <div/>,
}));

vi.mock("../src/components/viewer/views/file-tree/FileContentViewer", () => ({
    FileContentViewer: (props: FileContentViewerProps) => {
        fileContentViewerMock(props);
        return null;
    },
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

    it("uses the current file classification immediately after a file switch", () => {
        act(() => {
            root.render(<FileTreeView/>);
        });

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

        act(() => {
            root.render(<FileTreeView/>);
        });

        expect(fileContentViewerMock.mock.lastCall?.[0]).toEqual(expect.objectContaining({
            documentKind: "yaml-json-raw",
            content: expect.objectContaining({
                path: "docs/notes.txt",
            }),
        }));
    });
});
