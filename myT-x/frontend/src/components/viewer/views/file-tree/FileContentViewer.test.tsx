import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {FileContentViewer} from "./FileContentViewer";

vi.mock("react-window", () => ({
    FixedSizeList: () => null,
}));

vi.mock("../../../../hooks/useContainerHeight", () => ({
    useContainerHeight: () => 320,
}));

vi.mock("../../../../hooks/useCopyPathNotice", () => ({
    useCopyPathNotice: () => ({
        copyState: null,
        copyPath: vi.fn(),
    }),
}));

vi.mock("../../../../hooks/useShikiHighlight", () => ({
    useShikiHighlight: () => ({
        tokens: null,
        skipInfo: null,
    }),
}));

vi.mock("../shared/TreeOuter", () => ({
    makeScrollStableOuter: () => "div",
}));

vi.mock("./MarkdownPreview", () => ({
    MarkdownPreview: ({content}: {content: string}) => <div>{content}</div>,
}));

vi.mock("./FileContentRow", () => ({
    FileContentRow: () => null,
}));

vi.mock("./FileContentHeader", () => ({
    FileContentHeader: () => <div>header</div>,
}));

vi.mock("./useRowHeight", () => ({
    useRowHeight: () => 20,
}));

vi.mock("./useFileContentSelection", () => ({
    useFileContentSelection: () => ({
        copySelectionNotice: null,
        handleBodyKeyDown: vi.fn(),
        handleBodyMouseDown: vi.fn(),
        handleBodyBlur: vi.fn(),
        handleBodyCopy: vi.fn(),
    }),
}));

describe("FileContentViewer", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
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
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    async function flushRenderer(): Promise<void> {
        await act(async () => {
            await Promise.resolve();
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }

    it("routes binary sqlite files to the preview renderer", async () => {
        await act(async () => {
            root.render(
                <FileContentViewer
                    content={{
                        path: "data/sample.db",
                        content: "",
                        line_count: 1,
                        size: 1024,
                        truncated: false,
                        binary: true,
                    }}
                    isLoading={false}
                    documentKind="sqlite"
                    previewRenderer={() => <div>sqlite preview</div>}
                />,
            );
        });
        await flushRenderer();

        expect(container.textContent).toContain("sqlite preview");
        expect(container.textContent).not.toContain("Binary file");
    });

    it("keeps non-sqlite binary files on the binary fallback", async () => {
        await act(async () => {
            root.render(
                <FileContentViewer
                    content={{
                        path: "data/image.bin",
                        content: "",
                        line_count: 1,
                        size: 1024,
                        truncated: false,
                        binary: true,
                    }}
                    isLoading={false}
                    documentKind="yaml-json-raw"
                    previewRenderer={() => <div>should not render</div>}
                />,
            );
        });
        await flushRenderer();

        expect(container.textContent).toContain("Binary file");
        expect(container.textContent).not.toContain("should not render");
    });
});
