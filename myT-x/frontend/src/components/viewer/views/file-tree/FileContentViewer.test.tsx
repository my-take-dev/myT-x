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

    async function renderViewer(
        documentKind: Parameters<typeof FileContentViewer>[0]["documentKind"],
        overrides?: Partial<Parameters<typeof FileContentViewer>[0]>,
    ): Promise<void> {
        await act(async () => {
            root.render(
                <FileContentViewer
                    content={{
                        path: overrides?.content?.path ?? "docs/sample.txt",
                        content: overrides?.content?.content ?? "# preview",
                        line_count: overrides?.content?.line_count ?? 1,
                        size: overrides?.content?.size ?? 128,
                        truncated: overrides?.content?.truncated ?? false,
                        binary: overrides?.content?.binary ?? false,
                    }}
                    isLoading={overrides?.isLoading ?? false}
                    documentKind={documentKind}
                    renderMode={overrides?.renderMode}
                    canPreview={overrides?.canPreview}
                    onRenderModeChange={overrides?.onRenderModeChange}
                    onRefresh={overrides?.onRefresh}
                    previewRenderer={overrides?.previewRenderer ?? (() => <div>custom preview</div>)}
                />,
            );
        });
        await flushRenderer();
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

    it("defaults sqlite documents to preview mode in uncontrolled mode", async () => {
        await renderViewer("sqlite", {
            content: {
                path: "docs/sample.db",
                content: "",
                line_count: 1,
                size: 128,
                truncated: false,
                binary: false,
            },
        });

        const toggleButton = container.querySelector<HTMLButtonElement>(".file-content-toggle-preview");
        expect(toggleButton?.getAttribute("aria-pressed")).toBe("true");
    });

    it.each([
        "markdown",
        "mermaid",
        "drawio-svg",
        "drawio-xml",
        "swagger",
        "graphviz",
        "markmap",
        "wavedrom",
        "vega-lite",
        "vega",
    ] as const)("keeps %s documents on raw mode in uncontrolled mode", async (documentKind) => {
        await renderViewer(documentKind, {
            content: {
                path: `docs/${documentKind}.txt`,
                content: "# preview",
                line_count: 1,
                size: 128,
                truncated: false,
                binary: false,
            },
        });

        const toggleButton = container.querySelector<HTMLButtonElement>(".file-content-toggle-preview");
        expect(toggleButton?.getAttribute("aria-pressed")).toBe("false");
    });

    it("keeps yaml-json-raw documents on the raw view without a toggle button", async () => {
        await renderViewer("yaml-json-raw", {
            content: {
                path: "docs/config.txt",
                content: "plain text",
                line_count: 1,
                size: 10,
                truncated: false,
                binary: false,
            },
        });

        expect(container.querySelector(".file-content-toggle-preview")).toBeNull();
        expect(container.textContent).not.toContain("custom preview");
    });

    it("updates the toggle button title and aria attributes with the preview shortcut", async () => {
        await renderViewer("sqlite", {
            content: {
                path: "docs/sample.db",
                content: "",
                line_count: 1,
                size: 10,
                truncated: false,
                binary: true,
            },
        });

        const toggleButton = container.querySelector<HTMLButtonElement>(".file-content-toggle-preview");
        expect(toggleButton).not.toBeNull();
        expect(toggleButton?.title).toBe("ソース表示 (Ctrl+Shift+V)");
        expect(toggleButton?.getAttribute("aria-label")).toBe("ソース表示 (Ctrl+Shift+V)");
        expect(toggleButton?.getAttribute("aria-keyshortcuts")).toBe("Control+Shift+V");

        act(() => {
            toggleButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(toggleButton?.title).toBe("ソース表示 (Ctrl+Shift+V)");
        expect(toggleButton?.getAttribute("aria-label")).toBe("ソース表示 (Ctrl+Shift+V)");
        expect(toggleButton?.getAttribute("aria-pressed")).toBe("true");
    });

    it("renders a refresh button in the file header", async () => {
        const onRefresh = vi.fn();
        await renderViewer("markdown", {onRefresh});

        const refreshButton = container.querySelector<HTMLButtonElement>("button[aria-label='ファイルを再読み込み']");
        expect(refreshButton).not.toBeNull();

        act(() => {
            refreshButton?.click();
        });

        expect(onRefresh).toHaveBeenCalledTimes(1);
    });
});
